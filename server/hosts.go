package server

import (
	"bufio"
	"bytes"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/rstms/netboot/files"
	"github.com/rstms/netboot/message"
	"github.com/rstms/netboot/template"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
)

// mirrors used by netboot proxy feature
const DEFAULT_DEBIAN_MIRROR = "http://ftp.us.debian.org"
const DEFAULT_DEBIAN_SECURITY_MIRROR = "http://security.debian.org"
const DEFAULT_OPENBSD_MIRROR = "http://mirrors.mit.edu"
const DEFAULT_ALPINE_MIRROR = "https://dl-cdn.alpinelinux.org"

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

var MAC_PATTERN = regexp.MustCompile(`^([0-9A-Fa-f]{2}[:-]{0,1}){5}([0-9A-Fa-f]{2})$`)
var NORMALIZED_MAC_PATTERN = regexp.MustCompile(`^[0-9a-f]{12}$`)
var IPXE_PATTERN = regexp.MustCompile(`^([0-9A-Fa-f]{2}[:-]{0,1}){5}([0-9A-Fa-f]{2})\.ipxe$`)
var TARBALL_PATTERN = regexp.MustCompile(`^([0-9a-f]{12})\.tgz$`)
var ALPINE_VERSION_PATTERN = regexp.MustCompile(`([0-9][0-9]*)\.([0-9][0-9]*)\.([0-9][0-9]*)$`)
var APKOVL_PATTERN = regexp.MustCompile(`^([0-9A-Fa-f]{2}[:-]{0,1}){5}([0-9A-Fa-f]{2})\.apkovl\.tar\.gz$`)

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
	Name             string
	cacheDir         string
	ipxeDir          string
	distDir          string
	cache            map[string]string
	httpPort         int
	httpsPort        int
	proxy            bool
	noDeleteIpxe     bool
	mirrorUrl        map[string]string
	httpURL          string
	httpsURL         string
	whitelistCommand string
}

