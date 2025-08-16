package server

import (
	"bytes"
	"fmt"
	"github.com/rstms/netboot/bootimg"
	"github.com/rstms/netboot/bootiso"
	"github.com/rstms/netboot/template"
	"log"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
)

type MkBoot struct {
	TempDir   string
	IpxeDir   string
	URL       string
	BootFiles []string
	Config    *Config
	ISO       string
}

func NewMkBoot(tempDir, ipxeDir, url string, bootFiles []string, config *Config) *MkBoot {
	m := MkBoot{
		TempDir:   tempDir,
		IpxeDir:   ipxeDir,
		URL:       url,
		BootFiles: bootFiles,
		Config:    config,
	}
	return &m
}

// prepare OS-specific boot files in the cache directory named with MAC address
func (m *MkBoot) Generate() (string, error) {

	m.ISO = filepath.Join(m.IpxeDir, fmt.Sprintf("%s.iso", strings.ReplaceAll(m.Config.Address, ":", "-")))
	if IsFile(m.ISO) {
		err := os.Remove(m.ISO)
		if err != nil {
			return "", Fatal(err)
		}
	}
	log.Printf("Mkboot.Generate: %s\n", FormatJSON(m))
	switch strings.ToLower(m.Config.OS) {
	case "openbsd":
		err := m.mkbootOpenBSD()
		if err != nil {
			return "", Fatal(err)
		}
	case "debian":
		err := m.mkbootDebian()
		if err != nil {
			return "", Fatal(err)
		}
	case "alpine":
		err := m.mkbootAlpine()
		if err != nil {
			return "", Fatal(err)
		}
	default:
		return "", Fatalf("unexpected OS: '%s'", m.Config.OS)
	}
	return m.ISO, nil
}

func (m *MkBoot) writeBootFileList() (string, error) {
	filename := filepath.Join(m.TempDir, "boot_files")
	files := []string{}
	for _, file := range m.BootFiles {
		_, name := filepath.Split(file)
		files = append(files, name)
	}
	err := os.WriteFile(filename, []byte(strings.Join(files, "\n")+"\n"), 0600)
	if err != nil {
		return "", err
	}
	return filename, nil
}

func (m *MkBoot) mkbootOpenBSD() error {
	log.Printf("mkbootOpenBSD: %s %s\n", m.Config.Version, m.Config.Arch)

	if runtime.GOOS != "openbsd" {
		return Fatalf("cannot generate OpenBSD boot resources on %s", runtime.GOOS)
	}

	distDir := filepath.Join("dist", "openbsd", m.Config.Version, m.Config.Arch)
	if !IsDirFS(template.Dist, distDir) {
		return Fatalf("unsupported: OpenBSD %s %s", m.Config.Version, m.Config.Arch)
	}

	srcIso := "cd" + strings.ReplaceAll(m.Config.Version, ".", "") + ".iso"

	err := CopyFileFromFS(filepath.Join(m.TempDir, srcIso), filepath.Join(distDir, srcIso), template.Dist)
	if err != nil {
		return Fatal(err)
	}

	script := "mkboot.openbsd"
	for _, file := range []string{script, "rc.netboot", "rc.package"} {
		err = CopyFileFromFS(filepath.Join(m.TempDir, file), filepath.Join("mkboot", file), template.Mkboot)
		if err != nil {
			return Fatal(err)
		}
	}
	err = os.Chmod(filepath.Join(m.TempDir, script), 0700)
	if err != nil {
		return Fatal(err)
	}

	user, err := user.Current()
	if err != nil {
		return Fatal(err)
	}

	bootFiles, err := m.writeBootFileList()
	if err != nil {
		return Fatal(err)
	}

	// FIXME: add GDL to ISO
	// FIXME: generate client certs for ISO

	dstIso := filepath.Join(m.IpxeDir, m.Config.Address+".boot")

	args := []string{
		"./" + script,
		dstIso,
		srcIso,
		m.Config.Address,
		m.Config.Version,
		m.Config.Arch,
		m.URL,
		user.Username,
		bootFiles,
	}
	if m.Config.Serial != "" {
		args = append(args, m.Config.Serial)
	}
	cmd := exec.Command("/usr/bin/doas", args...)
	log.Printf("executing: %v\n", cmd)

	var stdout bytes.Buffer
	//cmd.Stdout = &stdout
	var stderr bytes.Buffer
	//cmd.Stderr = &stderr
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	cmd.Dir = m.TempDir
	err = cmd.Run()
	switch err.(type) {
	case nil:
	case *exec.ExitError:
		exitCode := cmd.ProcessState.ExitCode()
		elines := strings.Split(stderr.String(), "\n")
		for _, eline := range elines {
			log.Printf("stderr: %s\n", eline)
		}
		return Fatalf("script %s exited %d\n", script, exitCode)
	default:
		return Fatal(err)
	}
	log.Println(stdout.String())

	return m.buildISO()
}

