package server

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/rstms/netboot/bootimg"
	"github.com/rstms/netboot/bootiso"
	"github.com/rstms/netboot/template"
	"github.com/spf13/viper"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const DEFAULT_DEBIAN_MIRROR = "http://ftp.us.debian.org"
const DEFAULT_OPENBSD_MIRROR = "https://mirrors.mit.edu"
const DEFAULT_ALPINE_MIRROR = "https://dl-cdn.alpinelinux.org"

type Config struct {
	Address           string `json:"address"`
	OS                string `json:"os"`
	Version           string `json:"version"`
	Arch              string `json:"arch"`
	Serial            string `json:"serial"`
	Config            string `json:"config"`
	DisklabelTemplate string `json:"disklabel_template"`
	KernelParams      string `json:"kernel_params"`
}

type Host struct {
	Address string `json:"address"`
}

type HostAddressResponse struct {
	MAC string `json:"mac"`
	IP  string `json:"ip"`
}

type Response struct {
	Message string `json:"message"`
}

type AddResponse struct {
	Message string   `json:"message"`
	Output  []string `json:"output"`
}

type HostListResponse struct {
	Message   string   `json:"message"`
	Addresses []string `json:"addresses"`
}

type DeleteResponse struct {
	Message string   `json:"message"`
	Files   []string `json:"files"`
}

var MAC_PATTERN = regexp.MustCompile(`^([0-9A-Fa-f]{2}[:-]){5}([0-9A-Fa-f]{2})$`)
var IPXE_PATTERN = regexp.MustCompile(`^([0-9A-Fa-f]{2}[:-]){5}([0-9A-Fa-f]{2})\.ipxe$`)
var TARBALL_PATTERN = regexp.MustCompile(`^([0-9A-Fa-f]{2}[:-]){5}([0-9A-Fa-f]{2})\.tgz$`)

/*
var ISO_PATTERN = regexp.MustCompile(`^([0-9A-Fa-f]{2}[:-]){5}([0-9A-Fa-f]{2})\.iso$`)
var KERNEL_PATTERN = regexp.MustCompile(`^([0-9A-Fa-f]{2}[:-]){5}([0-9A-Fa-f]{2}).kernel$`)
var INITRD_PATTERN = regexp.MustCompile(`^([0-9A-Fa-f]{2}[-:]){5}([0-9A-Fa-f]{2}).initrd$`)
var RESPONSE_PATTERN = regexp.MustCompile(`^([0-9A-Fa-f]{2}[:-]){5}([0-9A-Fa-f]{2}).conf$`)
var ALPINE_PATTERN = regexp.MustCompile(`^([0-9A-Fa-f]{2}[:-]){5}([0-9A-Fa-f]{2}).apkvol.tar.gz$`)
var TEMPLATE_PATTERN = regexp.MustCompile(`^([0-9A-Fa-f]{2}[:-]){5}([0-9A-Fa-f]{2})\.disklabel_template$`)
*/

type HostCache struct {
	dir     string
	ipxeDir string
	distDir string
	cache   map[string]string
}

func NewHostCache(dir string) (*HostCache, error) {

	viper.SetDefault("netboot.mirror.alpine", DEFAULT_ALPINE_MIRROR)
	viper.SetDefault("netboot.mirror.debian", DEFAULT_DEBIAN_MIRROR)
	viper.SetDefault("netboot.mirror.openbsd", DEFAULT_OPENBSD_MIRROR)

	c := HostCache{
		cache:   make(map[string]string),
		ipxeDir: filepath.Join(dir, "ipxe"),
		distDir: filepath.Join(dir, "dist"),
	}
	if !IsDir(c.ipxeDir) {
		err := os.MkdirAll(c.ipxeDir, 0700)
		if err != nil {
			return nil, Fatal(err)
		}
	}
	if !IsDir(c.distDir) {
		err := os.MkdirAll(c.distDir, 0700)
		if err != nil {
			return nil, Fatal(err)
		}
	}
	return &c, nil
}