func NewHostCache(dir string, httpPort, httpsPort int, proxyEnabled bool) (*HostCache, error) {

	prefix := "netboot.server."
	if ProgramName() == "netboot" {
		prefix = "server."
	}

	ViperSetDefault(prefix+"mirror.alpine", DEFAULT_ALPINE_MIRROR)
	ViperSetDefault(prefix+"mirror.debian", DEFAULT_DEBIAN_MIRROR)
	ViperSetDefault(prefix+"mirror.debian-security", DEFAULT_DEBIAN_SECURITY_MIRROR)
	ViperSetDefault(prefix+"mirror.openbsd", DEFAULT_OPENBSD_MIRROR)

	c := HostCache{
		Name:         "netboot",
		cacheDir:     dir,
		httpPort:     httpPort,
		httpsPort:    httpsPort,
		proxy:        proxyEnabled,
		cache:        make(map[string]string),
		ipxeDir:      filepath.Join(dir, "ipxe"),
		distDir:      filepath.Join(dir, "dist"),
		noDeleteIpxe: ViperGetBool(prefix + "no_delete_ipxe"),
		mirrorUrl: map[string]string{
			"alpine":          ViperGetString(prefix + "mirror.alpine"),
			"debian":          ViperGetString(prefix + "mirror.debian"),
			"debian-security": ViperGetString(prefix + "mirror.debian-security"),
			"openbsd":         ViperGetString(prefix + "mirror.openbsd"),
		},
		httpURL:          ViperGetString(prefix + "http_url"),
		httpsURL:         ViperGetString(prefix + "https_url"),
		whitelistCommand: ViperGetString(prefix + "whitelist_command"),
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

	dstImage := filepath.Join(c.ipxeDir, "netboot.png")
	if !IsFile(dstImage) {
		srcImage := filepath.Join("ipxe", "netboot.png")
		err := files.CopyFileFromFS(dstImage, srcImage, template.Ipxe)
		if err != nil {
			return nil, Fatal(err)
		}
	}

	log.Printf("ipxe dir: %s\n", c.ipxeDir)

	return &c, nil
}

// expand macros in file from Ipxe template writing to dstPath
func (c *HostCache) expandIpxeFile(dstPathname, srcName, url, httpUrl string, config *message.NetbootConfig) error {

	log.Printf("expandIpxeFile(%s, %s, %s, <config>)\n", dstPathname, srcName, url)

	src, err := template.Ipxe.Open(path.Join("ipxe", srcName))
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
	// FIXME: source address filtering goes here
	log.Printf("[%s] %s -> HTTP %s %s\n", c.Name, r.RemoteAddr, r.Method, r.URL.Path)
	return true
}

func (c *HostCache) requireClientCert(w http.ResponseWriter, r *http.Request) bool {
	cert := c.checkClientCert(w, r)
	clientCN := "none"
	if cert != nil {
		clientCN = cert.Subject.String()
	}
	log.Printf("[%s] %s CN=%s -> HTTPS %s %s\n", c.Name, r.RemoteAddr, clientCN, r.Method, r.URL.Path)
	if cert == nil {
		Warning("Missing client certificate: %s %s %s", r.RemoteAddr, r.Method, r.URL)
		c.fail(w, "authorization failed", http.StatusForbidden)
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
	log.Printf("[%s] %s %s -> HTTPS %s %s\n", c.Name, r.RemoteAddr, clientCN, r.Method, r.URL.Path)
	return cert
}

func (c *HostCache) checkClientCert(w http.ResponseWriter, r *http.Request) *x509.Certificate {
	// FIXME: source address filtering goes here
	switch {
	case r.TLS == nil:
	case r.TLS.PeerCertificates == nil || len(r.TLS.PeerCertificates) < 1:
	default:
		return r.TLS.PeerCertificates[0]
	}
	return nil
}

func (c *HostCache) fail(w http.ResponseWriter, message string, status int) {

	log.Printf("[%s] <- fail [%d] %s\n", c.Name, status, message)
	http.Error(w, message, status)
}

func (c *HostCache) respond(w http.ResponseWriter, label string, response any) {
	log.Printf("[%s] <- response [200] %s\n", c.Name, label)
	json.NewEncoder(w).Encode(response)
}

func (c *HostCache) RootHandler(w http.ResponseWriter, r *http.Request) {
	return
}

func (c *HostCache) RootHandlerTLS(w http.ResponseWriter, r *http.Request) {
	return
}

func (c *HostCache) rootHandler(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/netboot.png":
		http.ServeFile(w, r, filepath.Join(c.ipxeDir, "netboot.png"))
		return
	}
	c.fail(w, "forbidden", http.StatusForbidden)
}

func (c *HostCache) VersionHandlerTLS(w http.ResponseWriter, r *http.Request) {
	c.optionalClientCert(w, r)
	_, err := w.Write([]byte(htmlPrefix + fmt.Sprintf("netboot server v%s", Version) + htmlSuffix))
	if err != nil {
		Warning("%v", Fatal(err))
		c.fail(w, "failed", http.StatusInternalServerError)
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
	log.Printf("[%s] <- response [200] epochTime=%d\n", c.Name, epochTime)
}

func (c *HostCache) GDLHandlerTLS(w http.ResponseWriter, r *http.Request) {
	c.optionalClientCert(w, r)
	version := r.PathValue("version")
	arch := r.PathValue("arch")
	file := r.PathValue("file")
	if file != "gdl.tgz" {
		Warning("invalid gdl filename: %s", r.URL.Path)
		c.fail(w, "invalid path", http.StatusBadRequest)
		return
	}
	parts := regexp.MustCompile(`^([0-9]+)\.([0-9]+)$`).FindStringSubmatch(version)
	if len(parts) != 3 {
		Warning("invalid gdl version: %s", r.URL.Path)
		c.fail(w, "invalid path", http.StatusBadRequest)
		return
	}
	major := parts[1]
	minor := parts[2]
	gdlPathname := filepath.Join("dist", "openbsd", version, arch, fmt.Sprintf("gdl%s%s.tgz", major, minor))
	http.ServeFileFS(w, r, template.Dist, gdlPathname)
}

func (c *HostCache) checkPath(prefix string, w http.ResponseWriter, r *http.Request) (string, string, bool) {
	dir, name := path.Split(r.URL.Path)
	if dir != prefix {
		c.fail(w, "invalid path", http.StatusBadRequest)
		return "", "", false

	}
	mac, ext, ok := strings.Cut(name, ".")
	if !ok {
		Warning("filename missing exension: %s", r.URL.Path)
		c.fail(w, "invalid path", http.StatusBadRequest)
		return "", "", false
	}
	if !MAC_PATTERN.MatchString(mac) {
		Warning("filename not MAC address: %s", r.URL.Path)
		c.fail(w, "invalid path", http.StatusBadRequest)
		return "", "", false
	}
	// FIXME: check if this is working with xx:xx:xx:xx:xx:xx.apkovl.tar.gz
	if APKOVL_PATTERN.MatchString(name) {
		ext = "apkovl.tar.gz"
	}
	return normalizeMAC(mac), strings.ToLower(ext), true
}

func (c *HostCache) SanBootHandler(w http.ResponseWriter, r *http.Request) {
	if !c.validateHttpRequest(w, r) {
		return
	}
	mac, ext, ok := c.checkPath("/san/", w, r)
	if !ok {
		return
	}
	switch ext {
	case "boot", "modloop", "apkovl.tar.gz":
		// IPXE sanboot command from the OpenBSD autoexec.ipxe
		// FIXME: verify that this request follows a very recent client-cert validated request for MAC.iso from the same source
		// it is possible that the ipxe client will be reusing the same session - see if we can set a cookie and check for it
		http.ServeFile(w, r, filepath.Join(c.cacheDir, "ipxe", mac+"."+ext))
		return
	}
	Warning("unexpected sanboot request: %s", r.URL.Path)
	c.fail(w, "invalid path", http.StatusBadRequest)
}

func (c *HostCache) PNGHandlerTLS(w http.ResponseWriter, r *http.Request) {
	if !c.requireClientCert(w, r) {
		return
	}
	http.ServeFileFS(w, r, template.Ipxe, "/ipxe/netboot.png")
}

func (c *HostCache) ISOHandlerTLS(w http.ResponseWriter, r *http.Request) {
	if !c.validateHttpRequest(w, r) {
		return
	}
	if r.URL.Path != "/iso/netboot.iso" {
		c.fail(w, "invalid path", http.StatusBadRequest)
		return

	}
	netbootIso := filepath.Join(c.ipxeDir, "netboot.iso")
	if !IsFile(netbootIso) {
		err := files.UnzipFileFromFS(netbootIso, filepath.Join("ipxe", "netboot.xyz.iso.gz"), template.Ipxe)
		if err != nil {
			c.fail(w, "invalid path", http.StatusInternalServerError)
			return
		}
	}
	http.ServeFile(w, r, netbootIso)
}

func (c *HostCache) IPXEHandlerTLS(w http.ResponseWriter, r *http.Request) {
	if !c.requireClientCert(w, r) {
		return
	}
	mac, ext, ok := c.checkPath("/ipxe/", w, r)
	if !ok {
		return
	}
	switch ext {
	case "iso", "img", "sh", "ipxe", "kernel", "initrd", "response", "tgz", "disk", "cacerts", "postinstall", "netboot":
		http.ServeFile(w, r, filepath.Join(c.cacheDir, "ipxe", mac+"."+ext))
		return
	}
	Warning("unexpected ipxe request: %s", r.URL.Path)
	c.fail(w, "invalid path", http.StatusBadRequest)
}

func (c *HostCache) proxyHandler(mirror string, w http.ResponseWriter, r *http.Request) {

	if !c.proxy {
		Warning("disabled proxy received request: %s", mirror)
		c.fail(w, "netboot proxy disabled", http.StatusNotImplemented)
		return
	}

	// FIXME: add mechanism for verifying cached assets are in sync with distribution mirrors

	if !IsDir(c.distDir) {
		err := os.MkdirAll(c.distDir, 0700)
		if err != nil {
			Warning("%v", Fatal(err))
			c.fail(w, "configuration error", http.StatusInternalServerError)
			return
		}
	}
	cacheRoot, err := os.OpenRoot(c.distDir)
	if err != nil {
		Warning("%v", Fatal(err))
		c.fail(w, "configuration error", http.StatusInternalServerError)
		return
	}

	mirrorUrl := c.mirrorUrl[mirror]
	if mirrorUrl == "" {
		Warning("No mirror configured: %s", mirror)
		c.fail(w, "no mirror configured", http.StatusNotImplemented)
		return
	}
	pathname, data, err := DownloadFileRoot(cacheRoot, mirrorUrl, r.URL.Path)
	if err != nil {
		Warning("Proxy '%s' failed: %v", r.URL.Path, err)
		c.fail(w, "proxy failure", http.StatusBadGateway)
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
			c.fail(w, "failed writing response", http.StatusInternalServerError)
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
		c.fail(w, fmt.Sprintf("failed parsing form: %v", err), http.StatusBadRequest)
		return
	}

	uploadFile, fileHeader, err := r.FormFile("uploadFile")
	if err != nil {
		c.fail(w, fmt.Sprintf("failed retreiving upload file: %v", err), http.StatusBadRequest)
		return
	}
	defer uploadFile.Close()

	packageFilename := fileHeader.Filename

	if !TARBALL_PATTERN.MatchString(packageFilename) {
		c.fail(w, fmt.Sprintf("illegal filename: %s", packageFilename), http.StatusBadRequest)
		return
	}

	packageFile, err := os.Create(filepath.Join(c.ipxeDir, packageFilename))
	if err != nil {
		c.fail(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer packageFile.Close()
	fileBytes, err := io.Copy(packageFile, uploadFile)
	if err != nil {
		c.fail(w, err.Error(), http.StatusBadRequest)
		return
	}

	c.respond(w, "UploadResponse", Response{Message: fmt.Sprintf("%v bytes written", fileBytes)})
}

func (c *HostCache) mkURLs(r *http.Request) (string, string) {
	log.Printf("mkURLs: r.Host=%s r.URL.Hostname=%s r.URL.Port=%s r.URL=%s\n", r.Host, r.URL.Hostname(), r.URL.Port(), r.URL)
	log.Printf("mkURLs: c.httpURL=%s c.httpsURL=%s\n", c.httpURL, c.httpsURL)
	httpsURL := c.httpsURL
	if httpsURL == "" {
		httpsURL = "https://" + r.Host
	}
	httpURL := c.httpURL
	if httpURL == "" {
		httpURL = "http://" + r.URL.Hostname()
		host, _, ok := strings.Cut(r.Host, ":")
		if ok {
			httpURL = fmt.Sprintf("http://%s:%d", host, c.httpPort)
		}
	}
	log.Printf("mkURLs: returning httpsURL=%s httpURL=%s\n", httpsURL, httpURL)
	return httpsURL, httpURL
}

func (c *HostCache) AddHostHandlerTLS(w http.ResponseWriter, r *http.Request) {
	if !c.requireClientCert(w, r) {
		return
	}
	var config message.NetbootConfig
	err := json.NewDecoder(r.Body).Decode(&config)
	if err != nil {
		c.fail(w, err.Error(), http.StatusBadRequest)
		return
	}

	if !MAC_PATTERN.MatchString(config.Address) {
		c.fail(w, "invalid MAC address", http.StatusBadRequest)
		return
	}
	config.Address = normalizeMAC(config.Address)

	log.Printf("adding host %s %s %s\n", config.Address, config.OS, config.Version)

	log.Printf("AddHostHandler setting MAC=%s IP=''\n", config.Address)
	c.cache[config.Address] = ""

	distNames, err := template.DistNames()
	if err != nil {
		Warning("%v", Fatal(err))
		c.fail(w, "failed DistNames lookup", http.StatusInternalServerError)
		return
	}

	if !slices.Contains(distNames, config.OS) {
		message := fmt.Sprintf("unrecognized OS: %s", config.OS)
		Warning("%s", message)
		c.fail(w, message, http.StatusBadRequest)
		return
	}

	distVersions, err := template.DistVersions(config.OS)
	if err != nil {
		Warning("%v", Fatal(err))
		c.fail(w, "failed verifying OS version", http.StatusInternalServerError)
		return
	}

	if !slices.Contains(distVersions, config.Version) {
		message := fmt.Sprintf("unrecognized OS Version: %s %s", config.OS, config.Version)
		Warning("%s", message)
		c.fail(w, message, http.StatusBadRequest)
		return
	}

	netbootURL, netbootHttpURL := c.mkURLs(r)

	// create temp directory for files used to generate EFI image and ISO
	tempDir, err := os.MkdirTemp("", "netboot-*")
	if err != nil {
		Warning("%v", Fatal(err))
		c.fail(w, "failed generating netboot files", http.StatusInternalServerError)
		return
	}

	// FIXME
	///defer os.RemoveAll(tempDir)
	log.Printf("NOT REMOVING netboot tempDir: %s\n", tempDir)

	// IPXE menu: ipxe/MAC.ipxe
	autoexec := filepath.Join(c.ipxeDir, fmt.Sprintf("%s.ipxe", config.Address))
	err = c.expandIpxeFile(autoexec, config.OS+"-autoexec.ipxe", netbootURL, netbootHttpURL, &config)
	if err != nil {
		Warning("%v", Fatal(err))
		c.fail(w, "failed generating ipxe menu", http.StatusInternalServerError)
		return
	}

	// installer response file: /ipxe/MAC.response
	responsePathname := filepath.Join(c.ipxeDir, fmt.Sprintf("%s.response", config.Address))
	decodedBytes, err := base64.StdEncoding.DecodeString(config.Response)
	if err != nil {
		Warning("%v", Fatal(err))
		c.fail(w, "failed decoding response file", http.StatusInternalServerError)
		return
	}
	log.Printf("addHostHandler: response=%s\n", responsePathname)
	err = os.WriteFile(responsePathname, decodedBytes, 0660)
	if err != nil {
		Warning("%v", Fatal(err))
		c.fail(w, "failed writing response file", http.StatusInternalServerError)
		return
	}

	// disk partitioning template: /ipxe/MAC.disk
	disklabelTemplatePathname := filepath.Join(c.ipxeDir, fmt.Sprintf("%s.disk", config.Address))
	log.Printf("addHostHandler: disklabelTemplate=%s\n", disklabelTemplatePathname)
	if config.DisklabelTemplate != "" {
		decodedBytes, err := base64.StdEncoding.DecodeString(config.DisklabelTemplate)
		if err != nil {
			Warning("%v", Fatal(err))
			c.fail(w, "failed decoding partition template", http.StatusInternalServerError)
			return
		}
		formattedBytes, err := formatPartitionTemplate(decodedBytes)
		if err != nil {
			Warning("%v", Fatal(err))
			c.fail(w, "failed formatting partition template", http.StatusInternalServerError)
			return
		}
		err = os.WriteFile(disklabelTemplatePathname, formattedBytes, 0660)
		if err != nil {
			Warning("%v", Fatal(err))
			c.fail(w, "failed writing partition template", http.StatusInternalServerError)
			return
		}
	}

	// netboot: /ipxe/MAC.iso
	isoFile, bootFiles, err := c.GenerateISO(tempDir, netbootURL, netbootHttpURL, &config)
	if err != nil {
		Warning("%v", Fatal(err))
		c.fail(w, "failed generating boot ISO", http.StatusInternalServerError)
		return
	}

	c.respond(w, "AddResponse", AddResponse{Message: fmt.Sprintf("%s configured", config.Address), ISO: booturl(netbootURL, isoFile), Files: bootFiles})
}

func formatPartitionTemplate(src []byte) ([]byte, error) {
	buf := bytes.NewBuffer([]byte(strings.ReplaceAll(string(src), ":", "\n")))
	lines := []string{}
	scanner := bufio.NewScanner(buf)
	for scanner.Scan() {
		line := scanner.Text()
		line = strings.TrimSpace(line)
		if line != "" {
			lines = append(lines, line)
		}
	}
	err := scanner.Err()
	if err != nil {
		return []byte{}, Fatal(err)
	}
	return []byte(strings.Join(lines, "\n") + "\n"), nil
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
		c.fail(w, err.Error(), http.StatusBadRequest)
		return
	}

	if !MAC_PATTERN.MatchString(request.Address) {
		c.fail(w, "invalid MAC address", http.StatusBadRequest)
		return
	}
	macAddr := normalizeMAC(request.Address)
	log.Printf("DeleteHostHandler setting MAC=%s IP=''\n", macAddr)
	c.cache[request.Address] = ""
	c.deleteAddressFiles(macAddr, w)
}

func (c *HostCache) deleteAddressFiles(inAddress string, w http.ResponseWriter) {
	log.Printf("deleteAddressFiles: %s\n", inAddress)
	if c.noDeleteIpxe {
		return
	}
	addresses, err := c.hostAddresses()
	if err != nil {
		c.fail(w, err.Error(), http.StatusBadRequest)
		return
	}
	for _, address := range addresses {
		if strings.ToLower(inAddress) == strings.ToLower(address) {
			files, err := c.deleteHostFiles(address)
			if err != nil {
				c.fail(w, err.Error(), http.StatusBadRequest)
				return
			}
			c.respond(w, "DeleteResponse", DeleteResponse{Message: fmt.Sprintf("deleted: %d", len(files)), Files: files})
			return
		}
	}
	c.fail(w, "host address not found", http.StatusNotFound)
	return
}

func (c *HostCache) HostBootedHandlerTLS(w http.ResponseWriter, r *http.Request) {
	if !c.requireClientCert(w, r) {
		return
	}
	address := normalizeMAC(r.PathValue("mac"))
	ip := r.PathValue("ip")
	switch {
	case address == "":
	case ip == "":
	default:
		log.Printf("HostBootedHandler setting MAC=%s IP=%s\n", address, ip)
		c.cache[address] = ip
		c.deleteAddressFiles(address, w)
		c.respond(w, "BootReportResponse", Response{Message: "boot acknowleded"})
		return
	}
	c.fail(w, "invalid path", http.StatusBadRequest)
}

func (c *HostCache) HostAddressQueryHandlerTLS(w http.ResponseWriter, r *http.Request) {
	if !c.requireClientCert(w, r) {
		return
	}
	segments := strings.Split(r.URL.Path, "/")
	if len(segments) > 3 {
		address := segments[3]
		log.Printf("HostAddressQueryHandler returning MAC=%s IP=%s\n", address, c.cache[address])
		c.respond(w, "HostAddressResponse", HostAddressResponse{MAC: address, IP: c.cache[address]})
		return
	}
	c.fail(w, "invalid path", http.StatusBadRequest)
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
		c.fail(w, err.Error(), http.StatusBadRequest)
		return
	}

	c.respond(w, "HostListResponse", HostListResponse{Message: fmt.Sprintf("config count: %d", len(addresses)), Addresses: addresses})
}

// Generate a url-customized netboot ISO returning generated iso pathname
func (c *HostCache) GenerateISO(tempDir, url, httpUrl string, config *message.NetbootConfig) (string, []string, error) {

	log.Printf("Generating netboot ISO for %s with URL %s\n", config.Address, url)

	bootFiles := []string{}

	tarball := filepath.Join(c.ipxeDir, config.Address+".tgz")

	// add autoexec.ipxe to bootFiles
	autoexec := filepath.Join(tempDir, "autoexec.ipxe")
	err := files.CopyFile(autoexec, filepath.Join(c.ipxeDir, config.Address+".ipxe"))
	if err != nil {
		return "", []string{}, Fatal(err)
	}
	bootFiles = append(bootFiles, autoexec)

	// add root CA from tarball to bootFiles
	clientCA := filepath.Join(tempDir, "keymaster.pem")
	err = files.ExtractTarballFile(clientCA, "etc/ssl/keymaster.pem", tarball)
	if err != nil {
		return "", []string{}, Fatal(err)
	}
	bootFiles = append(bootFiles, clientCA)

	// add client cert from tarball to bootFiles
	clientCert := filepath.Join(tempDir, "netboot.pem")
	err = files.ExtractTarballFile(clientCert, "etc/ssl/netboot.pem", tarball)
	if err != nil {
		return "", []string{}, Fatal(err)
	}
	bootFiles = append(bootFiles, clientCert)

	// add client cert key from tarball to bootFiles
	clientKey := filepath.Join(tempDir, "netboot.key")
	err = files.ExtractTarballFile(clientKey, "etc/ssl/netboot.key", tarball)
	if err != nil {
		return "", []string{}, Fatal(err)
	}
	bootFiles = append(bootFiles, clientKey)

	// extract tarball netboot_exec to temp dir for iso
	netbootExec := filepath.Join(tempDir, "netboot_exec")
	err = files.ExtractTarballFile(netbootExec, "root/netboot_exec", tarball)
	if err != nil {
		return "", []string{}, Fatal(err)
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
	netbootEnv += fmt.Sprintf("_gdl_url='%s/gdl/%s/%s/gdl.tgz'\n", url, config.Version, config.Arch)
	err = os.WriteFile(netbootEnvFile, []byte(netbootEnv), 0644)
	if err != nil {
		return "", []string{}, Fatal(err)
	}
	bootFiles = append(bootFiles, netbootEnvFile)

	// extract tarball postinstall to temp dir for debian mkboot
	// NOTE: intentionally not added to bootFiles
	postinstall := filepath.Join(tempDir, "postinstall")
	err = files.ExtractTarballFile(postinstall, "postinstall", tarball)
	if err != nil {
		return "", []string{}, Fatal(err)
	}

	mkboot := NewMkBoot(tempDir, c.ipxeDir, url, &bootFiles, config)
	isoFile, err := mkboot.Generate()
	if err != nil {
		return "", []string{}, Fatal(err)
	}
	log.Printf("Generated %s\n", isoFile)
	return isoFile, bootFiles, nil

}

func normalizeMAC(mac string) string {
	return strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(mac, ":", ""), "-", ""))
}

func (c *HostCache) ShutdownHandlerTLS(w http.ResponseWriter, r *http.Request) {
	if !c.requireClientCert(w, r) {
		return
	}
	c.respond(w, "ShutdownRequestReponse", Response{Message: "shutdown request acknowleged"})
	InternalShutdownRequest <- struct{}{}
}

func (c *HostCache) AddWhitelistAddressHandlerTLS(w http.ResponseWriter, r *http.Request) {
	if !c.requireClientCert(w, r) {
		return
	}
	if c.whitelistCommand == "" {
		c.respond(w, "WhitelistAddressResponse", Response{Message: "not configured"})
		return
	}
	ip := r.PathValue("ip")
	cmd := exec.Command("sh", "-c", c.whitelistCommand+" "+"add "+ip)
	data, err := cmd.Output()
	if err != nil {
		log.Printf("whitelist command: %v\n", cmd)
		log.Printf("failed: %v\n", err)
		c.fail(w, "failed (see netboot log for detail)", http.StatusInternalServerError)
		return
	}
	log.Printf("Whitelist add: %s\n%s\n", ip, string(data))
	c.respond(w, "WhitelistAddressResponse", Response{Message: "whitelisted: " + ip})
}

func (c *HostCache) DeleteWhitelistAddressHandlerTLS(w http.ResponseWriter, r *http.Request) {
	if !c.requireClientCert(w, r) {
		return
	}
	if c.whitelistCommand == "" {
		c.respond(w, "WhitelistAddressResponse", Response{Message: "not configured"})
		return
	}
	ip := r.PathValue("ip")
	cmd := exec.Command("sh", "-c", c.whitelistCommand+" "+"delete "+ip)
	data, err := cmd.Output()
	if err != nil {
		log.Printf("whitelist command: %v\n", cmd)
		log.Printf("failed: %v\n", err)
		c.fail(w, "failed (see netboot log for detail)", http.StatusInternalServerError)
		return
	}
	log.Printf("Whitelist delete: %s\n%s\n", ip, string(data))
	c.respond(w, "WhitelistAddressResponse", Response{Message: "deleted: " + ip})
}