func (m *MkBoot) mkbootAlpine() error {
	panic("fixme")
}

func (m *MkBoot) mkbootDebian() error {
	log.Printf("mkbootDebian: %s %s\n", m.Config.Version, m.Config.Arch)

	// generate error if version/arch not present
	distDir := filepath.Join("dist", "debian", m.Config.Version, m.Config.Arch)
	if !IsDirFS(template.Dist, distDir) {
		return Fatalf("unsupported: Debian %s %s", m.Config.Version, m.Config.Arch)
	}

	// copy cacerts.tgz: /ipxe/MAC.cacerts
	// will be patched into the initrd by rstms-netboot-debian.ipxe as /cacerts.tgz
	cacerts := filepath.Join(m.IpxeDir, m.Config.Address+".cacerts")
	err := CopyFileFromFS(cacerts, filepath.Join("certs", "cacerts.tgz"), template.Certs)
	if err != nil {
		return Fatal(err)
	}

	// postinstall: /ipxe/MAC.postinstall
	err = CopyFile(filepath.Join(m.IpxeDir, m.Config.Address+".postinstall"), filepath.Join(m.TempDir, "postinstall"))
	if err != nil {
		return Fatal(err)
	}

	// copy the debian installer kernel: /ipxe/MAC.kernel
	srcKernal := filepath.Join(distDir, "linux")
	dstKernal := filepath.Join(m.IpxeDir, m.Config.Address+".kernel")
	err = CopyFileFromFS(dstKernal, srcKernal, template.Dist)
	if err != nil {
		return Fatal(err)
	}

	// copy the debian installer initrd: /ipxe/MAC.initrd
	srcInitrd := filepath.Join(distDir, "initrd.gz")
	dstInitrd := filepath.Join(m.IpxeDir, m.Config.Address+".initrd")
	err = CopyFileFromFS(dstInitrd, srcInitrd, template.Dist)
	if err != nil {
		return Fatal(err)
	}

	return m.buildISO()
}

// build a netboot ISO with embedded IPXE menu autoexec.ipxe
func (m *MkBoot) buildISO() error {

	// copy netboot source ISO from IPXE template
	srcIso := filepath.Join(m.TempDir, "netboot.iso")
	err := CopyFileFromFS(srcIso, filepath.Join("ipxe", "netboot.xyz.iso"), template.Ipxe)
	if err != nil {
		return Fatal(err)
	}

	// copy source EFI boot disk image for CreateEFIImage from IPXE template
	efiBin := filepath.Join(m.TempDir, "BOOTX64.EFI")
	err = CopyFileFromFS(efiBin, filepath.Join("ipxe", "netboot.xyz.efi"), template.Ipxe)
	if err != nil {
		return Fatal(err)
	}

	// use the customized autoexec.ipxe in the temp directory
	autoexec := filepath.Join(m.TempDir, "autoexec.ipxe")

	// generate the EFI boot disk image with autoexec (ipxe menu)
	efiImage := filepath.Join(m.TempDir, "efi.img")
	err = bootimg.CreateEFIImage(efiImage, efiBin, autoexec)
	if err != nil {
		return Fatal(err)
	}

	// generate the netboot ISO
	err = bootiso.CreateNetbootISOImage(m.ISO, srcIso, efiImage, autoexec, m.BootFiles)
	if err != nil {
		return Fatal(err)
	}

	return nil
}
