package server

import (
	"bufio"
	"embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/rstms/netboot/bootimg"
	"github.com/rstms/netboot/bootiso"
	"github.com/rstms/netboot/template"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

type Config struct {
	Address           string `json:"address"`
	OS                string `json:"os"`
	Version           string `json:"version"`
	Arch              string `json:"arch"`
	Serial            string `json:"serial"`
	Config            string `json:"config"`
	DisklabelTemplate string `json:"disklabel_template"`
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

var MAC_PATTERN = regexp.MustCompile(`^([0-9A-Fa-f]{2}:){5}([0-9A-Fa-f]{2})$`)
var IPXE_PATTERN = regexp.MustCompile(`^([0-9A-Fa-f]{2}:){5}([0-9A-Fa-f]{2})\.ipxe$`)
var KERNEL_PATTERN = regexp.MustCompile(`^([0-9A-Fa-f]{2}:){5}([0-9A-Fa-f]{2}).kernel$`)
var INITRD_PATTERN = regexp.MustCompile(`^([0-9A-Fa-f]{2}:){5}([0-9A-Fa-f]{2}).initrd$`)
var RESPONSE_PATTERN = regexp.MustCompile(`^([0-9A-Fa-f]{2}:){5}([0-9A-Fa-f]{2}).conf$`)
var ALPINE_PATTERN = regexp.MustCompile(`^([0-9A-Fa-f]{2}:){5}([0-9A-Fa-f]{2}).apkvol.tar.gz$`)
var TARBALL_PATTERN = regexp.MustCompile(`^([0-9A-Fa-f]{2}:){5}([0-9A-Fa-f]{2})\.tgz$`)

type HostCache struct {
	dir   string
	cache map[string]string
}

func NewHostCache(dir string) (*HostCache, error) {
	c := HostCache{
		cache: make(map[string]string),
		dir:   dir,
	}
	if !IsDir(dir) {
		err := os.Mkdir(dir, 0700)
		if err != nil {
			return nil, err
		}
	}
	/*
		// copy template files to cache
		err := c.updateCache(template.Netboot, "netboot")
		if err != nil {
			return nil, err
		}
	*/
	return &c, nil
}

func (c *HostCache) updateCache(fs embed.FS, templateDir string) error {
	files, err := fs.ReadDir(templateDir)
	if err != nil {
		return err
	}
	for _, file := range files {
		fmt.Printf("file: %v\n", file)
		srcPath := filepath.Join(templateDir, file.Name())
		dstPath := filepath.Join(c.dir, file.Name())
		if !IsFile(dstPath) {
			err := c.copyTemplateFile(fs, dstPath, srcPath)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *HostCache) copyTemplateFile(fs embed.FS, dstPath, srcPath string) error {
	srcFile, err := fs.Open(srcPath)
	if err != nil {
		return err
	}
	defer srcFile.Close()
	dstFile, err := os.Create(dstPath)
	if err != nil {
		return err
	}
	defer dstFile.Close()
	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return err
	}
	return nil
}

func (c *HostCache) copyIPXETemplate(dstFilename, srcFilename, url string, config *Config) error {

	log.Printf("copyIPXETemplate(%s, %s, %s, <config>)\n", dstFilename, srcFilename, url)

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
		case strings.HasPrefix(line, "sanboot "):
			// openbsd IPXE menu
			versionTag := strings.ReplaceAll(config.Version, ".", "")
			_, err = fmt.Fprintf(dst, "sanboot %s/pub/OpenBSD/%s/%s/netboot%s.img", url, config.Version, config.Arch, versionTag)
		case strings.HasPrefix(line, "set debian_mirror "):
			// debian IPXE menu
			_, err = fmt.Fprintf(dst, "set debian_mirror %s\n", url)
		case strings.HasPrefix(line, "set netboot "):
			// debian IPXE menu
			_, err = fmt.Fprintf(dst, "set netboot %s\n", url)
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

func copyFile(dstPath, srcPath string) error {
	srcFile, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer srcFile.Close()
	dstFile, err := os.Create(dstPath)
	if err != nil {
		return err
	}
	defer dstFile.Close()
	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return err
	}
	return nil
}

func fail(w http.ResponseWriter, message string, status int) {
	log.Printf("Fail:  [%d] %s", status, message)
	http.Error(w, message, status)
}

func respond(w http.ResponseWriter, response any) {
	log.Printf("Response:  [200] %v", response)

	json.NewEncoder(w).Encode(response)
}

func (c *HostCache) RootHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("RootHandler: %s %s\n", r.Method, r.URL)
	fail(w, "unknown", 404)
}

func (c *HostCache) NetbootISOHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("NetbootISOHandler: %s %s\n", r.Method, r.URL)
	netbootURL := "https://" + r.Host
	isoFile, err := c.IsoFile(netbootURL, nil)
	if err != nil {
		fail(w, "iso not found", 404)
	}
	http.ServeFile(w, r, isoFile)
	return
}

func (c *HostCache) IPXEHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("IPXEHandler: %s %s\n", r.Method, r.URL)
	dir, name := path.Split(r.URL.Path)
	if dir != "/ipxe/" {
		fail(w, "invalid path", http.StatusBadRequest)
	}
	log.Printf("name=%s\n", name)
	switch {
	case IPXE_PATTERN.MatchString(name):
	case KERNEL_PATTERN.MatchString(name):
	case INITRD_PATTERN.MatchString(name):
	case RESPONSE_PATTERN.MatchString(name):
	case TARBALL_PATTERN.MatchString(name):
	case ALPINE_PATTERN.MatchString(name):
	default:
		fail(w, fmt.Sprintf("unexpected file request: %s", r.URL.Path), 404)
	}
	http.ServeFile(w, r, filepath.Join(c.dir, name))
}