func (c *HostCache) expandIpxeFile(dstFilename, srcFilename, url string, config *Config) error {

	log.Printf("expandIpxeFile(%s, %s, %s, <config>)\n", dstFilename, srcFilename, url)

	src, err := template.Ipxe.Open(filepath.Join("ipxe", srcFilename))
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.Create(dstFilename)
	if err != nil {
		return err
	}
	defer dst.Close()

	// These string replacements are applied to multiple classes of IPXE file
	// the prefixes are unique so far, but make sure we don't duplicate any
	// IPXE files are generated using https://github.com/rstms/rstms-netboot-xyz

	scanner := bufio.NewScanner(src)
	for scanner.Scan() {
		line := scanner.Text()
		var err error
		switch {
		// FIXME: need an alpine case
		case strings.HasPrefix(line, "chain --replace "):
			_, err = fmt.Fprintf(dst, "chain --replace %s/ipxe/${net0/mac}.ipxe || error", url)
		/*
			case strings.HasPrefix(line, "sanboot "):
				// openbsd IPXE menu
				versionTag := strings.ReplaceAll(config.Version, ".", "")
				_, err = fmt.Fprintf(dst, "sanboot %s/pub/OpenBSD/%s/%s/netboot%s.img", url, config.Version, config.Arch, versionTag)
		*/
		case strings.HasPrefix(line, "set debian_mirror "):
			// debian IPXE menu
			_, err = fmt.Fprintf(dst, "set debian_mirror %s\n", url)
		case strings.HasPrefix(line, "set netboot "):
			// debian, openbsd IPXE menu
			_, err = fmt.Fprintf(dst, "set netboot %s\n", url)
		case strings.HasPrefix(line, "set kernel_params "):
			// debian IPXE menu
			_, err = fmt.Fprintf(dst, "set kernel_params %s\n", config.KernelParams)
		default:
			_, err = dst.Write([]byte(line + "\n"))
		}
		if err != nil {
			return err
		}
	}
	err = scanner.Err()
	if err != nil {
		return err
	}
	return nil
}

func fail(w http.ResponseWriter, message string, status int) {
	log.Printf("Fail:  [%d] %s\n", status, message)
	http.Error(w, message, status)
}

func respond(w http.ResponseWriter, response any) {
	log.Printf("Response:  [200] %v\n", response)

	json.NewEncoder(w).Encode(response)
}

func (c *HostCache) RootHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("Request: %s %s\n", r.Method, r.URL)
	fail(w, "unknown", 404)
}

func (c *HostCache) UTCHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("Request: %s %s\n", r.Method, r.URL)
	epochTime := time.Now().Unix()
	w.Header().Set("Content-Disposition", "attachment; filename=utc")
	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprintf(w, "%d\n", epochTime)
	log.Printf("Response:  [200] %d\n", epochTime)
}

func (c *HostCache) GDLHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("Request: %s %s\n", r.Method, r.URL)
	version := r.PathValue("version")
	arch := r.PathValue("arch")
	file := r.PathValue("file")
	if file != "gdl.tgz" {
		Warning("invalid gdl filename: %s\n", r.URL.Path)
		fail(w, "invalid path", http.StatusBadRequest)
		return
	}
	parts := regexp.MustCompile(`^([0-9]+)\.([0-9]+)$`).FindStringSubmatch(version)
	if len(parts) != 3 {
		Warning("invalid gdl version: %s\n", r.URL.Path)
		fail(w, "invalid path", http.StatusBadRequest)
		return
	}
	major := parts[1]
	minor := parts[2]
	gdlPathname := filepath.Join("dist", "openbsd", version, arch, fmt.Sprintf("gdl%s%s.tgz", major, minor))
	http.ServeFileFS(w, r, template.Dist, gdlPathname)
}

