package server

import (
	"fmt"
	"github.com/rstms/netboot/bootiso"
	"github.com/rstms/netboot/files"
	"github.com/rstms/netboot/message"
	"github.com/rstms/netboot/template"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

var VERSION_PATTERN = regexp.MustCompile(`^(\d+)\.(\d+)\.(\d+)$`)

const NETBOOT_IPXE_IMG = "rstms-netboot.img.gz"
const NETBOOT_IPXE_ISO = "rstms-netboot.iso.gz"
const NETBOOT_IPXE_EFI = "rstms-netboot.efi.gz"

const GENERATE_IMAGE = true
const NO_GENERATE_IMAGE = false

type MkBoot struct {
	TempDir   string
	IpxeDir   string
	DistDir   string
	URL       string
	BootFiles *[]string
	Config    *message.NetbootConfig
	ISO       string
}

func NewMkBoot(tempDir, ipxeDir, distDir, url string, bootFiles *[]string, config *message.NetbootConfig) *MkBoot {
	m := MkBoot{
		TempDir:   tempDir,
		IpxeDir:   ipxeDir,
		DistDir:   distDir,
		URL:       url,
		BootFiles: bootFiles,
		Config:    config,
	}
	return &m
}

// prepare OS-specific boot files in the cache directory named with MAC address
func (m *MkBoot) Generate() (string, error) {

	m.ISO = filepath.Join(m.IpxeDir, fmt.Sprintf("%s.iso", m.Config.Address))
	if IsFile(m.ISO) {
		err := os.Remove(m.ISO)
		if err != nil {
			return "", Fatal(err)
		}
	}
	log.Printf("Mkboot.Generate: %s\n", FormatJSON(m))

	switch m.Config.OS {
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
	case "devuan":
		err := m.mkbootDevuan()
		if err != nil {
			return "", Fatal(err)
		}
	case "alpine":
		err := m.mkbootAlpine(false)
		if err != nil {
			return "", Fatal(err)
		}
	case "windows":
		err := m.mkbootWindows()
		if err != nil {
			return "", Fatal(err)
		}
	default:
		return "", Fatalf("unexpected OS: '%s'", m.Config.OS)
	}

	if m.Config.AlpineLoader != "" {
		err := m.mkbootAlpine(true)
		if err != nil {
			return "", Fatal(err)
		}
	}

	return m.ISO, nil
}

// semver srot a slice of strings using a regex pattern to parse major, minor, patch groups
func SemverSort(unsorted []string, pattern *regexp.Regexp) ([]string, error) {
	keyMap := make(map[string]string)
	keys := []string{}
	log.Printf("unsorted: %s\n", FormatJSON(unsorted))
	if len(unsorted) <= 1 {
		return unsorted, nil
	}
	for _, entry := range unsorted {
		parts := pattern.FindStringSubmatch(entry)
		if len(parts) == 4 {
			major, err := strconv.Atoi(parts[1])
			if err != nil {
				return nil, Fatal(err)
			}
			minor, err := strconv.Atoi(parts[2])
			if err != nil {
				return nil, Fatal(err)
			}
			version, err := strconv.Atoi(parts[3])
			if err != nil {
				return nil, Fatal(err)
			}
			key := fmt.Sprintf("%04d.%04d.%04d", major, minor, version)
			keys = append(keys, key)
			keyMap[key] = entry
		}
	}
	slices.Sort(keys)
	sorted := []string{}
	for _, key := range keys {
		sorted = append(sorted, keyMap[key])
	}
	log.Printf("sorted: %s\n", FormatJSON(sorted))
	return sorted, nil
}

func DefaultDist(distDir, osName string) (string, string, string, error) {
	versions, err := template.DistVersions(distDir, osName)
	if err != nil {
		return "", "", "", Fatal(err)
	}
	sortedVersions, err := SemverSort(versions, VERSION_PATTERN)
	if err != nil {
		return "", "", "", Fatal(err)
	}
	version := sortedVersions[0]

	archs, err := template.DistArchs(distDir, osName, version)
	if err != nil {
		return "", "", "", Fatal(err)
	}
	arch := archs[0]

	var mirror string
	switch osName {
	case "alpine":
		mirror = DEFAULT_ALPINE_MIRROR
	case "debian":
		mirror = DEFAULT_DEBIAN_MIRROR
	case "devuan":
		mirror = DEFAULT_DEVUAN_MIRROR
	case "openbsd":
		mirror = DEFAULT_OPENBSD_MIRROR
	default:
		return "", "", "", Fatalf("unexpected os: %s\n", osName)
	}

	log.Printf("DefaultDist(%s) returning version=%s arch=%s mirror=%s\n", osName, version, arch, mirror)

	return version, arch, mirror, nil
}

func (m *MkBoot) mkbootOpenBSD() error {

	log.Printf("mkbootOpenBSD: %s %s\n", m.Config.Version, m.Config.Arch)

	// generate error if version/arch not present
	_, err := template.DistPath(m.DistDir, m.Config.OS, m.Config.Version, m.Config.Arch)
	if err != nil {
		return Fatal(err)
	}

	// extract template-customized openbsd netboot ISO to /ipxe/MAC.boot
	// autoexec.ipxe in the iso image will sanboot /san/MAC.boot
	srcBoot := filepath.Join("ipxe", fmt.Sprintf("openbsd-%s-%s.iso.gz", m.Config.Version, m.Config.Arch))
	dstBoot := filepath.Join(m.IpxeDir, m.Config.Address+".boot")
	err = files.UnzipFileFromFS(dstBoot, srcBoot, template.Ipxe)
	if err != nil {
		return Fatal(err)
	}
	log.Printf("mkbootOpenbsd: boot=%s\n", dstBoot)

	/* don't write the GDL to the iso, rc.netboot will FTP download it
	// add gdl??.tgz to netboot iso bootfiles
	tag := strings.ReplaceAll(m.Config.Version, ".", "")
	srcGdl := filepath.Join("dist", "openbsd", m.Config.Version, m.Config.Arch, "gdl"+tag+".tgz")
	dstGdl := filepath.Join(m.TempDir, "gdl.tgz")
	log.Printf("mkbootOpenbsd: gdl=%s\n", dstGdl)
	err = files.CopyFileFromFS(dstGdl, srcGdl, template.Dist)
	if err != nil {
		return Fatal(err)
	}
	*m.BootFiles = append(*m.BootFiles, dstGdl)
	*/

	srcImage := NETBOOT_IPXE_IMG
	injectBootFiles := true
	err = m.buildIMG("openbsd", srcImage, injectBootFiles)
	if err != nil {
		return Fatal(err)
	}

	err = m.buildISO("openbsd")
	if err != nil {
		return Fatal(err)
	}

	log.Printf("mkbootOpenbsd: boot=%s\n", dstBoot)

	return nil
}

func (m *MkBoot) mkbootDebian() error {
	log.Printf("mkbootDebian: %s %s\n", m.Config.Version, m.Config.Arch)

	// generate error if version/arch not present
	distDir, err := template.DistPath(m.DistDir, "debian", m.Config.Version, m.Config.Arch)
	if err != nil {
		return Fatal(err)
	}
	log.Printf("distDir: %s\n", distDir)

	// copy cacerts.tgz from package tarball: /ipxe/MAC.cacerts
	// patched into the initrd by rstms-netboot-debian.ipxe (autoexec.ipxe) as /cacerts.tgz
	tarballPathname := filepath.Join(m.IpxeDir, fmt.Sprintf("%s.tgz", m.Config.Address))
	cacerts := filepath.Join(m.IpxeDir, m.Config.Address+".cacerts")
	err = files.ExtractTarballFile(cacerts, "root/cacerts.tgz", tarballPathname)
	if err != nil {
		return Fatal(err)
	}

	// generate netboot tarball: /ipxe/MAC.netboot
	// contains all BootFiles, extracts to /netboot
	// patched into the initrd by rstms-netboot-debian.ipxe (autoexec.ipxe) as /netboot.tgz
	err = m.writeDebianNetbootTarball("Debian")
	if err != nil {
		return Fatal(err)
	}

	// for debian, overwrite /ipxe/MAC.postinstall with template/mkboot/rc.netboot.debian
	postinstall := filepath.Join(m.IpxeDir, m.Config.Address+".postinstall")
	log.Printf("mkbootDebian: postinstall=%s\n", postinstall)
	err = files.CopyFileFromFS(postinstall, "mkboot/rc.netboot.debian", template.Mkboot)
	if err != nil {
		return Fatal(err)
	}

	// copy the debian installer kernel: /ipxe/MAC.kernel

	dstKernel := filepath.Join(m.IpxeDir, m.Config.Address+".kernel")
	log.Printf("mkbootDebian: kernel=%s\n", dstKernel)
	//log.Printf("dstKernel=%s\n", dstKernel)
	//log.Printf("distDir=%s\n", distDir)
	//log.Printf("m.DistDir=%s\n", m.DistDir)
	err = files.CopyFileFromFS(dstKernel, "linux", os.DirFS(distDir))
	if err != nil {
		return Fatal(err)
	}

	// copy the debian installer initrd: /ipxe/MAC.initrd
	dstInitrd := filepath.Join(m.IpxeDir, m.Config.Address+".initrd")
	log.Printf("mkbootDebian: initrd=%s\n", dstInitrd)
	err = files.CopyFileFromFS(dstInitrd, "initrd.gz", os.DirFS(distDir))
	if err != nil {
		return Fatal(err)
	}

	err = m.buildIMG("debian", NETBOOT_IPXE_IMG, true)
	if err != nil {
		return Fatal(err)
	}

	err = m.buildISO("debian")
	if err != nil {
		return Fatal(err)
	}

	return nil
}

func (m *MkBoot) mkbootDevuan() error {
	log.Printf("mkbootDevuan: %s %s\n", m.Config.Version, m.Config.Arch)

	// generate error if version/arch not present
	distDir, err := template.DistPath(m.DistDir, "devuan", m.Config.Version, m.Config.Arch)
	if err != nil {
		return Fatal(err)
	}
	log.Printf("distDir: %s\n", distDir)

	// copy cacerts.tgz from package tarball: /ipxe/MAC.cacerts
	// patched into the initrd by rstms-netboot-devuan.ipxe (autoexec.ipxe) as /cacerts.tgz
	tarballPathname := filepath.Join(m.IpxeDir, fmt.Sprintf("%s.tgz", m.Config.Address))
	cacerts := filepath.Join(m.IpxeDir, m.Config.Address+".cacerts")
	err = files.ExtractTarballFile(cacerts, "root/cacerts.tgz", tarballPathname)
	if err != nil {
		return Fatal(err)
	}

	// generate netboot tarball: /ipxe/MAC.netboot
	// contains all BootFiles, extracts to /netboot
	// patched into the initrd by rstms-netboot-devuan.ipxe (autoexec.ipxe) as /netboot.tgz
	err = m.writeDebianNetbootTarball("Devuan")
	if err != nil {
		return Fatal(err)
	}

	// for devuan, overwrite /ipxe/MAC.postinstall with template/mkboot/rc.netboot.devuan
	postinstall := filepath.Join(m.IpxeDir, m.Config.Address+".postinstall")
	log.Printf("mkbootDevuan: postinstall=%s\n", postinstall)
	err = files.CopyFileFromFS(postinstall, "mkboot/rc.netboot.devuan", template.Mkboot)
	if err != nil {
		return Fatal(err)
	}

	// copy the devuan installer kernel: /ipxe/MAC.kernel

	dstKernel := filepath.Join(m.IpxeDir, m.Config.Address+".kernel")
	log.Printf("mkbootDevuan: kernel=%s\n", dstKernel)
	//log.Printf("dstKernel=%s\n", dstKernel)
	//log.Printf("distDir=%s\n", distDir)
	//log.Printf("m.DistDir=%s\n", m.DistDir)
	err = files.CopyFileFromFS(dstKernel, "linux", os.DirFS(distDir))
	if err != nil {
		return Fatal(err)
	}

	// copy the devuan installer initrd: /ipxe/MAC.initrd
	dstInitrd := filepath.Join(m.IpxeDir, m.Config.Address+".initrd")
	log.Printf("mkbootDevuan: initrd=%s\n", dstInitrd)
	err = files.CopyFileFromFS(dstInitrd, "initrd.gz", os.DirFS(distDir))
	if err != nil {
		return Fatal(err)
	}

	err = m.buildIMG("devuan", NETBOOT_IPXE_IMG, true)
	if err != nil {
		return Fatal(err)
	}

	err = m.buildISO("devuan")
	if err != nil {
		return Fatal(err)
	}

	return nil
}

func (m *MkBoot) mkbootAlpine(imageLoader bool) error {
	log.Printf("mkbootAlpine: imageLoader=%v\n", imageLoader)
	version := m.Config.Version
	arch := m.Config.Arch
	mirror := m.Config.Mirror

	var err error

	if imageLoader {
		version, arch, mirror, err = DefaultDist(m.DistDir, "alpine")
		if err != nil {
			return Fatal(err)
		}
	}

	distDir, err := template.DistPath(m.DistDir, "alpine", version, arch)
	if err != nil {
		return Fatal(err)
	}

	match := ALPINE_VERSION_PATTERN.FindStringSubmatch(version)
	if len(match) != 4 {
		return Fatalf("unexpected alpine version: %v", version)
	}
	major := match[1]
	minor := match[2]

	// copy the alpine netboot kernel: /ipxe/MAC.kernel
	dstKernel := filepath.Join(m.IpxeDir, m.Config.Address+".kernel")
	log.Printf("mkbootAlpine: kernel=%s\n", dstKernel)
	err = files.CopyFileFromFS(dstKernel, "kernel", os.DirFS(distDir))
	if err != nil {
		return Fatal(err)
	}

	// copy the alpine netboot initrd: /ipxe/MAC.initrd
	dstInitrd := filepath.Join(m.IpxeDir, m.Config.Address+".initrd")
	log.Printf("mkbootAlpine: initrd=%s\n", dstInitrd)
	err = files.CopyFileFromFS(dstInitrd, "initrd", os.DirFS(distDir))
	if err != nil {
		return Fatal(err)
	}

	// copy the alpine netboot modloop: /ipxe/MAC.modloop
	dstModloop := filepath.Join(m.IpxeDir, m.Config.Address+".modloop")
	log.Printf("mkbootAlpine: modloop=%s\n", dstModloop)
	err = files.CopyFileFromFS(dstModloop, "modloop", os.DirFS(distDir))
	if err != nil {
		return Fatal(err)
	}

	// generate overlay tarball
	modes := make(map[string]fs.FileMode)

	ovlDir := filepath.Join(m.TempDir, "apkovl")
	err = os.Mkdir(ovlDir, 0755)
	if err != nil {
		return Fatal(err)
	}
	modes[ovlDir] = 0755

	etcDir := filepath.Join(ovlDir, "etc")
	dir := filepath.Join(etcDir, "ssl")
	err = os.MkdirAll(dir, 0755)
	if err != nil {
		return Fatal(err)
	}
	modes[dir] = 0755

	file := filepath.Join(etcDir, ".default_boot_services")
	err = os.WriteFile(file, []byte{}, 0644)
	if err != nil {
		return Fatal(err)
	}
	modes[file] = 0644

	dir = filepath.Join(etcDir, "runlevels")
	modes[dir] = 0755

	dir = filepath.Join(etcDir, "runlevels", "default")
	err = os.MkdirAll(dir, 0755)
	if err != nil {
		return Fatal(err)
	}
	modes[dir] = 0755

	linkTarget := "/etc/init.d/local"
	linkFile := filepath.Join(etcDir, "runlevels", "default", "local")
	// add filename to symlinks for WriteTarball
	symlinks := []string{linkFile}
	// write link target as link file content
	err = os.WriteFile(linkFile, []byte(linkTarget), 0755)
	if err != nil {
		return Fatal(err)
	}
	modes[linkFile] = 0755

	apkDir := filepath.Join(etcDir, "apk")
	err = os.Mkdir(apkDir, 0755)
	if err != nil {
		return Fatal(err)
	}
	modes[apkDir] = 0755

	if !strings.HasPrefix(mirror, "http") {
		return Fatalf("unexpected non-URL alpine mirror: %s", mirror)
	}
	repoData := fmt.Sprintf("%s/alpine/v%s.%s/main\n", mirror, major, minor)
	repoData += fmt.Sprintf("%s/alpine/v%s.%s/community\n", mirror, major, minor)
	file = filepath.Join(apkDir, "repositories")
	err = os.WriteFile(file, []byte(repoData), 0644)
	if err != nil {
		return Fatal(err)
	}
	modes[file] = 0644

	localDir := filepath.Join(etcDir, "local.d")
	err = os.Mkdir(localDir, 0755)
	if err != nil {
		return Fatal(err)
	}
	modes[localDir] = 0755

	autostart := filepath.Join(localDir, "auto-setup-alpine.start")
	err = files.CopyFileFromFS(autostart, filepath.Join("mkboot", "rc.netboot.alpine"), template.Mkboot)
	if err != nil {
		return Fatal(err)
	}
	err = os.Chmod(autostart, 0755)
	if err != nil {
		return Fatal(err)
	}
	modes[autostart] = 0755

	// write apk overlay /ipxe/MAC.apkovl
	dstTarball := filepath.Join(m.IpxeDir, m.Config.Address+".apkovl.tar.gz")
	log.Printf("mkbootAlpine: tarball=%s\n", dstTarball)
	err = files.WriteTarball(dstTarball, ovlDir, true, symlinks, modes)
	if err != nil {
		return Fatal(err)
	}

	// if called with imageLoader set, we are using the alpine installer
	// to write an IMG file for another OS, so don't generate the alpine one
	label := "alpine"
	if !imageLoader {
		label += "-loader"
		err = m.buildIMG(label, NETBOOT_IPXE_IMG, true)
		if err != nil {
			return Fatal(err)
		}
	}

	err = m.buildISO(label)
	if err != nil {
		return Fatal(err)
	}

	return nil
}

func (m *MkBoot) mkbootWindows() error {
	log.Printf("mkbootWindows: %s %s\n", m.Config.Version, m.Config.Arch)

	// generate error if version/arch not present
	_, err := template.DistPath(m.DistDir, "windows", m.Config.Version, m.Config.Arch)
	if err != nil {
		return Fatal(err)
	}

	isoDir := filepath.Join(m.TempDir, "iso")

	netbootDir := filepath.Join(isoDir, "$OEM$", "$1", "netboot")
	err = os.MkdirAll(netbootDir, 0700)
	if err != nil {
		return Fatal(err)
	}

	err = files.ExtractTarball(netbootDir, filepath.Join(m.IpxeDir, m.Config.Address+".tgz"))
	if err != nil {
		return Fatal(err)
	}

	for _, name := range []string{"postinstall", "boxen.run", "install.site"} {
		err := os.Rename(filepath.Join(netbootDir, name), filepath.Join(netbootDir, name)+".ps1")
		if err != nil {
			return Fatal(err)
		}
	}

	err = files.CopyFile(filepath.Join(netbootDir, "netboot.env"), filepath.Join(m.TempDir, "netboot.env"))
	if err != nil {
		return Fatal(err)
	}

	err = files.CopyFile(filepath.Join(isoDir, "autounattend.xml"), filepath.Join(m.IpxeDir, m.Config.Address+".response"))
	if err != nil {
		return Fatal(err)
	}

	err = bootiso.CreateISO(m.ISO, isoDir, "unattend_iso", true)
	if err != nil {
		return Fatal(err)
	}
	return nil
}

func (m *MkBoot) windowsUserDir(oemDir, userName string) string {
	return filepath.Join(oemDir, "Users", userName+"."+strings.ToUpper(m.Config.Hostname))
}

// build a netboot IMG with embedded IPXE autoexec.ipxe and BootFiles
func (m *MkBoot) buildIMG(label, srcImage string, injectFiles bool) error {

	// hetzner rescue mode netboot image: /ipxe/MAC.img
	srcImagePathname := filepath.Join("ipxe", srcImage)
	dstImage := filepath.Join(m.IpxeDir, m.Config.Address+".img")
	err := files.UnzipFileFromFS(dstImage, srcImagePathname, template.Ipxe)
	if err != nil {
		return Fatal(err)
	}

	if injectFiles {
		err = InjectBootFiles(dstImage, *m.BootFiles)
		if err != nil {
			return Fatal(err)
		}
	}
	log.Printf("BuildIMG[%s] wrote %s\n", label, dstImage)
	return nil
}

// build a netboot ISO with embedded IPXE menu autoexec.ipxe
func (m *MkBoot) buildISO(label string) error {

	// FIXME: netboot iso seems to be using the client certificate and CA baked into the source ISO
	// instead it should use /netboot.pem, /netboot.key from the ISO root directory

	// copy netboot source ISO from IPXE template
	srcIso := filepath.Join(m.TempDir, "netboot.iso")
	err := files.UnzipFileFromFS(srcIso, filepath.Join("ipxe", NETBOOT_IPXE_ISO), template.Ipxe)
	if err != nil {
		return Fatal(err)
	}

	// copy source EFI boot disk image for CreateEFIImage from IPXE template
	efiBin := filepath.Join(m.TempDir, "BOOTX64.EFI")
	err = files.UnzipFileFromFS(efiBin, filepath.Join("ipxe", NETBOOT_IPXE_EFI), template.Ipxe)
	if err != nil {
		return Fatal(err)
	}

	// use the customized autoexec.ipxe in the temp directory
	autoexec := filepath.Join(m.TempDir, "autoexec.ipxe.iso")

	// generate the EFI boot disk image with autoexec (ipxe menu)
	efiImage := filepath.Join(m.TempDir, "efi.img")
	err = CreateEFIImage(efiImage, efiBin, autoexec)
	if err != nil {
		return Fatal(err)
	}

	// generate the netboot ISO
	err = bootiso.CreateNetbootISO(m.ISO, srcIso, efiImage, *m.BootFiles)
	if err != nil {
		return Fatal(err)
	}

	log.Printf("BuildISO[%s] wrote %s\n", label, m.ISO)

	return nil
}

func FormatMAC(mac, separator string) (string, error) {
	if !NORMALIZED_MAC_PATTERN.MatchString(mac) {
		return "", Fatalf("expected normalized MAC, got: %v", mac)
	}
	var formatted string
	var sep string
	for i := 0; i < len(mac); i += 2 {
		formatted += sep + mac[i:i+2]
		sep = separator
	}
	return formatted, nil
}

func (m *MkBoot) writeDebianNetbootTarball(label string) error {
	modes := make(map[string]fs.FileMode)
	netbootDir := filepath.Join(m.TempDir, "netboot")
	err := os.MkdirAll(filepath.Join(netbootDir, "netboot"), 0700)
	if err != nil {
		return Fatal(err)
	}
	for _, srcPathname := range *m.BootFiles {
		_, dstName := filepath.Split(srcPathname)
		dstPathname := filepath.Join(netbootDir, "netboot", dstName)
		err = files.CopyFile(dstPathname, srcPathname)
		if err != nil {
			return Fatal(err)
		}
		modes[dstPathname] = 0600
	}
	netBall := filepath.Join(m.IpxeDir, m.Config.Address+".netboot")
	log.Printf("mkboot%s: netbootTarball=%s\n", label, netBall)
	err = files.WriteTarball(netBall, filepath.Join(m.TempDir, "netboot"), true, []string{}, modes)
	if err != nil {
		return Fatal(err)
	}
	return nil
}