func (c *HostCache) DebianHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("DebianHandler: %s %s\n", r.Method, r.URL)
	http.ServeFileFS(w, r, template.Debian, r.URL.Path)
}

func (c *HostCache) OpenBSDHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("OpenBSDHandler: %s %s\n", r.Method, r.URL)
	fail(w, "unknown", 404)
	//http.ServeFileFS(w, r, template.Debian, r.URL.Path)
}

func (c *HostCache) UploadPackageHandler(w http.ResponseWriter, r *http.Request) {

	fmt.Printf("UploadPackageHandler\n")

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

	fmt.Printf("AddHostHandler\n")
	var olines []string
	var in Config
	err := json.NewDecoder(r.Body).Decode(&in)
	if err != nil {
		fail(w, err.Error(), http.StatusBadRequest)
		return
	}

	fmt.Printf("Config: %s\n", FormatJSON(in))

	if !MAC_PATTERN.MatchString(in.Address) {
		fail(w, "invalid MAC address", http.StatusBadRequest)
		return
	}

	c.cache[in.Address] = ""

	switch in.OS {
	case "debian":
	case "openbsd":
	case "alpine":
	default:
		fail(w, "unrecognized OS", http.StatusBadRequest)
		return
	}

	srcMenuFilename := "rstms-netboot-" + in.OS + ".ipxe"
	hostMenuPathname := filepath.Join(c.dir, fmt.Sprintf("%s.ipxe", in.Address))
	responsePathname := filepath.Join(c.dir, fmt.Sprintf("%s.conf", in.Address))
	disklabelTemplatePathname := filepath.Join(c.dir, fmt.Sprintf("%s.disklabel_template", in.Address))
	tarballPathname := filepath.Join(c.dir, fmt.Sprintf("%s.tgz", in.Address))

	netbootURL := "https://" + r.Host

	err = c.copyIPXETemplate(hostMenuPathname, srcMenuFilename, netbootURL, &in)
	if err != nil {
		fail(w, err.Error(), http.StatusBadRequest)
		return
	}
	olines = append(olines, "generated IPXE menu")

	decodedBytes, err := base64.StdEncoding.DecodeString(in.Config)
	if err != nil {
		fail(w, err.Error(), http.StatusBadRequest)
		return
	}

	err = os.WriteFile(responsePathname, decodedBytes, 0660)
	if err != nil {
		fail(w, err.Error(), http.StatusBadRequest)
		return
	}

	if in.DisklabelTemplate != "" {
		err = os.WriteFile(disklabelTemplatePathname, []byte(in.DisklabelTemplate+"\n"), 0660)
		if err != nil {
			fail(w, err.Error(), http.StatusBadRequest)
			return
		}
		olines = append(olines, "generated disklabel template")
	}

	fmt.Printf("responsePathname: %s\n", responsePathname)
	fmt.Printf("hostMenuPathname: %s\n", hostMenuPathname)
	fmt.Printf("disklabelTemplatePathname: %s\n", disklabelTemplatePathname)
	fmt.Printf("tarballPathname: %s\n", tarballPathname)

	mkboot := NewMkBoot(&in, c.dir, tarballPathname, responsePathname, disklabelTemplatePathname, netbootURL)
	err = mkboot.Generate()
	if err != nil {
		fail(w, err.Error(), http.StatusBadRequest)
	}
	olines = append(olines, "generated OS boot resources")
	_, err = c.IsoFile(netbootURL, &in)
	if err != nil {
		fail(w, err.Error(), http.StatusBadRequest)
	}
	olines = append(olines, "generated netboot ISO")
	respond(w, AddResponse{Message: fmt.Sprintf("%s configured", in.Address), Output: olines})
}