func (c *HostCache) IPXEHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("Request: %s %s\n", r.Method, r.URL)
	dir, name := path.Split(r.URL.Path)
	if dir != "/ipxe/" {
		fail(w, "invalid path", http.StatusBadRequest)
		return

	}
	base, ext, ok := strings.Cut(name, ".")
	if !ok {
		Warning("filename missing exension: %s\n", r.URL.Path)
		fail(w, "invalid path", http.StatusBadRequest)
		return
	}
	if !MAC_PATTERN.MatchString(base) {
		Warning("filename not MAC address: %s\n", r.URL.Path)
		fail(w, "invalid path", http.StatusBadRequest)
		return
	}

	switch ext {
	case "iso":
	case "ipxe":
	case "kernel":
	case "initrd":
	case "conf":
	case "tgz":
	case "disk":
	case "apkvol.tar.gz":
	default:
		Warning("unexpected extension: %s\n", r.URL.Path)
		fail(w, fmt.Sprintf("invalid path"), http.StatusBadRequest)
		return
	}
	http.ServeFile(w, r, filepath.Join(c.dir, r.URL.Path))
}

func (c *HostCache) proxy(mirror, dir string, w http.ResponseWriter, r *http.Request) {

	path := filepath.Join(c.distDir, dir)
	filesystem, err := os.OpenRoot(path)
	if err != nil {
		Warning("%v\n", Fatal(err))
		fail(w, "configuration error", http.StatusInternalServerError)
		return
	}
	cacheFS := filesystem.FS()
	if !IsFileFS(cacheFS, r.URL.Path) {
		mirrorUrl := viper.GetString("netboot.mirror." + mirror)
		if mirrorUrl == "" {
			Warning("No mirror configured: %s", mirror)
			fail(w, "no mirror configured", http.StatusNotImplemented)
			return
		}
		err := downloadToFS(cacheFS, mirrorUrl, r.URL.Path)
		if err != nil {
			Warning("Download '%s' failed: %v\n", r.URL.Path, err)
			fail(w, err.Error(), http.StatusBadRequest)
			return
		}
	}
}

func (c *HostCache) DebianHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("Request: %s %s\n", r.Method, r.URL)
	c.proxy("debian", "debian", w, r)
}

func (c *HostCache) OpenBSDHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("Request: %s %s\n", r.Method, r.URL)
	c.proxy("openbsd", "pub", w, r)
}

func (c *HostCache) AlpineHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("Request: %s %s\n", r.Method, r.URL)
	c.proxy("alpine", "alpine", w, r)
}

