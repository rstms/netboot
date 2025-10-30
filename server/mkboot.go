package server

import (
	"fmt"
	"github.com/rstms/ffs/image"
	"github.com/rstms/netboot/bootiso"
	"github.com/rstms/netboot/files"
	"github.com/rstms/netboot/message"
	"github.com/rstms/netboot/template"
	"io/fs"
	"log"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

var VERSION_PATTERN = regexp.MustCompile(`^(\d+)\.(\d+)\.(\d+)$`)

const GENERATE_IMAGE = true
const NO_GENERATE_IMAGE = false

type MkBoot struct {
	TempDir   string
	IpxeDir   string
	URL       string
	BootFiles *[]string
	Config    *message.NetbootConfig
	ISO       string
}

func NewMkBoot(tempDir, ipxeDir, url string, bootFiles *[]string, config *message.NetbootConfig) *MkBoot {
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

	m.ISO = filepath.Join(m.IpxeDir, fmt.Sprintf("%s.iso", m.Config.Address))
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

func (m *MkBoot) checkDistDir(os, version, arch string) (string, error) {
	log.Printf("checkDistDir: os=%s version=%s arch=%s\n", os, version, arch)
	// generate error if version/arch not present
	distDir := filepath.Join("dist", os, version, arch)
	log.Printf("checking distDir: %s\n", distDir)
	if !files.IsDirFS(template.Dist, distDir) {
		return "", Fatalf("unsupported: %s %s %s", os, version, arch)
	}
	return distDir, nil
}

func DefaultDist(os string) (string, string, string, error) {
	distDir := path.Join("dist", os)
	versionEntries, err := fs.ReadDir(template.Dist, distDir)
	if err != nil {
		return "", "", "", Fatal(err)
	}
	vmap := make(map[string]string)
	keys := []string{}
	for _, entry := range versionEntries {
		parts := VERSION_PATTERN.FindStringSubmatch(entry.Name())
		if len(parts) == 4 {
			major, err := strconv.Atoi(parts[1])
			if err != nil {
				return "", "", "", Fatal(err)
			}
			minor, err := strconv.Atoi(parts[2])
			if err != nil {
				return "", "", "", Fatal(err)
			}
			version, err := strconv.Atoi(parts[3])
			if err != nil {
				return "", "", "", Fatal(err)
			}
			key := fmt.Sprintf("%04d.%04d.%04d", major, minor, version)
			keys = append(keys, key)
			vmap[key] = entry.Name()
		}
	}
	if len(keys) < 1 {
		return "", "", "", Fatalf("no dist dirs found for os: %s", os)
	}
	slices.Sort(keys)
	log.Printf("sorted_versions: %s\n", FormatJSON(keys))
	version := vmap[keys[0]]

	archEntries, err := fs.ReadDir(template.Dist, path.Join(distDir, version))
	if err != nil {
		return "", "", "", Fatal(err)
	}
	if len(archEntries) < 1 {
		return "", "", "", Fatalf("no arch dirs found for os:%s version:%s\n", os, version)
	}
	arch := archEntries[0].Name()

	var mirror string
	switch os {
	case "alpine":
		mirror = DEFAULT_ALPINE_MIRROR
	case "debian":
		mirror = DEFAULT_DEBIAN_MIRROR
	case "openbsd":
		mirror = DEFAULT_OPENBSD_MIRROR
	default:
		return "", "", "", Fatalf("unexpected os: %s\n", os)
	}

	log.Printf("DefaultDist(%s) returning version=%s arch=%s mirror=%s\n", os, version, arch, mirror)

	return version, arch, mirror, nil
}

func (m *MkBoot) mkbootOpenBSD() error {
	log.Printf("mkbootOpenBSD: %s %s\n", m.Config.Version, m.Config.Arch)

	_, err := m.checkDistDir("openbsd", m.Config.Version, m.Config.Arch)
	if err != nil {
		return Fatal(err)
	}

	// unzip template customized openbsd netboot img to /ipxe/MAC.boot
	// autoexec in iso image will sanboot /san/MAC.boot
	srcBoot := filepath.Join("ipxe", fmt.Sprintf("openbsd-%s-%s.img.gz", m.Config.Version, m.Config.Arch))
	dstBoot := filepath.Join(m.IpxeDir, m.Config.Address+".boot")
	err = files.UnzipFileFromFS(dstBoot, srcBoot, template.Ipxe)
	if err != nil {
		return Fatal(err)
	}
	log.Printf("mkbootOpenbsd: boot=%s\n", dstBoot)

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

	srcImage := "netboot.xyz.img.gz"
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
	distDir, err := m.checkDistDir("debian", m.Config.Version, m.Config.Arch)
	if err != nil {
		return Fatal(err)
	}

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
	err = m.writeDebianNetbootTarball()
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
	srcKernel := filepath.Join(distDir, "linux")
	dstKernel := filepath.Join(m.IpxeDir, m.Config.Address+".kernel")
	log.Printf("mkbootDebian: kernel=%s\n", dstKernel)
	err = files.CopyFileFromFS(dstKernel, srcKernel, template.Dist)
	if err != nil {
		return Fatal(err)
	}

	// copy the debian installer initrd: /ipxe/MAC.initrd
	srcInitrd := filepath.Join(distDir, "initrd.gz")
	dstInitrd := filepath.Join(m.IpxeDir, m.Config.Address+".initrd")
	log.Printf("mkbootDebian: initrd=%s\n", dstInitrd)
	err = files.CopyFileFromFS(dstInitrd, srcInitrd, template.Dist)
	if err != nil {
		return Fatal(err)
	}

	err = m.buildIMG("debian", "netboot.xyz.img.gz", true)
	if err != nil {
		return Fatal(err)
	}

	err = m.buildISO("debian")
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
		version, arch, mirror, err = DefaultDist("alpine")
		if err != nil {
			return Fatal(err)
		}
	}

	distDir, err := m.checkDistDir("alpine", version, arch)
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
	srcKernel := filepath.Join(distDir, "kernel")
	dstKernel := filepath.Join(m.IpxeDir, m.Config.Address+".kernel")
	log.Printf("mkbootAlpine: kernel=%s\n", dstKernel)
	err = files.CopyFileFromFS(dstKernel, srcKernel, template.Dist)
	if err != nil {
		return Fatal(err)
	}

	// copy the alpine netboot initrd: /ipxe/MAC.initrd
	srcInitrd := filepath.Join(distDir, "initrd")
	dstInitrd := filepath.Join(m.IpxeDir, m.Config.Address+".initrd")
	log.Printf("mkbootAlpine: initrd=%s\n", dstInitrd)
	err = files.CopyFileFromFS(dstInitrd, srcInitrd, template.Dist)
	if err != nil {
		return Fatal(err)
	}

	// copy the alpine netboot modloop: /ipxe/MAC.modloop
	srcModloop := filepath.Join(distDir, "modloop")
	dstModloop := filepath.Join(m.IpxeDir, m.Config.Address+".modloop")
	log.Printf("mkbootAlpine: modloop=%s\n", dstModloop)
	err = files.CopyFileFromFS(dstModloop, srcModloop, template.Dist)
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
		err = m.buildIMG(label, "netboot.xyz.img.gz", true)
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
	_, err := m.checkDistDir("windows", m.Config.Version, m.Config.Arch)
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
		err = m.injectBootFiles(dstImage)
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
	err := files.UnzipFileFromFS(srcIso, filepath.Join("ipxe", "netboot.xyz.iso.gz"), template.Ipxe)
	if err != nil {
		return Fatal(err)
	}

	// copy source EFI boot disk image for CreateEFIImage from IPXE template
	efiBin := filepath.Join(m.TempDir, "BOOTX64.EFI")
	err = files.UnzipFileFromFS(efiBin, filepath.Join("ipxe", "netboot.xyz.efi.gz"), template.Ipxe)
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

func CreateEFIImage(dstImage, efiBin, autoexec string) error {
	log.Printf("CreateEFIImage(%s, %s, %s)\n", dstImage, efiBin, autoexec)
	img, err := image.CreateImage(dstImage, "IPXE", "iPXE", 12, 1440*1024)
	if err != nil {
		return Fatal(err)
	}
	defer img.Close()
	err = img.Mkdir("EFI")
	if err != nil {
		return Fatal(err)
	}
	err = img.Mkdir("EFI/BOOT")
	if err != nil {
		return Fatal(err)
	}
	_, name := filepath.Split(efiBin)
	err = img.AddFile(path.Join("EFI", "BOOT", name), efiBin)
	if err != nil {
		return Fatal(err)
	}
	err = img.AddFile("autoexec.ipxe", autoexec)
	if err != nil {
		return Fatal(err)
	}
	return nil
}

func (m *MkBoot) injectBootFiles(fatImage string) error {
	log.Printf("injectBootFiles: %s\n", fatImage)
	image, err := image.OpenImage(fatImage)
	if err != nil {
		return Fatal(err)
	}
	defer image.Close()
	for i, injectPathname := range *m.BootFiles {
		_, name := filepath.Split(injectPathname)
		log.Printf("[%d] name=%s injectPath=%s\n", i, name, injectPathname)
		switch name {
		case "autoexec.ipxe.iso":
			name = ""
		case "autoexec.ipxe.img":
			name = "autoexec.ipxe"
		}
		if name != "" {
			log.Printf("injectBootFile %s -> %s\n", injectPathname, name)
			err = image.AddFile(name, injectPathname)
			if err != nil {
				return Fatal(err)
			}
		}
	}
	return nil
}

func dumpFAT(dstImage string) error {
	log.Printf("FAT files: %s\n", dstImage)
	image, err := image.OpenImage(dstImage)
	if err != nil {
		return Fatal(err)
	}
	defer image.Close()
	files, err := image.ScanFiles()
	if err != nil {
		return Fatal(err)
	}
	for i, file := range files {
		log.Printf("[%d] %+v\n", i, file)
	}
	return nil
}

func (m *MkBoot) writeDebianNetbootTarball() error {
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
	log.Printf("mkbootDebian: netbootTarball=%s\n", netBall)
	err = files.WriteTarball(netBall, filepath.Join(m.TempDir, "netboot"), true, []string{}, modes)
	if err != nil {
		return Fatal(err)
	}
	return nil
}
