package server

import (
	"embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/rstms/netboot/template"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type Config struct {
	Address           string `json:"address"`
	OS                string `json:"os"`
	Version           string `json:"version"`
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
var PKG_PATTERN = regexp.MustCompile(`^([0-9A-Fa-f]{2}:){5}([0-9A-Fa-f]{2})\.tgz$`)

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
	// copy template files to cache
	err := c.updateCache(template.Netboot, "netboot")
	if err != nil {
		return nil, err
	}
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
	log.Printf("  [%d] %s", status, message)
	http.Error(w, message, status)
}

func respond(w http.ResponseWriter, response any) {
	log.Printf("  [200] %v", response)
	json.NewEncoder(w).Encode(response)
}

func (c *HostCache) DefaultHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("%s %s\n", r.Method, r.URL)
	fail(w, "unknown", 404)
}

func (c *HostCache) UploadPackageHandler(w http.ResponseWriter, r *http.Request) {

	fmt.Printf("UploadPackageHandler: %+v\n", *r)

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

	if !PKG_PATTERN.MatchString(packageFilename) {
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

	fmt.Printf("AddHostHandler: %+v\n", *r)

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

	osMenuPathname := filepath.Join(c.dir, fmt.Sprintf("netboot-%s.ipxe", in.OS))
	responsePathname := filepath.Join(c.dir, fmt.Sprintf("%s.conf", in.Address))
	hostMenuPathname := filepath.Join(c.dir, fmt.Sprintf("%s.ipxe", in.Address))
	disklabelTemplatePathname := filepath.Join(c.dir, fmt.Sprintf("%s.disklabel_template", in.Address))
	tarballPathname := filepath.Join(c.dir, fmt.Sprintf("%s.tgz", in.Address))

	err = copyFile(hostMenuPathname, osMenuPathname)
	if err != nil {
		fail(w, err.Error(), http.StatusBadRequest)
		return
	}

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
	}

	fmt.Printf("responsePathname: %s\n", responsePathname)
	fmt.Printf("hostMenuPathname: %s\n", hostMenuPathname)
	fmt.Printf("disklabelTemplatePathname: %s\n", disklabelTemplatePathname)
	fmt.Printf("tarballPathname: %s\n", tarballPathname)

	mkboot := NewMkBoot(&in, c.dir, tarballPathname, responsePathname, disklabelTemplatePathname)
	oLines, err := mkboot.Generate()
	if err != nil {
		fail(w, err.Error(), http.StatusBadRequest)
	}

	respond(w, AddResponse{Message: fmt.Sprintf("%s configured", in.Address), Output: oLines})
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
	fmt.Printf("DeleteHostHandler: %+v\n", *r)
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
	fmt.Printf("HostBootedHandler: %+v\n", *r)
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
	fmt.Printf("HostAddressQueryHandler: %+v\n", *r)
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
	fmt.Printf("ListHostsHandler: %+v\n", *r)
	addresses, err := c.hostAddresses()
	if err != nil {
		fail(w, err.Error(), http.StatusBadRequest)
		return
	}

	respond(w, HostListResponse{Message: fmt.Sprintf("config count: %d", len(addresses)), Addresses: addresses})
}