func (c *HostCache) UploadPackageHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("Request: %s %s\n", r.Method, r.URL)

	err := r.ParseMultipartForm(256 << 20) // limit file size to 256MB
	if err != nil {
		fail(w, fmt.Sprintf("failed parsing form: %v", err), http.StatusBadRequest)
		return
	}

	uploadFile, fileHeader, err := r.FormFile("uploadFile")
	if err != nil {
		fail(w, fmt.Sprintf("failed retreiving upload file: %v", err), http.StatusBadRequest)
		return
	}
	defer uploadFile.Close()

	packageFilename := fileHeader.Filename

	if !TARBALL_PATTERN.MatchString(packageFilename) {
		fail(w, fmt.Sprintf("illegal filename: %s", packageFilename), http.StatusBadRequest)
		return
	}

	packageFile, err := os.Create(filepath.Join(c.dir, packageFilename))
	if err != nil {
		fail(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer packageFile.Close()
	fileBytes, err := io.Copy(packageFile, uploadFile)
	if err != nil {
		fail(w, err.Error(), http.StatusBadRequest)
		return
	}

	respond(w, Response{Message: fmt.Sprintf("%v bytes written", fileBytes)})
}

func (c *HostCache) AddHostHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("Request: %s %s\n", r.Method, r.URL)
	var olines []string
	var config Config
	err := json.NewDecoder(r.Body).Decode(&config)
	if err != nil {
		fail(w, err.Error(), http.StatusBadRequest)
		return
	}

	fmt.Printf("Config: %s\n", FormatJSON(config))

	if !MAC_PATTERN.MatchString(config.Address) {
		fail(w, "invalid MAC address", http.StatusBadRequest)
		return
	}

	c.cache[config.Address] = ""

	// FIXME: check that OS Version is supported
	switch config.OS {
	case "debian":
	case "openbsd":
	case "alpine":
	default:
		fail(w, "unrecognized OS", http.StatusBadRequest)
		return
	}

	netbootURL := "https://" + r.Host

	srcMenuFilename := "rstms-netboot-" + config.OS + ".ipxe"
	hostMenuPathname := filepath.Join(c.ipxeDir, fmt.Sprintf("%s.ipxe", config.Address))
	fmt.Printf("hostMenuPathname: %s\n", hostMenuPathname)
	err = c.expandIpxeFile(hostMenuPathname, srcMenuFilename, netbootURL, &config)
	if err != nil {
		fail(w, err.Error(), http.StatusBadRequest)
		return
	}
	olines = append(olines, "generated IPXE menu")

	responsePathname := filepath.Join(c.ipxeDir, fmt.Sprintf("%s.conf", config.Address))
	fmt.Printf("responsePathname: %s\n", responsePathname)
	decodedBytes, err := base64.StdEncoding.DecodeString(config.Config)
	if err != nil {
		fail(w, err.Error(), http.StatusBadRequest)
		return
	}
	err = os.WriteFile(responsePathname, decodedBytes, 0660)
	if err != nil {
		fail(w, err.Error(), http.StatusBadRequest)
		return
	}
	olines = append(olines, "generated response file")

	// disk partitioning template: MAC.disk
	disklabelTemplatePathname := filepath.Join(c.ipxeDir, fmt.Sprintf("%s.disk", config.Address))
	fmt.Printf("disklabelTemplatePathname: %s\n", disklabelTemplatePathname)
	if config.DisklabelTemplate != "" {
		err = os.WriteFile(disklabelTemplatePathname, []byte(config.DisklabelTemplate+"\n"), 0660)
		if err != nil {
			fail(w, err.Error(), http.StatusBadRequest)
			return
		}
		olines = append(olines, "generated disk partition template")
	}

	// tarball MAC.tgz
	tarballPathname := filepath.Join(c.ipxeDir, fmt.Sprintf("%s.tgz", config.Address))
	fmt.Printf("tarballPathname: %s\n", tarballPathname)
	mkboot := NewMkBoot(&config, c.dir, tarballPathname, responsePathname, disklabelTemplatePathname, netbootURL)
	err = mkboot.Generate()
	if err != nil {
		fail(w, err.Error(), http.StatusBadRequest)
		return
	}
	olines = append(olines, "generated package tarball")

	isoFile, err := c.GenerateISO(netbootURL, &config)
	if err != nil {
		fail(w, err.Error(), http.StatusBadRequest)
		return
	}
	_, isoName := filepath.Split(isoFile)
	isoUrl := path.Join(netbootURL, "ipxe", isoName)
	olines = append(olines, "iso: "+isoUrl)

	respond(w, AddResponse{Message: fmt.Sprintf("%s configured", config.Address), Output: olines})
}

func (c *HostCache) deleteHostFiles(address string) ([]string, error) {
	fmt.Printf("deleteAddressFiles: %s\n", address)
	deletedFiles := []string{}
	files, err := ioutil.ReadDir(c.ipxeDir)
	if err != nil {
		return []string{}, Fatal(err)
	}
	pattern, err := regexp.Compile(fmt.Sprintf("^%s.*$", strings.ToLower(address)))
	if err != nil {
		return []string{}, Fatal(err)
	}

	for _, file := range files {
		filename := file.Name()
		if pattern.MatchString(strings.ToLower(filename)) {
			err := os.Remove(filepath.Join(c.dir, filename))
			if err != nil {
				return []string{}, Fatal(err)
			}
			deletedFiles = append(deletedFiles, filename)
		}
	}
	return deletedFiles, nil
}

func (c *HostCache) DeleteHostHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("Request: %s %s\n", r.Method, r.URL)
	var request Host
	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		fail(w, err.Error(), http.StatusBadRequest)
		return
	}

	if !MAC_PATTERN.MatchString(request.Address) {
		fail(w, "invalid MAC address", http.StatusBadRequest)
		return
	}
	c.cache[request.Address] = ""
	c.deleteAddressFiles(request.Address, w)
}

