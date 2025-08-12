package server

import (
	"bytes"
	"fmt"
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
	Config            *Config
	IpxeDir           string
	Tarball           string
	Response          string
	DiskLabelTemplate string
	URL               string
	ExternalCPIO      bool
}

func NewMkBoot(config *Config, ipxeDir, tarball, response, diskLabelTemplate, url string) *MkBoot {
	m := MkBoot{config, ipxeDir, tarball, response, diskLabelTemplate, url, true}
	return &m
}

// prepare OS-specific boot files in the cache directory named with MAC address
func (m *MkBoot) Generate() error {
	fmt.Printf("Generate: %s\n", FormatJSON(m))
	switch strings.ToLower(m.Config.OS) {
	case "openbsd":
		return m.mkbootOpenBSD()
	case "debian":
		return m.mkbootDebian()
	case "alpine":
		return m.mkbootAlpine()
	}
	return fmt.Errorf("unexpected OS: '%s'", m.Config.OS)
}

/*
repo=/home/mkrueger/go/src/github.com/rstms/netboot
dist=$repo/template/pub
cache=/home/mkrueger/.cache/netboot

mac='00:0c:29:f5:84:d2'
version=7.5
arch=amd64
url=https://zippy.rstms.net:4443
src_iso=$dist/OpenBSD/$version/$arch/cd75.iso
dst_dir=$cache

doas $repo/scripts/mkboot.openbsd $mac $version $arch $url $src_iso $dst_dir
*/

func (m *MkBoot) mkbootOpenBSD() error {
	fmt.Printf("mkbootOpenBSD: %s %s\n", m.Config.Version, m.Config.Arch)
	if runtime.GOOS != "foo-openbsd" {
		return Fatalf("cannot generate OpenBSD boot resources on %s", runtime.GOOS)
	}
	distDir := filepath.Join("dist", "openbsd", m.Config.Version, m.Config.Arch)
	if !IsDirFS(template.Dist, distDir) {
		return Fatalf("unsupported: OpenBSD %s %s", m.Config.Version, m.Config.Arch)
	}

	tempDir, err := os.MkdirTemp("", "netboot_mkboot_*")
	if err != nil {
		return Fatal(err)
	}
	// FIXME
	//defer os.RemoveAll(tempDir)

	srcIso := "cd" + strings.ReplaceAll(m.Config.Version, ".", "") + ".iso"

	err = CopyFileFromFS(filepath.Join(tempDir, srcIso), filepath.Join(distDir, srcIso), template.Dist)
	if err != nil {
		return Fatal(err)
	}

	script := "mkboot.openbsd"
	for _, file := range []string{script, "rc.netboot", "rc.package"} {
		err = CopyFileFromFS(filepath.Join(tempDir, file), filepath.Join("mkboot", file), template.Mkboot)
		if err != nil {
			return Fatal(err)
		}
	}
	err = os.Chmod(filepath.Join(tempDir, script), 0700)
	if err != nil {
		return Fatal(err)
	}

	user, err := user.Current()
	if err != nil {
		return Fatal(err)
	}

	// FIXME: generate client certs for ISO
	args := []string{
		"./" + script,
		m.Config.Address,
		m.Config.Version,
		m.Config.Arch,
		m.URL,
		srcIso,
		m.IpxeDir,
		user.Username,
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

	cmd.Dir = tempDir
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
	return nil
}

func (m *MkBoot) mkbootAlpine() error {
	panic("fixme")
}

func (m *MkBoot) mkbootDebian() error {
	fmt.Printf("mkbootDebian: %s %s\n", m.Config.Version, m.Config.Arch)

	debianDistDir := filepath.Join("dist", "debian", m.Config.Version, m.Config.Arch)
	if !IsDirFS(template.Dist, debianDistDir) {
		return Fatalf("unsupported: Debian %s %s", m.Config.Version, m.Config.Arch)
	}
	err := CopyFileFromFS(filepath.Join(m.IpxeDir, m.Config.Address+".kernel"), filepath.Join(debianDistDir, "linux"), template.Dist)
	if err != nil {
		return Fatal(err)
	}
	err = CopyFileFromFS(filepath.Join(m.IpxeDir, m.Config.Address+".initrd"), filepath.Join(debianDistDir, "initrd.gz.keymaster"), template.Dist)
	if err != nil {
		return Fatal(err)
	}

	return nil
}
