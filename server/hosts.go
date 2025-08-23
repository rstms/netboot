package server

import (
	"bufio"
	"bytes"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
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

// mirrors used by netboot proxy feature
const DEFAULT_DEBIAN_MIRROR = "http://ftp.us.debian.org"
const DEFAULT_DEBIAN_SECURITY_MIRROR = "http://security.debian.org"
const DEFAULT_OPENBSD_MIRROR = "http://mirrors.mit.edu"
const DEFAULT_ALPINE_MIRROR = "https://dl-cdn.alpinelinux.org"

type Config struct {
	Address           string `json:"address"`
	OS                string `json:"os"`
	Version           string `json:"version"`
	Arch              string `json:"arch"`
	Serial            string `json:"serial"`
	Mirror            string `json:"mirror"`
	Response          string `json:"response"`
	DisklabelTemplate string `json:"disklabel_template"`
	KernelParams      string `json:"kernel_params"`
	Debug             bool   `json:"debug"`
	Quiet             bool   `json:"quiet"`
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
	ISO     string   `json: "iso"`
	Files   []string `json:"files"`
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
var ALPINE_VERSION_PATTERN = regexp.MustCompile(`([0-9][0-9]*)\.([0-9][0-9]*)\.([0-9][0-9]*)$`)
var APKOVL_PATTERN = regexp.MustCompile(`^([0-9A-Fa-f]{2}[:-]){5}([0-9A-Fa-f]{2})\.apkovl\.tar\.gz$`)

const htmlPrefix = `
<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
</head>
<body>
`

const htmlSuffix = `
</script>
</body>
</html>
`

type HostCache struct {
	cacheDir  string
	ipxeDir   string
	distDir   string
	cache     map[string]string
	httpPort  int
	httpsPort int
	proxy     bool
	template  *Template
}

func NewHostCache(dir string, template *Template) (*HostCache, error) {

	viper.SetDefault("netboot_server.mirror.alpine", DEFAULT_ALPINE_MIRROR)
	viper.SetDefault("netboot_server.mirror.debian", DEFAULT_DEBIAN_MIRROR)
	viper.SetDefault("netboot_server.mirror.debian-security", DEFAULT_DEBIAN_SECURITY_MIRROR)
	viper.SetDefault("netboot_server.mirror.openbsd", DEFAULT_OPENBSD_MIRROR)

	c := HostCache{
		cache:    make(map[string]string),
		cacheDir: dir,
		ipxeDir:  filepath.Join(dir, "ipxe"),
		distDir:  filepath.Join(dir, "dist"),
		template: template,
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

	dstImage := filepath.Join(c.ipxeDir, "ipxe.png")
	if !IsFile(dstImage) {
		srcImage := filepath.Join("ipxe", "ipxe.png")
		err := CopyFileFromFS(dstImage, srcImage, c.template.Ipxe)
		if err != nil {
			return nil, Fatal(err)
		}
	}

	return &c, nil
}

// expand macros in file from Ipxe template writing to dstPath
func (c *HostCache) expandIpxeFile(dstPathname, srcName, url, httpUrl string, config *Config) error {

	log.Printf("expandIpxeFile(%s, %s, %s, <config>)\n", dstPathname, srcName, url)

	src, err := c.template.Ipxe.Open(filepath.Join("ipxe", srcName))
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.Create(dstPathname)
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
		case strings.HasPrefix(line, "set netboot "):
			_, err = fmt.Fprintf(dst, "set netboot %s\n", url)
		case strings.HasPrefix(line, "set sanboot "):
			_, err = fmt.Fprintf(dst, "set sanboot %s\n", httpUrl)
		case strings.HasPrefix(line, "set kernel_params"):
			_, err = fmt.Fprintf(dst, "set kernel_params %s\n", config.KernelParams)
		case strings.HasPrefix(line, "set serial_params"):
			_, err = fmt.Fprintf(dst, "set serial_params %s\n", config.Serial)
		case strings.HasPrefix(line, "set mirror"):
			_, err = fmt.Fprintf(dst, "set mirror %s\n", config.Mirror)
		case strings.HasPrefix(line, "set branch"):
			match := ALPINE_VERSION_PATTERN.FindStringSubmatch(config.Version)
			if len(match) != 4 {
				return Fatalf("unexpected alpine version format: %s\n", config.Version)
			}
			branch := fmt.Sprintf("v%s.%s", match[1], match[2])
			_, err = fmt.Fprintf(dst, "set branch %s\n", branch)
		case strings.HasPrefix(line, "set version"):
			_, err = fmt.Fprintf(dst, "set version %s\n", config.Version)
		case strings.HasPrefix(line, "set arch"):
			_, err = fmt.Fprintf(dst, "set arch %s\n", config.Arch)
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

// return value for possible client access list validation
func (c *HostCache) validateHttpRequest(w http.ResponseWriter, r *http.Request) bool {
	log.Printf("Request: HTTP %s %s %s\n", r.RemoteAddr, r.Method, r.URL)
	return true
}

func (c *HostCache) requireClientCert(w http.ResponseWriter, r *http.Request) bool {
	cert := c.checkClientCert(w, r)
	clientCN := "<no_client_cert>"
	if cert != nil {
		clientCN = cert.Subject.String()
	}
	log.Printf("Request: TLS %s %s %s %s\n", clientCN, r.RemoteAddr, r.Method, r.URL.Path)
	if cert == nil {
		Warning("Missing client certificate: %s %s %s", r.RemoteAddr, r.Method, r.URL)
		fail(w, "authorization failed", http.StatusForbidden)
		return false
	}
	return true
}

func (c *HostCache) optionalClientCert(w http.ResponseWriter, r *http.Request) *x509.Certificate {
	cert := c.checkClientCert(w, r)
	clientCN := "<no_client_cert>"
	if cert != nil {
		clientCN = cert.Subject.String()
	}
	log.Printf("Request: TLS %s %s %s %s\n", clientCN, r.RemoteAddr, r.Method, r.URL.Path)
	return cert
}

func (c *HostCache) checkClientCert(w http.ResponseWriter, r *http.Request) *x509.Certificate {
	switch {
	case r.TLS == nil:
	case r.TLS.PeerCertificates == nil || len(r.TLS.PeerCertificates) < 1:
	default:
		return r.TLS.PeerCertificates[0]
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
	if !c.validateHttpRequest(w, r) {
		return
	}
	c.rootHandler(w, r)
}

func (c *HostCache) RootHandlerTLS(w http.ResponseWriter, r *http.Request) {
	c.optionalClientCert(w, r)
	c.rootHandler(w, r)
}

func (c *HostCache) rootHandler(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/ipxe.png":
		http.ServeFile(w, r, filepath.Join(c.ipxeDir, "ipxe.png"))
		return
	}
	fail(w, "forbidden", http.StatusForbidden)
}

func (c *HostCache) VersionHandlerTLS(w http.ResponseWriter, r *http.Request) {
	c.optionalClientCert(w, r)
	_, err := w.Write([]byte(htmlPrefix + fmt.Sprintf("netboot server v%s", Version) + htmlSuffix))
	if err != nil {
		Warning("%v", Fatal(err))
		fail(w, "failed", http.StatusInternalServerError)
		return
	}
}

func (c *HostCache) UTCHandler(w http.ResponseWriter, r *http.Request) {
	if !c.validateHttpRequest(w, r) {
		return
	}
	c.utcHandler(w, r)
}

func (c *HostCache) UTCHandlerTLS(w http.ResponseWriter, r *http.Request) {
	c.optionalClientCert(w, r)
	c.utcHandler(w, r)
}

func (c *HostCache) utcHandler(w http.ResponseWriter, r *http.Request) {
	epochTime := time.Now().Unix()
	w.Header().Set("Content-Disposition", "attachment; filename=utc")
	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprintf(w, "%d\n", epochTime)
	log.Printf("Response:  [200] %d\n", epochTime)
}

func (c *HostCache) GDLHandlerTLS(w http.ResponseWriter, r *http.Request) {
	c.optionalClientCert(w, r)
	version := r.PathValue("version")
	arch := r.PathValue("arch")
	file := r.PathValue("file")
	if file != "gdl.tgz" {
		Warning("invalid gdl filename: %s", r.URL.Path)
		fail(w, "invalid path", http.StatusBadRequest)
		return
	}
	parts := regexp.MustCompile(`^([0-9]+)\.([0-9]+)$`).FindStringSubmatch(version)
	if len(parts) != 3 {
		Warning("invalid gdl version: %s", r.URL.Path)
		fail(w, "invalid path", http.StatusBadRequest)
		return
	}
	major := parts[1]
	minor := parts[2]
	gdlPathname := filepath.Join("dist", "openbsd", version, arch, fmt.Sprintf("gdl%s%s.tgz", major, minor))
	http.ServeFileFS(w, r, c.template.Dist, gdlPathname)
}

func (c *HostCache) checkPath(prefix string, w http.ResponseWriter, r *http.Request) (string, bool) {
	dir, name := path.Split(r.URL.Path)
	if dir != prefix {
		fail(w, "invalid path", http.StatusBadRequest)
		return "", false

	}
	base, ext, ok := strings.Cut(name, ".")
	if !ok {
		Warning("filename missing exension: %s", r.URL.Path)
		fail(w, "invalid path", http.StatusBadRequest)
		return "", false
	}
	if !MAC_PATTERN.MatchString(base) {
		Warning("filename not MAC address: %s", r.URL.Path)
		fail(w, "invalid path", http.StatusBadRequest)
		return "", false
	}
	if APKOVL_PATTERN.MatchString(base) {
		ext = "apkovl.tar.gz"
	}
	return strings.ToLower(ext), true
}

func (c *HostCache) SanBootHandler(w http.ResponseWriter, r *http.Request) {
	if !c.validateHttpRequest(w, r) {
		return
	}
	ext, ok := c.checkPath("/san/", w, r)
	if !ok {
		return
	}
	switch ext {
	case "boot", "modloop", "apkovl.tar.gz":
		// IPXE sanboot command from the OpenBSD autoexec.ipxe
		// FIXME: verify that this request follows a very recent client-cert validated request for MAC.iso from the same source
		// it is possible that the ipxe client will be reusing the same session - see if we can set a cookie and check for it
		http.ServeFile(w, r, filepath.Join(c.cacheDir, strings.ReplaceAll(r.URL.Path, "/san/", "/ipxe/")))
		return
	}
	Warning("unexpected sanboot request: %s", r.URL.Path)
	fail(w, "invalid path", http.StatusBadRequest)
}

func (c *HostCache) PNGHandlerTLS(w http.ResponseWriter, r *http.Request) {
	if !c.requireClientCert(w, r) {
		return
	}
	http.ServeFileFS(w, r, c.template.Ipxe, "/ipxe/netboot.png")
}

func (c *HostCache) IPXEHandlerTLS(w http.ResponseWriter, r *http.Request) {
	if !c.requireClientCert(w, r) {
		return
	}
	ext, ok := c.checkPath("/ipxe/", w, r)
	if !ok {
		return
	}
	switch ext {
	case "iso", "ipxe", "kernel", "initrd", "response", "tgz", "disk", "cacerts", "postinstall":
		http.ServeFile(w, r, filepath.Join(c.cacheDir, r.URL.Path))
		return
	}
	Warning("unexpected ipxe request: %s", r.URL.Path)
	fail(w, "invalid path", http.StatusBadRequest)
}

func (c *HostCache) proxyHandler(mirror string, w http.ResponseWriter, r *http.Request) {

	if !c.proxy {
		Warning("disabled proxy received request: %s", mirror)
		fail(w, "netboot proxy disabled", http.StatusNotImplemented)
		return
	}

	// FIXME: add mechanism for verifying cached assets are in sync with distribution mirrors

	if !IsDir(c.distDir) {
		err := os.MkdirAll(c.distDir, 0700)
		if err != nil {
			Warning("%v", Fatal(err))
			fail(w, "configuration error", http.StatusInternalServerError)
			return
		}
	}
	cacheRoot, err := os.OpenRoot(c.distDir)
	if err != nil {
		Warning("%v", Fatal(err))
		fail(w, "configuration error", http.StatusInternalServerError)
		return
	}

	mirrorUrl := viper.GetString("netboot_server.mirror." + mirror)
	if mirrorUrl == "" {
		Warning("No mirror configured: %s", mirror)
		fail(w, "no mirror configured", http.StatusNotImplemented)
		return
	}
	pathname, data, err := DownloadFileRoot(cacheRoot, mirrorUrl, r.URL.Path)
	if err != nil {
		Warning("Proxy '%s' failed: %v", r.URL.Path, err)
		fail(w, "proxy failure", http.StatusBadGateway)
		return
	}
	switch {
	case pathname != "":
		http.ServeFileFS(w, r, cacheRoot.FS(), pathname)
		return
	case len(data) > 0:
		buf := bytes.NewBuffer(data)
		count, err := io.Copy(w, buf)
		if err != nil {
			Warning("%v", Fatal(err))
			fail(w, "failed writing response", http.StatusInternalServerError)
			return
		}
		log.Printf("Relayed %d bytes\n", count)
		return
	}
	Warning("DownloadFileRoot returned no data for '%s'", r.URL.Path)
}

func (c *HostCache) DebianHandler(w http.ResponseWriter, r *http.Request) {
	if !c.validateHttpRequest(w, r) {
		return
	}
	c.proxyHandler("debian", w, r)
}

func (c *HostCache) DebianHandlerTLS(w http.ResponseWriter, r *http.Request) {
	c.optionalClientCert(w, r)
	c.proxyHandler("debian", w, r)
}

func (c *HostCache) DebianSecurityHandler(w http.ResponseWriter, r *http.Request) {
	if !c.validateHttpRequest(w, r) {
		return
	}
	c.proxyHandler("debian-security", w, r)
}

func (c *HostCache) DebianSecurityHandlerTLS(w http.ResponseWriter, r *http.Request) {
	c.optionalClientCert(w, r)
	c.proxyHandler("debian-security", w, r)
}

func (c *HostCache) OpenBSDHandler(w http.ResponseWriter, r *http.Request) {
	if !c.validateHttpRequest(w, r) {
		return
	}
	c.proxyHandler("openbsd", w, r)
}

func (c *HostCache) OpenBSDHandlerTLS(w http.ResponseWriter, r *http.Request) {
	c.optionalClientCert(w, r)
	c.proxyHandler("openbsd", w, r)
}

func (c *HostCache) AlpineHandler(w http.ResponseWriter, r *http.Request) {
	if !c.validateHttpRequest(w, r) {
		return
	}
	c.proxyHandler("alpine", w, r)
}

func (c *HostCache) AlpineHandlerTLS(w http.ResponseWriter, r *http.Request) {
	c.optionalClientCert(w, r)
	c.proxyHandler("alpine", w, r)
}

func (c *HostCache) UploadPackageHandlerTLS(w http.ResponseWriter, r *http.Request) {
	if !c.requireClientCert(w, r) {
		return
	}

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

	packageFile, err := os.Create(filepath.Join(c.ipxeDir, packageFilename))
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

// copy file to temp dir, add to bootfiles, emit log message
func addBootFile(tempDir, filename, srcPathname string, bootFiles []string) error {
	dstPathname := filepath.Join(tempDir, filename)
	err := CopyFile(dstPathname, srcPathname)
	if err != nil {
		return err
	}
	bootFiles = append(bootFiles, dstPathname)
	log.Printf("adding boot file %s -> %s\n", srcPathname, dstPathname)
	return nil
}

func (c *HostCache) mkURLs(r *http.Request) (string, string) {
	httpsURL := "https://" + r.Host
	httpURL := "http://" + r.URL.Hostname()
	host, _, ok := strings.Cut(r.Host, ":")
	if ok {
		httpURL = fmt.Sprintf("http://%s:%d", host, c.httpPort)
	}
	return httpsURL, httpURL
}

func (c *HostCache) AddHostHandlerTLS(w http.ResponseWriter, r *http.Request) {
	if !c.requireClientCert(w, r) {
		return
	}
	var config Config
	err := json.NewDecoder(r.Body).Decode(&config)
	if err != nil {
		fail(w, err.Error(), http.StatusBadRequest)
		return
	}

	if !MAC_PATTERN.MatchString(config.Address) {
		fail(w, "invalid MAC address", http.StatusBadRequest)
		return
	}

	log.Printf("adding host %s %s %s\n", config.Address, config.OS, config.Version)

	log.Printf("AddHostHandler setting MAC=%s IP=''\n", config.Address)
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

	netbootURL, netbootHttpURL := c.mkURLs(r)

	// create temp directory for files used to generate EFI image and ISO
	tempDir, err := os.MkdirTemp("", "netboot-*")
	if err != nil {
		Warning("%v", Fatal(err))
		fail(w, "failed generating netboot files", http.StatusInternalServerError)
		return
	}
	// FIXME
	//defer os.RemoveAll(tempDir)

	bootFiles := []string{}

	// IPXE menu: <TEMP_DIR>/autoexec.ipxe (to be ebedded in ISO)
	autoexec := filepath.Join(tempDir, "autoexec.ipxe")
	err = c.expandIpxeFile(autoexec, config.OS+"-autoexec.ipxe", netbootURL, netbootHttpURL, &config)
	if err != nil {
		Warning("%v", Fatal(err))
		fail(w, "failed generating ipxe menu", http.StatusInternalServerError)
		return
	}

	// installer response file: /ipxe/MAC.response
	responsePathname := filepath.Join(c.ipxeDir, fmt.Sprintf("%s.response", config.Address))
	decodedBytes, err := base64.StdEncoding.DecodeString(config.Response)
	if err != nil {
		Warning("%v", Fatal(err))
		fail(w, "failed decoding response file", http.StatusInternalServerError)
		return
	}
	err = os.WriteFile(responsePathname, decodedBytes, 0660)
	if err != nil {
		Warning("%v", Fatal(err))
		fail(w, "failed writing response file", http.StatusInternalServerError)
		return
	}

	// disk partitioning template: /ipxe/MAC.disk
	disklabelTemplatePathname := filepath.Join(c.ipxeDir, fmt.Sprintf("%s.disk", config.Address))
	log.Printf("disklabelTemplatePathname: %s\n", disklabelTemplatePathname)
	if config.DisklabelTemplate != "" {
		decodedBytes, err := base64.StdEncoding.DecodeString(config.DisklabelTemplate)
		if err != nil {
			Warning("%v", Fatal(err))
			fail(w, "failed decoding partition template", http.StatusInternalServerError)
			return
		}
		err = os.WriteFile(disklabelTemplatePathname, decodedBytes, 0660)
		if err != nil {
			Warning("%v", Fatal(err))
			fail(w, "failed writing partition template", http.StatusInternalServerError)
			return
		}
	}

	// tarball: /ipxe/MAC.tgz
	//tarballPathname := filepath.Join(c.ipxeDir, fmt.Sprintf("%s.tgz", config.Address))
	//addBootFile(tempDir, "package.tgz", tarballPathname, bootFiles)

	// netboot: /ipxe/MAC.iso
	isoFile, err := c.GenerateISO(tempDir, netbootURL, netbootHttpURL, bootFiles, &config)
	if err != nil {
		Warning("%v", Fatal(err))
		fail(w, "failed generating boot ISO", http.StatusInternalServerError)
		return
	}

	respond(w, AddResponse{Message: fmt.Sprintf("%s configured", config.Address), ISO: booturl(netbootURL, isoFile), Files: bootFiles})
}

func booturl(urlBase, pathname string) string {
	_, name := filepath.Split(pathname)
	return urlBase + "/" + path.Join("ipxe", name)
}

func (c *HostCache) deleteHostFiles(address string) ([]string, error) {
	log.Printf("deleteHostFiles: %s\n", address)
	deletedFiles := []string{}
	files, err := ioutil.ReadDir(c.ipxeDir)
	if err != nil {
		return []string{}, Fatal(err)
	}
	address = strings.ReplaceAll(strings.ToLower(address), ":", "[:-]")
	pattern, err := regexp.Compile(fmt.Sprintf(`^%s\..*$`, address))
	if err != nil {
		return []string{}, Fatal(err)
	}

	for _, file := range files {
		filename := file.Name()
		if pattern.MatchString(strings.ToLower(filename)) {
			err := os.Remove(filepath.Join(c.ipxeDir, filename))
			if err != nil {
				return []string{}, Fatal(err)
			}
			deletedFiles = append(deletedFiles, filename)
		}
	}
	return deletedFiles, nil
}

func (c *HostCache) DeleteHostHandlerTLS(w http.ResponseWriter, r *http.Request) {
	if !c.requireClientCert(w, r) {
		return
	}
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
	log.Printf("DeleteHostHandler setting MAC=%s IP=''\n", request.Address)
	c.cache[request.Address] = ""
	c.deleteAddressFiles(request.Address, w)
}

func (c *HostCache) deleteAddressFiles(inAddress string, w http.ResponseWriter) {
	log.Printf("deleteAddressFiles: %s\n", inAddress)
	if viper.GetBool("netboot_server.no_delete_ipxe") {
		return
	}
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

func (c *HostCache) HostBootedHandlerTLS(w http.ResponseWriter, r *http.Request) {
	if !c.requireClientCert(w, r) {
		return
	}
	address := r.PathValue("mac")
	ip := r.PathValue("ip")
	switch {
	case address == "":
	case ip == "":
	default:
		log.Printf("HostBootedHandler setting MAC=%s IP=%s\n", address, ip)
		c.cache[address] = ip
		c.deleteAddressFiles(address, w)
		return
	}
	fail(w, "invalid path", http.StatusBadRequest)
}

func (c *HostCache) HostAddressQueryHandlerTLS(w http.ResponseWriter, r *http.Request) {
	if !c.requireClientCert(w, r) {
		return
	}
	segments := strings.Split(r.URL.Path, "/")
	if len(segments) > 3 {
		address := segments[3]
		log.Printf("HostAddressQueryHandler returning MAC=%s IP=%s\n", address, c.cache[address])
		respond(w, HostAddressResponse{MAC: address, IP: c.cache[address]})
		return
	}
	fail(w, "invalid path", http.StatusBadRequest)
}

func (c *HostCache) hostAddresses() ([]string, error) {
	addresses := []string{}
	files, err := ioutil.ReadDir(c.ipxeDir)
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

func (c *HostCache) ListHostsHandlerTLS(w http.ResponseWriter, r *http.Request) {
	if !c.requireClientCert(w, r) {
		return
	}
	addresses, err := c.hostAddresses()
	if err != nil {
		fail(w, err.Error(), http.StatusBadRequest)
		return
	}

	respond(w, HostListResponse{Message: fmt.Sprintf("config count: %d", len(addresses)), Addresses: addresses})
}

// Generate a url-customized netboot ISO returning generated iso pathname
func (c *HostCache) GenerateISO(tempDir, url, httpUrl string, bootFiles []string, config *Config) (string, error) {

	log.Printf("Generating netboot ISO for %s with URL %s\n", config.Address, url)

	tarball := filepath.Join(c.ipxeDir, config.Address+".tgz")

	// add root CA from tarball to bootFiles
	clientCA := filepath.Join(tempDir, "keymaster.pem")
	err := ExtractTarballFile(clientCA, "etc/ssl/keymaster.pem", tarball)
	if err != nil {
		return "", Fatal(err)
	}
	bootFiles = append(bootFiles, clientCA)

	// add client cert from tarball to bootFiles
	clientCert := filepath.Join(tempDir, "netboot.pem")
	err = ExtractTarballFile(clientCert, "etc/ssl/netboot.pem", tarball)
	if err != nil {
		return "", Fatal(err)
	}
	bootFiles = append(bootFiles, clientCert)

	// add client cert key from tarball to bootFiles
	clientKey := filepath.Join(tempDir, "netboot.key")
	err = ExtractTarballFile(clientKey, "etc/ssl/netboot.key", tarball)
	if err != nil {
		return "", Fatal(err)
	}
	bootFiles = append(bootFiles, clientKey)

	// extract tarball netboot_exec to temp dir for iso
	netbootExec := filepath.Join(tempDir, "netboot_exec")
	err = ExtractTarballFile(netbootExec, "root/netboot_exec", tarball)
	if err != nil {
		return "", Fatal(err)
	}
	bootFiles = append(bootFiles, netbootExec)

	// write netboot.env to temp dir for iso
	netbootEnvFile := filepath.Join(tempDir, "netboot.env")
	var netbootEnv string
	netbootEnv += fmt.Sprintf("_url=%s\n", url)
	netbootEnv += fmt.Sprintf("_mac=%s\n", config.Address)
	netbootEnv += fmt.Sprintf("_certs='--cacert /cdrom/keymaster.pem --cert /cdrom/netboot.pem --key /cdrom/netboot.key'\n")
	if config.Debug {
		netbootEnv += "_debug=1\n"
	} else {
		netbootEnv += "_debug=\n"
	}
	if config.Quiet {
		netbootEnv += "_quiet=1\n"
	} else {
		netbootEnv += "_quiet=\n"
	}
	err = os.WriteFile(netbootEnvFile, []byte(netbootEnv), 0644)
	if err != nil {
		return "", Fatal(err)
	}
	bootFiles = append(bootFiles, netbootEnvFile)

	// extract tarball postinstall to temp dir for debian mkboot
	// NOTE: intentionally not added to bootFiles
	postinstall := filepath.Join(tempDir, "postinstall")
	err = ExtractTarballFile(postinstall, "postinstall", tarball)
	if err != nil {
		return "", Fatal(err)
	}

	mkboot := NewMkBoot(tempDir, c.ipxeDir, url, bootFiles, config, c.template)
	isoFile, err := mkboot.Generate()
	if err != nil {
		return "", Fatal(err)
	}
	log.Printf("Generated %s\n", isoFile)
	return isoFile, nil

}