func (c *HostCache) deleteAddressFiles(inAddress string, w http.ResponseWriter) {
	fmt.Printf("deleteAddressFiles: %s\n", inAddress)
	addresses, err := c.hostAddresses()
	if err != nil {
		fail(w, err.Error(), http.StatusBadRequest)
		return
	}
	for _, address := range addresses {
		if strings.ToLower(inAddress) == strings.ToLower(address) {
			files, err := c.deleteHostFiles(address)
			if err != nil {
				fail(w, err.Error(), http.StatusBadRequest)
				return
			}
			respond(w, DeleteResponse{Message: fmt.Sprintf("deleted: %d", len(files)), Files: files})
			return
		}
	}
	fail(w, "host address not found", http.StatusNotFound)
	return
}

func (c *HostCache) HostBootedHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("Request: %s %s\n", r.Method, r.URL)
	segments := strings.Split(r.URL.Path, "/")
	if len(segments) > 3 {
		address := segments[3]
		if len(segments) > 4 {
			ip := segments[4]
			c.cache[address] = ip
		}
		c.deleteAddressFiles(address, w)
		return

	}
	fail(w, "invalid path", http.StatusBadRequest)
}

func (c *HostCache) HostAddressQueryHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("Request: %s %s\n", r.Method, r.URL)
	segments := strings.Split(r.URL.Path, "/")
	if len(segments) > 3 {
		address := segments[3]
		respond(w, HostAddressResponse{MAC: address, IP: c.cache[address]})
		return
	}
	fail(w, "invalid path", http.StatusBadRequest)
}

func (c *HostCache) hostAddresses() ([]string, error) {
	addresses := []string{}
	files, err := ioutil.ReadDir(c.dir)
	if err != nil {
		return addresses, err
	}
	for _, file := range files {
		filename := file.Name()
		if IPXE_PATTERN.MatchString(filename) {
			fields := strings.Split(filename, ".")
			addresses = append(addresses, fields[0])
		}
	}
	return addresses, nil
}

func (c *HostCache) ListHostsHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("Request: %s %s\n", r.Method, r.URL)
	addresses, err := c.hostAddresses()
	if err != nil {
		fail(w, err.Error(), http.StatusBadRequest)
		return
	}

	respond(w, HostListResponse{Message: fmt.Sprintf("config count: %d", len(addresses)), Addresses: addresses})
}

// Generate a url-customized netboot ISO
func (c *HostCache) GenerateISO(url string, config *Config) (string, error) {

	fmt.Printf("Generating netboot ISO for %s with URL %s\n", config.Address, url)

	isoFile := filepath.Join(c.dir, fmt.Sprintf("%s.iso", strings.ReplaceAll(config.Address, ":", "-")))
	if IsFile(isoFile) {
		err := os.Remove(isoFile)
		if err != nil {
			return "", Fatal(err)
		}
	}

	isoDir, err := os.MkdirTemp("", "netboot_iso_*")
	if err != nil {
		return "", Fatal(err)
	}
	// FIXME: delete temp dir
	// defer os.RemoveAll(isoDir)

	// copy and customize the template autoexec.ipxe to the isoDir
	autoexec := filepath.Join(isoDir, "autoexec.ipxe")
	err = c.expandIpxeFile(autoexec, "menu.ipxe", url, config)
	if err != nil {
		return "", Fatal(err)
	}

	efiBin := filepath.Join(isoDir, "BOOTX64.EFI")
	err = CopyFileFromFS(efiBin, filepath.Join("ipxe", "netboot.xyz.efi"), template.Ipxe)
	if err != nil {
		return "", Fatal(err)
	}

	// generate the EFI boot disk image
	efiImage := filepath.Join(isoDir, "efi.img")
	err = bootimg.CreateEFIImage(efiImage, efiBin, autoexec)
	if err != nil {
		return "", Fatal(err)
	}

	srcIso := filepath.Join(isoDir, "netboot.xyz.iso")
	err = CopyFileFromFS(srcIso, filepath.Join("ipxe", "netboot.xyz.iso"), template.Ipxe)
	if err != nil {
		return "", Fatal(err)
	}

	// generate the netboot ISO
	err = bootiso.CreateNetbootISOImage(isoFile, srcIso, efiImage, autoexec)
	if err != nil {
		return "", Fatal(err)
	}
	fmt.Printf("Generated %s\n", isoFile)
	return isoFile, nil
}
