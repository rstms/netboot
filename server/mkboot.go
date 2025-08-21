package server

import (
	"fmt"
	"github.com/rstms/netboot/bootimg"
	"github.com/rstms/netboot/bootiso"
	"github.com/rstms/netboot/template"
	"log"
	"os"
	"path/filepath"
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

func (m *MkBoot) mkbootOpenBSD() error {
	log.Printf("mkbootOpenBSD: %s %s\n", m.Config.Version, m.Config.Arch)

	// copy openbsd netboot iso: /ipxe/MAC.boot
	srcBoot := filepath.Join("ipxe", fmt.Sprintf("openbsd-%s-%s.iso", m.Config.Version, m.Config.Arch))
	dstBoot := filepath.Join(m.IpxeDir, m.Config.Address+".boot")
	err := CopyFileFromFS(dstBoot, srcBoot, template.Ipxe)
	if err != nil {
		return Fatal(err)
	}

	// add netboot.env to netboot iso bootfiles
	m.BootFiles = append(m.BootFiles, filepath.Join(m.TempDir, "netboot.env"))

	// add gdl??.tgz to netboot iso bootfiles
	tag := strings.ReplaceAll(m.Config.Version, ".", "")
	srcGdl := filepath.Join("dist", "openbsd", m.Config.Version, m.Config.Arch, "gdl"+tag+".tgz")
	dstGdl := filepath.Join(m.TempDir, "gdl.tgz")
	err = CopyFileFromFS(dstGdl, srcGdl, template.Dist)
	if err != nil {
		return Fatal(err)
	}
	m.BootFiles = append(m.BootFiles, dstGdl)

	return m.buildISO()
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

	// stage netboot.rc from template as /ipxe/MAC.postinstall
	// NOTE: this IS NOT /postinstall from package.tgz (see alpine)
	postinstall := filepath.Join(m.IpxeDir, m.Config.Address+".postinstall")
	err = CopyFileFromFS(postinstall, "mkboot/rc.netboot.debian", template.Mkboot)
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

func (m *MkBoot) mkbootAlpine() error {

	log.Printf("mkbootAlpine: %s %s\n", m.Config.Version, m.Config.Arch)

	match := ALPINE_VERSION_PATTERN.FindStringSubmatch(m.Config.Version)
	if len(match) != 4 {
		return Fatalf("unexpected alpine version: %v", m.Config.Version)
	}
	major := match[1]
	minor := match[2]

	// generate error if version/arch not present
	distDir := filepath.Join("dist", "alpine", m.Config.Version, m.Config.Arch)
	if !IsDirFS(template.Dist, distDir) {
		return Fatalf("unsupported: alpine %s %s", m.Config.Version, m.Config.Arch)
	}

	// copy the alpine netboot kernel: /ipxe/MAC.kernel
	srcKernel := filepath.Join(distDir, "kernel")
	dstKernel := filepath.Join(m.IpxeDir, m.Config.Address+".kernel")
	err := CopyFileFromFS(dstKernel, srcKernel, template.Dist)
	if err != nil {
		return Fatal(err)
	}

	// copy the alpine netboot initrd: /ipxe/MAC.initrd
	srcInitrd := filepath.Join(distDir, "initrd")
	dstInitrd := filepath.Join(m.IpxeDir, m.Config.Address+".initrd")
	err = CopyFileFromFS(dstInitrd, srcInitrd, template.Dist)
	if err != nil {
		return Fatal(err)
	}

	// copy the alpine netboot modloop: /ipxe/MAC.modloop
	srcModloop := filepath.Join(distDir, "modloop")
	dstModloop := filepath.Join(m.IpxeDir, m.Config.Address+".modloop")
	err = CopyFileFromFS(dstModloop, srcModloop, template.Dist)
	if err != nil {
		return Fatal(err)
	}

	// stage postinstall script: /ipxe/MAC.postinstall
	// alpine rc.netboot downloads postinstall from the ipxe dir
	// NOTE: this IS /postinstall from package.tgz (see debian)
	srcPostinstall := filepath.Join(m.TempDir, "postinstall")
	dstPostinstall := filepath.Join(m.IpxeDir, m.Config.Address+".postinstall")
	err = CopyFile(dstPostinstall, srcPostinstall)
	if err != nil {
		return Fatal(err)
	}

	// generate overlay tarball

	ovlDir := filepath.Join(m.TempDir, "apkovl")
	err = os.Mkdir(ovlDir, 0755)
	if err != nil {
		return Fatal(err)
	}

	etcDir := filepath.Join(ovlDir, "etc")
	err = os.MkdirAll(filepath.Join(etcDir, "ssl"), 0755)
	if err != nil {
		return Fatal(err)
	}

	err = os.WriteFile(filepath.Join(etcDir, ".default_boot_services"), []byte{}, 0644)
	if err != nil {
		return Fatal(err)
	}

	err = os.MkdirAll(filepath.Join(etcDir, "runlevels", "default"), 0755)
	if err != nil {
		return Fatal(err)
	}

	err = os.Symlink("/etc/init.d/local", filepath.Join(etcDir, "runlevels", "default", "local"))
	if err != nil {
		return Fatal(err)
	}

	apkDir := filepath.Join(etcDir, "apk")
	err = os.Mkdir(apkDir, 0755)
	if err != nil {
		return Fatal(err)
	}

	if !strings.HasPrefix(m.Config.Mirror, "http") {
		return Fatalf("unexpected non-URL alpine mirror: %s", m.Config.Mirror)
	}
	repoData := fmt.Sprintf("%s/alpine/v%s.%s/main\n", m.Config.Mirror, major, minor)
	repoData += fmt.Sprintf("%s/alpine/v%s.%s/community\n", m.Config.Mirror, major, minor)
	err = os.WriteFile(filepath.Join(apkDir, "repositories"), []byte(repoData), 0644)
	if err != nil {
		return Fatal(err)
	}

	localDir := filepath.Join(etcDir, "local.d")
	err = os.Mkdir(localDir, 0755)
	if err != nil {
		return Fatal(err)
	}
	autostart := filepath.Join(localDir, "auto-setup-alpine.start")
	err = CopyFileFromFS(autostart, filepath.Join("mkboot", "rc.netboot.alpine"), template.Mkboot)
	if err != nil {
		return Fatal(err)
	}
	err = os.Chmod(autostart, 0755)
	if err != nil {
		return Fatal(err)
	}

	// add netboot.env to netboot iso bootfiles
	m.BootFiles = append(m.BootFiles, filepath.Join(m.TempDir, "netboot.env"))

	// write apk overlay /ipxe/MAC.apkovl
	err = WriteTarball(filepath.Join(m.IpxeDir, m.Config.Address+".apkovl.tar.gz"), ovlDir, true)
	if err != nil {
		return Fatal(err)
	}

	return m.buildISO()
}

// build a netboot ISO with embedded IPXE menu autoexec.ipxe
func (m *MkBoot) buildISO() error {

	// FIXME: netboot iso seems to be using the client certificate and CA baked into the source ISO
	// instead it should use /netboot.pem, /netboot.key from the ISO root directory

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