func (c *HostCache) deleteHostFiles(address string) ([]string, error) {

	fmt.Printf("deleteHostFiles: %s\n", address)

	deletedFiles := []string{}
	files, err := ioutil.ReadDir(c.dir)
	if err != nil {
		return []string{}, err
	}
	pattern, err := regexp.Compile(fmt.Sprintf("^%s.*$", strings.ToLower(address)))
	if err != nil {
		return []string{}, err
	}

	for _, file := range files {
		filename := file.Name()
		if pattern.MatchString(strings.ToLower(filename)) {
			err := os.Remove(filepath.Join(c.dir, filename))
			if err != nil {
				return []string{}, err
			}
			deletedFiles = append(deletedFiles, filename)
		}
	}
	return deletedFiles, nil
}

func (c *HostCache) DeleteHostHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println("DeleteHostHandler")
	var in Host
	err := json.NewDecoder(r.Body).Decode(&in)
	if err != nil {
		fail(w, err.Error(), http.StatusBadRequest)
		return
	}

	if !MAC_PATTERN.MatchString(in.Address) {
		fail(w, "invalid MAC address", http.StatusBadRequest)
		return
	}
	c.cache[in.Address] = ""
	c.deleteAddressFiles(in.Address, w)
}

func (c *HostCache) deleteAddressFiles(inAddress string, w http.ResponseWriter) {
	fmt.Println("deleteAddressFiles")
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
	fmt.Println("HostBootedHandler")
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
	fmt.Println("HostAddressQueryHandler")
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
	fmt.Println("ListHostsHandler")
	addresses, err := c.hostAddresses()
	if err != nil {
		fail(w, err.Error(), http.StatusBadRequest)
		return
	}

	respond(w, HostListResponse{Message: fmt.Sprintf("config count: %d", len(addresses)), Addresses: addresses})
}

// return the netboot.iso filename the url, generating the ISO if not present
func (c *HostCache) IsoFile(url string, config *Config) (string, error) {
	// ensure an ISO file exixts for the url
	encoded := strings.ReplaceAll(strings.ReplaceAll(url, "/", "_"), ":", "_")
	isoDir := filepath.Join(c.dir, "iso", encoded)
	if !IsDir(isoDir) {
		err := os.MkdirAll(isoDir, 0700)
		if err != nil {
			return "", err
		}
	}
	isoFile := filepath.Join(isoDir, "netboot.iso")
	if !IsFile(isoFile) {
		if config == nil {
			return "", fmt.Errorf("cannot generate ISO in GET endpoint")
		}
		err := c.GenerateISO(url, isoDir, isoFile, config)
		if err != nil {
			return "", err
		}
	}
	return isoFile, nil
}

// Generate a url-customized netboot ISO
func (c *HostCache) GenerateISO(url, isoDir, isoFile string, config *Config) error {

	fmt.Printf("Generating netboot ISO for %s\n", url)

	// copy and customize the template autoexec.ipxe to the isoDir
	autoexec := filepath.Join(isoDir, "autoexec.ipxe")
	err := c.copyIPXETemplate(autoexec, "menu.ipxe", url, config)
	if err != nil {
		return err
	}
	//defer os.Remove(autoexec)

	efiBin := filepath.Join(isoDir, "BOOTX64.EFI")
	err = c.copyTemplateFile(template.Ipxe, efiBin, filepath.Join("ipxe", "netboot.xyx.efi"))
	if err != nil {
		return err
	}
	//defer os.Remove(efiBin)

	// generate the EFI boot disk image
	efiImage := filepath.Join(isoDir, "efi.img")
	err = bootimg.CreateEFIImage(efiImage, efiBin, autoexec)
	if err != nil {
		return err
	}
	//defer os.Remove(efiImage)

	srcIso := filepath.Join(isoDir, "netboot.xyz.iso")
	err = c.copyTemplateFile(template.Ipxe, srcIso, filepath.Join("ipxe", "netboot.xyz.iso"))
	if err != nil {
		return err
	}
	//defer os.Remove(srcIso)

	// generate the netboot ISO
	err = bootiso.CreateNetbootISOImage(isoFile, srcIso, efiImage, autoexec)
	if err != nil {
		return err
	}
	fmt.Printf("Generated %s\n", isoFile)
	return nil
}
