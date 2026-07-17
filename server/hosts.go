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
const DEFAULT_OPENBSD_MIRROR = "https://ftp.openbsd.org"
const DEFAULT_ALPINE_MIRROR = "https://dl-cdn.alpinelinux.org"
const DEFAULT_ISO_KEY_LIFETIME = 600
const DEFAULT_WHITELIST_LIFETIME = 3600
const DEFAULT_MAX_DIST_UPLOAD_MB = 1024

const BOOTSTRAP_MAC_ADDRESS = "000000000000"

type Host struct {
	Address string `json:"address"`
}

var MAC_PATTERN = regexp.MustCompile(`^([0-9A-Fa-f]{2}[:-]{0,1}){5}([0-9A-Fa-f]{2})$`)
var NORMALIZED_MAC_PATTERN = regexp.MustCompile(`^[0-9a-f]{12}$`)
var IPXE_PATTERN = regexp.MustCompile(`^([0-9A-Fa-f]{2}[:-]{0,1}){5}([0-9A-Fa-f]{2})\.ipxe$`)
var TARBALL_PATTERN = regexp.MustCompile(`^([0-9a-f]{12})\.tgz$`)
var ALPINE_VERSION_PATTERN = regexp.MustCompile(`([0-9][0-9]*)\.([0-9][0-9]*)\.([0-9][0-9]*)$`)
var APKOVL_PATTERN = regexp.MustCompile(`^([0-9A-Fa-f]{2}[:-]{0,1}){5}([0-9A-Fa-f]{2})\.apkovl\.tar\.gz$`)

var MAC_ISO_PATTERN = regexp.MustCompile(`^([0-9A-Fa-f]{2}[:-]{0,1}){5}([0-9A-Fa-f]{2})\.iso$`)

var DIST_UPLOAD_PATTERNS []*regexp.Regexp = []*regexp.Regexp{
	regexp.MustCompile(`^pub/alpine/\d\.\d\.\d/x86_64/[[:word:].-]+`),
	regexp.MustCompile(`^pub/debian/[a-z]+/amd64/[[:word:].-]+`),
	regexp.MustCompile(`^pub/OpenBSD/\d\.\d/packages/amd64/[[:word:].-]+`),
	regexp.MustCompile(`^pub/windows/\d+/x64/[[:word:].-]+`),
	regexp.MustCompile(`^pub/OpenBSD/\d\.\d/bin/amd64/[[:word:].-]+`),
}

var DIST_DELETE_PATTERNS []*regexp.Regexp = []*regexp.Regexp{
	regexp.MustCompile(`^pub/alpine(/.*)*`),
	regexp.MustCompile(`^pub/debian(/.*)*`),
	regexp.MustCompile(`^pub/OpenBSD(/.*)*`),
	regexp.MustCompile(`^pub/windows(/.*)*`),
}

var DIST_TREE_PATTERNS []*regexp.Regexp = []*regexp.Regexp{
	regexp.MustCompile(`.*`),
}

var GDL_VERSION_PATTERN = regexp.MustCompile(`^pub/.*/rstms-gdl-(\d+)\.(\d+)\.(\d+).*\.tgz$`)

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
	Name              string
	cacheDir          string
	ipxeDir           string
	distDir           string
	uploadDir         string
	cache             map[string]message.HostState
	bootstrapCache    map[string]message.HostState
	httpPort          int
	httpsPort         int
	proxy             bool
	noDeleteIpxe      bool
	mirrorUrl         map[string]string
	httpURL           string
	httpsURL          string
	whitelistCommand  string
	isoKeys           map[string]string
	isoKeyLifetime    int
	isoMode           map[string]string
	whitelistLifetime int
	maxDistUploadMB   int64
}

func NewHostCache(dir string, httpPort, httpsPort int, proxyEnabled bool) (*HostCache, error) {

	prefix := "netboot.server."
	if ProgramName() == "netboot" {
		prefix = "server."
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, Fatal(err)
	}

	ViperSetDefault(prefix+"mirror.alpine", DEFAULT_ALPINE_MIRROR)
	ViperSetDefault(prefix+"mirror.debian", DEFAULT_DEBIAN_MIRROR)
	ViperSetDefault(prefix+"mirror.debian-security", DEFAULT_DEBIAN_SECURITY_MIRROR)
	ViperSetDefault(prefix+"mirror.openbsd", DEFAULT_OPENBSD_MIRROR)
	ViperSetDefault(prefix+"iso_key_lifetime", DEFAULT_ISO_KEY_LIFETIME)
	ViperSetDefault(prefix+"whitelist_lifetime", DEFAULT_WHITELIST_LIFETIME)
	ViperSetDefault(prefix+"dist_upload_dir", filepath.Join(homeDir, "dist"))
	ViperSetDefault(prefix+"max_dist_upload_mb", int64(DEFAULT_MAX_DIST_UPLOAD_MB))

	c := HostCache{
		Name:           "netboot",
		cacheDir:       dir,
		httpPort:       httpPort,
		httpsPort:      httpsPort,
		proxy:          proxyEnabled,
		cache:          make(map[string]message.HostState),
		bootstrapCache: make(map[string]message.HostState),
		ipxeDir:        filepath.Join(dir, "ipxe"),
		distDir:        filepath.Join(dir, "dist"),
		uploadDir:      filepath.Join(dir, "upload"),
		noDeleteIpxe:   ViperGetBool(prefix + "no_delete_ipxe"),
		mirrorUrl: map[string]string{
			"alpine":          ViperGetString(prefix + "mirror.alpine"),
			"debian":          ViperGetString(prefix + "mirror.debian"),
			"debian-security": ViperGetString(prefix + "mirror.debian-security"),
			"openbsd":         ViperGetString(prefix + "mirror.openbsd"),
		},
		httpURL:           ViperGetString(prefix + "http_url"),
		httpsURL:          ViperGetString(prefix + "https_url"),
		whitelistCommand:  ViperGetString(prefix + "whitelist_command"),
		whitelistLifetime: ViperGetInt(prefix + "whitelist_lifetime"),
		isoKeys:           make(map[string]string),
		isoMode:           make(map[string]string),
		isoKeyLifetime:    ViperGetInt(prefix + "iso_key_lifetime"),
		maxDistUploadMB:   ViperGetInt64(prefix + "max_dist_upload_mb"),
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
	if !IsDir(c.uploadDir) {
		err := os.MkdirAll(c.uploadDir, 0700)
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
	log.Printf("dist dir: %s\n", c.distDir)
	log.Printf("upload dir: %s\n", c.uploadDir)
	log.Printf("whitelist lifetime: %d\n", c.whitelistLifetime)

	return &c, nil
}

// expand macros in file from Ipxe template writing to dstPath
func (c *HostCache) expandIpxeFile(dstPathname, srcName, url, httpUrl string, config *message.NetbootConfig, version, arch, mirror string) error {

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
			_, err = fmt.Fprintf(dst, "set mirror %s\n", mirror)
		case strings.HasPrefix(line, "set branch"):
			match := ALPINE_VERSION_PATTERN.FindStringSubmatch(version)
			if len(match) != 4 {
				return Fatalf("unexpected alpine version format: %s\n", version)
			}
			branch := fmt.Sprintf("v%s.%s", match[1], match[2])
			_, err = fmt.Fprintf(dst, "set branch %s\n", branch)
		case strings.HasPrefix(line, "set version"):
			_, err = fmt.Fprintf(dst, "set version %s\n", version)
		case strings.HasPrefix(line, "set arch"):
			_, err = fmt.Fprintf(dst, "set arch %s\n", arch)
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
	//log.Printf("headers: %s\n", FormatJSON(r.Header))
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

// FIXME: implement high-performance sanboot request handler
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
		http.ServeFile(w, r, filepath.Join(c.ipxeDir, mac+"."+ext))
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
	c.optionalClientCert(w, r)
	isoKey := r.PathValue("key")
	mac, ok := c.isoKeys[isoKey]
	if !ok {
		Warning("unauthorized iso request: %+v", r)
		c.fail(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// generate a filename from the isoKey mac address
	isoFile := mac + ".iso"
	if !MAC_ISO_PATTERN.MatchString(isoFile) {
		Warning("unexpected isoKey generated file: %s", isoFile)
		c.fail(w, "invalid path", http.StatusBadRequest)
		return
	}

	// verify the file requested matches the isoKey lookup value
	_, requestFile := path.Split(r.URL.Path)
	if isoFile != requestFile {
		Warning("unexpected iso request file: %s", requestFile)
		c.fail(w, "path mismatch", http.StatusBadRequest)
		return
	}

	var responseFile string
	mode, ok := c.isoMode[isoKey]
	if !ok {
		mode = "iso"
	}
	switch mode {
	case "iso":
		responseFile = filepath.Join(c.ipxeDir, mac+".iso")
	case "img":
		responseFile = filepath.Join(c.ipxeDir, mac+".img")
	case "boot":
		responseFile = filepath.Join(c.ipxeDir, mac+".boot")
	default:
		Warning("unexpected iso mode: %s", mode)
		c.fail(w, "not found", http.StatusBadRequest)
		return
	}

	log.Printf("Vultr ISO request: key=%s file=%s\n", isoKey, responseFile)

	if !IsFile(responseFile) {
		Warning("iso request file not found: %s", responseFile)
		c.fail(w, "not found", http.StatusNotFound)
		return
	}
	http.ServeFile(w, r, responseFile)
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
		http.ServeFile(w, r, filepath.Join(c.ipxeDir, mac+"."+ext))
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

	// prioritize uploaded dist files
	if c.CheckUploadCache(mirror, w, r) {
		return
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

func (c *HostCache) CheckUploadCache(mirror string, w http.ResponseWriter, r *http.Request) bool {
	filePath := strings.ReplaceAll(r.URL.Path, "/", string(filepath.Separator))
	pathname := filepath.Join(c.uploadDir, filePath)
	if IsFile(pathname) {
		log.Printf("returning uploaded dist file: %s", pathname)
		http.ServeFile(w, r, pathname)
		return true
	}
	return false
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

	c.respond(w, "UploadResponse", message.NetbootResponse{Message: fmt.Sprintf("%v bytes written", fileBytes)})
}

func (c *HostCache) validateDistPathname(w http.ResponseWriter, r *http.Request, patterns []*regexp.Regexp) string {
	pathParts := strings.Split(r.URL.Path, "/")
	var uploadPathname string
	if len(pathParts) > 3 {
		uploadPathname = strings.Join(pathParts[3:], "/")
	}
	distPathname := filepath.Join(c.uploadDir, strings.ReplaceAll(uploadPathname, "/", string(filepath.Separator)))
	for _, pattern := range patterns {
		if pattern.MatchString(uploadPathname) {
			return distPathname
		}
	}
	c.fail(w, fmt.Sprintf("illegal filename: %s", uploadPathname), http.StatusBadRequest)
	return ""
}

func (c *HostCache) UploadDistHandlerTLS(w http.ResponseWriter, r *http.Request) {
	if !c.requireClientCert(w, r) {
		return
	}

	err := r.ParseMultipartForm(c.maxDistUploadMB << 20) // limit file size
	if err != nil {
		c.fail(w, fmt.Sprintf("failed parsing form: %v", err), http.StatusBadRequest)
		return
	}

	uploadFile, _, err := r.FormFile("uploadFile")
	if err != nil {
		c.fail(w, fmt.Sprintf("failed retreiving upload file: %v", err), http.StatusBadRequest)
		return
	}
	defer uploadFile.Close()

	distPathname := c.validateDistPathname(w, r, DIST_UPLOAD_PATTERNS)
	if distPathname == "" {
		c.fail(w, "invalid upload path", http.StatusBadRequest)
		return
	}

	dir, _ := filepath.Split(distPathname)
	if !IsDir(dir) {
		err := os.MkdirAll(dir, 0700)
		if err != nil {
			Warning("%v", Fatal(err))
			c.fail(w, "upload failed", http.StatusInternalServerError)
			return
		}
	}

	distFile, err := os.Create(distPathname)
	if err != nil {
		c.fail(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer distFile.Close()
	fileBytes, err := io.Copy(distFile, uploadFile)
	if err != nil {
		c.fail(w, err.Error(), http.StatusBadRequest)
		return
	}
	c.respond(w, "DistUploadResponse", message.NetbootResponse{Message: fmt.Sprintf("%v bytes written", fileBytes)})
}

func (c *HostCache) DeleteDistHandlerTLS(w http.ResponseWriter, r *http.Request) {
	if !c.requireClientCert(w, r) {
		return
	}
	pathname := c.validateDistPathname(w, r, DIST_DELETE_PATTERNS)
	if pathname == "" {
		return
	}
	var msg string
	switch {
	case IsFile(pathname):
		err := os.Remove(pathname)
		if err != nil {
			Warning("%v", Fatal(err))
			c.fail(w, "file delete failed", http.StatusInternalServerError)
			return
		}
		msg = "file deleted"
	case IsDir(pathname):
		err := os.RemoveAll(pathname)
		if err != nil {
			Warning("%v", Fatal(err))
			c.fail(w, "directory delete failed", http.StatusInternalServerError)
			return
		}
		msg = "directory deleted"
	default:
		c.fail(w, "not found", http.StatusNotFound)
		return
	}
	c.respond(w, "DistDeleteResponse", message.NetbootResponse{Message: msg})
}

func (c *HostCache) GetDistHandlerTLS(w http.ResponseWriter, r *http.Request) {
	if !c.requireClientCert(w, r) {
		return
	}
	pathname := c.validateDistPathname(w, r, DIST_TREE_PATTERNS)
	if pathname == "" {
		return
	}
	files, err := files.TreeFiles(c.uploadDir, pathname)
	if err != nil {
		Warning("%v", Fatal(err))
		c.fail(w, "dist directory failed", http.StatusInternalServerError)
		return
	}
	c.respond(w, "DistFilesResponse", message.NetbootDistFilesResponse{
		Message: pathname,
		Files:   files,
	})
}

func (c *HostCache) mkURLs(r *http.Request) (string, string) {
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

	if config.Address == BOOTSTRAP_MAC_ADDRESS {
		version, arch, mirror, err := DefaultDist("alpine")
		if err != nil {
			Warning("%v", Fatal(err))
			c.fail(w, "failed generating bootstrap iso", http.StatusInternalServerError)
			return
		}
		config = message.NetbootConfig{
			Address:     BOOTSTRAP_MAC_ADDRESS,
			OS:          "alpine",
			Version:     version,
			Arch:        arch,
			Mirror:      mirror,
			BootstrapId: config.BootstrapId,
		}
		c.bootstrapCache[config.BootstrapId] = message.HostState{BootstrapId: config.BootstrapId, State: "init"}
	}

	if !MAC_PATTERN.MatchString(config.Address) {
		c.fail(w, "invalid MAC address", http.StatusBadRequest)
		return
	}
	config.Address = normalizeMAC(config.Address)

	log.Printf("AddHostHandler: adding MAC=%s IP='' %s %s\n", config.Address, config.OS, config.Version)
	c.cache[config.Address] = message.HostState{MAC: config.Address, State: "init"}

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

	defer os.RemoveAll(tempDir)

	ipxeAutoexec := filepath.Join(c.ipxeDir, fmt.Sprintf("%s.ipxe", config.Address))

	if config.AlpineLoader != "" {
		// if AlpineLoader active, write ipxe/MAC.ipxe
		// from alpine-autoexec.ipxe for alpine image load
		version, arch, mirror, err := DefaultDist("alpine")
		if err != nil {
			Warning("%v", Fatal(err))
			c.fail(w, "failed selecting image loader dist", http.StatusInternalServerError)
			return
		}
		err = c.expandIpxeFile(ipxeAutoexec, "alpine-autoexec.ipxe", netbootURL, netbootHttpURL, &config, version, arch, mirror)
		if err != nil {
			Warning("%v", Fatal(err))
			c.fail(w, "failed generating image loader ipxe", http.StatusInternalServerError)
			return
		}

	} else {
		// AlpineLoader is not active, so write MAC.ipxe for selected OS
		autoexecName := config.OS
		if config.Address == BOOTSTRAP_MAC_ADDRESS {
			autoexecName = "bootstrap"
		}
		err = c.expandIpxeFile(ipxeAutoexec, autoexecName+"-autoexec.ipxe", netbootURL, netbootHttpURL, &config, config.Version, config.Arch, config.Mirror)
		if err != nil {
			Warning("%v", Fatal(err))
			c.fail(w, "failed generating ipxe", http.StatusInternalServerError)
			return
		}
	}

	// autoexec.ipxe.iso follows MAC.ipxe (alpine-autoexec.ipxe if alpine image load)
	err = files.CopyFile(filepath.Join(tempDir, "autoexec.ipxe.iso"), ipxeAutoexec)
	if err != nil {
		c.fail(w, "failed copying autoexec.ipxe.iso", http.StatusInternalServerError)
		return
	}

	// autoexec.ipxe.img is the ipxe for the selected OS regardless of alpine image load selection
	err = c.expandIpxeFile(filepath.Join(tempDir, "autoexec.ipxe.img"), config.OS+"-autoexec.ipxe", netbootURL, netbootHttpURL, &config, config.Version, config.Arch, config.Mirror)
	if err != nil {
		c.fail(w, "failed copying autoexec.ipxe.img", http.StatusInternalServerError)
		return
	}

	if config.Response != "" {
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

	checksum, err := files.CalculateSHA512(isoFile)
	if err != nil {
		Warning("%v", Fatal(err))
		c.fail(w, "failed calculating ISO checksum", http.StatusInternalServerError)
		return
	}

	c.respond(w, "AddResponse", message.NetbootAddHostResponse{
		Message:   fmt.Sprintf("%s configured", config.Address),
		ISO:       booturl(netbootURL, isoFile),
		IsoSHA512: checksum,
		Files:     bootFiles},
	)
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
	delete(c.cache, request.Address)
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
			c.respond(w, "DeleteResponse", message.NetbootDeleteHostResponse{Message: fmt.Sprintf("deleted: %d", len(files)), Files: files})
			return
		}
	}
	c.fail(w, "host address not found", http.StatusNotFound)
	return
}

func (c *HostCache) PutBootstrapHandlerTLS(w http.ResponseWriter, r *http.Request) {
	if !c.requireClientCert(w, r) {
		return
	}
	bootstrapId := r.PathValue("id")
	address := normalizeMAC(r.PathValue("mac"))
	ip := r.PathValue("ip")
	log.Printf("Received bootstrap report: ID=%s MAC=%s IP=%s\n", bootstrapId, address, ip)
	c.bootstrapCache[bootstrapId] = message.HostState{
		MAC:         address,
		IP:          ip,
		State:       "bootstrapped",
		BootstrapId: bootstrapId,
	}
	c.respond(w, "BootstrapResponse", message.NetbootResponse{Message: "bootstrap acknowledged"})
}

func (c *HostCache) GetHostSetStatusHandlerTLS(w http.ResponseWriter, r *http.Request) {
	c.optionalClientCert(w, r)
	c.updateHostStatus(w, r)
}

func (c *HostCache) PutHostStatusHandlerTLS(w http.ResponseWriter, r *http.Request) {
	if !c.requireClientCert(w, r) {
		return
	}
	c.updateHostStatus(w, r)
}

func (c *HostCache) updateHostStatus(w http.ResponseWriter, r *http.Request) {
	status := r.PathValue("status")
	address := normalizeMAC(r.PathValue("mac"))
	ip := r.PathValue("ip")

	if address == "" {
		Warning("netboot status report missing address: %s", address)
		c.fail(w, "invalid path", http.StatusBadRequest)
		return
	}

	log.Printf("Received netboot status: MAC=%s IP=%s Status=%s\n", address, ip, status)
	c.cache[address] = message.HostState{MAC: address, IP: ip, State: status}
	c.respond(w, "BootReportResponse", message.NetbootResponse{Message: "acknowledged status: " + status})
}

func (c *HostCache) HostAddressQueryHandlerTLS(w http.ResponseWriter, r *http.Request) {
	if !c.requireClientCert(w, r) {
		return
	}

	address := r.PathValue("mac")
	hostState, ok := c.bootstrapCache[address]
	if !ok {
		hostState, ok = c.cache[normalizeMAC(address)]
		if !ok {
			hostState = message.HostState{MAC: address}
		}
	}
	log.Printf("HostAddressQueryHandler returning: %s\n", FormatJSON(hostState))
	c.respond(w, "HostAddressResponse", hostState)
	return
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

	c.respond(w, "HostListResponse", message.NetbootListHostsResponse{Message: fmt.Sprintf("config count: %d", len(addresses)), Addresses: addresses})
}

// Generate a url-customized netboot ISO returning generated iso pathname
func (c *HostCache) GenerateISO(tempDir, url, httpUrl string, config *message.NetbootConfig) (string, []string, error) {

	log.Printf("Generating netboot ISO for %s with URL %s\n", config.Address, url)

	bootFiles := []string{}

	tarball := filepath.Join(c.ipxeDir, config.Address+".tgz")

	// add both autoexec.ipxe.iso and autoexec.ipxe.img to bootFiles
	// buildIMG and buildISO will select the appropriate one
	bootFiles = append(bootFiles, filepath.Join(tempDir, "autoexec.ipxe.iso"))
	bootFiles = append(bootFiles, filepath.Join(tempDir, "autoexec.ipxe.img"))

	// add root CA from tarball to bootFiles
	clientCA := filepath.Join(tempDir, "keymaster.pem")
	err := files.ExtractTarballFile(clientCA, "etc/ssl/keymaster.pem", tarball)
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
	env := make(map[string]string)
	env["_url"] = url
	env["_mac"] = config.Address
	env["_egress"] = config.EgressInterface
	env["_root"] = config.RootDevice
	env["_certs"] = "'--cacert /cdrom/keymaster.pem --cert /cdrom/netboot.pem --key /cdrom/netboot.key'"
	env["_image_load"] = config.AlpineLoader
	env["_bootstrap_id"] = config.BootstrapId
	if config.Debug {
		env["_debug"] = "1"
	} else {
		env["_debug"] = ""
	}
	if config.Quiet {
		env["_quiet"] = "1"
	} else {
		env["_quiet"] = ""
	}
	if config.Shutdown {
		env["_shutdown"] = "1"
	} else {
		env["_shutdown"] = ""
	}

	switch config.OS {
	case "openbsd":
		gdlUrl, err := c.gdlUrl(url, config.Version, config.Arch)
		if err != nil {
			return "", nil, Fatal(err)
		}
		env["_gdl_url"] = gdlUrl

	default:
		env["_gdl_url"] = fmt.Sprintf("%s/gdl/%s/%s/gdl.tgz", url, config.Version, config.Arch)
	}
	env["GDL_CA"] = "/etc/ssl/keymaster.pem"
	env["GDL_CERT"] = "/etc/ssl/netboot.pem"
	env["GDL_KEY"] = "/etc/ssl/netboot.key"

	netbootEnv := ""
	envPrefix := ""
	//envPrefix := "export "

	for key, value := range env {
		netbootEnv += fmt.Sprintf("%s%s=%s\n", envPrefix, key, value)
	}
	err = os.WriteFile(netbootEnvFile, []byte(netbootEnv), 0644)
	if err != nil {
		return "", []string{}, Fatal(err)
	}
	bootFiles = append(bootFiles, netbootEnvFile)

	// extract tarball postinstall to ipxe/MAC.postinstall
	postinstall := filepath.Join(c.ipxeDir, fmt.Sprintf("%s.postinstall", config.Address))
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

func (c *HostCache) gdlUrl(url, version, arch string) (string, error) {
	gdlPath := filepath.Join(c.uploadDir, "pub", "OpenBSD", version, "packages", arch)
	log.Printf("gdlPath=%s\n", filepath.Join(c.uploadDir, gdlPath))
	files, err := files.TreeFiles(c.uploadDir, gdlPath)
	if err != nil {
		return "", Fatal(err)
	}
	gdlFiles := []string{}
	for _, file := range files {
		if GDL_VERSION_PATTERN.MatchString(file) {
			gdlFiles = append(gdlFiles, file)
		}
	}
	if len(gdlFiles) < 1 {
		return "", Fatalf("no gdl file found for OpenBSD %s %s", version, arch)
	}
	sortedFiles, err := SemverSort(gdlFiles, GDL_VERSION_PATTERN)
	if err != nil {
		return "", Fatal(err)
	}
	tarball := sortedFiles[len(sortedFiles)-1]
	return fmt.Sprintf("%s/%s", url, tarball), nil
}

func normalizeMAC(mac string) string {
	return strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(mac, ":", ""), "-", ""))
}

func (c *HostCache) ShutdownHandlerTLS(w http.ResponseWriter, r *http.Request) {
	if !c.requireClientCert(w, r) {
		return
	}
	c.respond(w, "ShutdownRequestReponse", message.NetbootResponse{Message: "shutdown request acknowledged"})
	InternalShutdownRequest <- struct{}{}
}

func (c *HostCache) AddWhitelistAddressHandlerTLS(w http.ResponseWriter, r *http.Request) {
	if !c.requireClientCert(w, r) {
		return
	}
	if c.whitelistCommand == "" {
		c.respond(w, "WhitelistAddressResponse", message.NetbootResponse{Message: "not configured"})
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
	go func() {
		timeout := time.NewTimer(time.Duration(uint64(c.whitelistLifetime)) * time.Second)
		<-timeout.C
		msg, _ := c.deleteWhitelist(ip)
		log.Printf("whitelist expired: %s\n", msg)
	}()
	c.respond(w, "WhitelistAddressResponse", message.NetbootResponse{Message: "whitelisted: " + ip})
}

func (c *HostCache) DeleteWhitelistAddressHandlerTLS(w http.ResponseWriter, r *http.Request) {
	if !c.requireClientCert(w, r) {
		return
	}
	if c.whitelistCommand == "" {
		c.respond(w, "WhitelistAddressResponse", message.NetbootResponse{Message: "not configured"})
		return
	}
	ip := r.PathValue("ip")
	msg, ok := c.deleteWhitelist(ip)
	if !ok {
		c.fail(w, msg, http.StatusInternalServerError)
		return
	}
	c.respond(w, "WhitelistAddressResponse", message.NetbootResponse{Message: msg})
}

func (c *HostCache) deleteWhitelist(ipaddr string) (string, bool) {
	cmd := exec.Command("sh", "-c", c.whitelistCommand+" "+"delete "+ipaddr)
	output, err := cmd.Output()
	log.Printf("Whitelist delete: %v\n", cmd)
	if err != nil {
		log.Printf("failed: %v\n", err)
		return "failed (see log for detail)", false
	}
	log.Printf("    output: '%s'\n", strings.TrimSpace(string(output)))
	return "deleted: " + ipaddr, true
}

func (c *HostCache) AddIsoKeyHandlerTLS(w http.ResponseWriter, r *http.Request) {
	if !c.requireClientCert(w, r) {
		return
	}
	key := r.PathValue("key")
	mac := normalizeMAC(r.PathValue("mac"))
	mode := r.PathValue("mode")
	c.isoKeys[key] = mac
	c.isoMode[key] = mode
	go func() {
		timeout := time.NewTimer(time.Duration(uint64(c.isoKeyLifetime)) * time.Second)
		<-timeout.C
		_, ok := c.deleteIsoKey(key)
		if ok {
			log.Printf("auto deleted ISOKey[%s]\n", key)
		}
	}()
	log.Printf("Added ISOkey[%s]=%s\n", key, mac)
	c.respond(w, "iso key added", message.NetbootResponse{Message: "key added"})
}

func (c *HostCache) DeleteIsoKeyHandlerTLS(w http.ResponseWriter, r *http.Request) {
	if !c.requireClientCert(w, r) {
		return
	}
	key := r.PathValue("key")
	msg, ok := c.deleteIsoKey(key)
	if !ok {
		c.fail(w, msg, http.StatusNotFound)
		return
	}
	c.respond(w, msg, message.NetbootResponse{Message: msg})
}

func (c *HostCache) deleteIsoKey(key string) (string, bool) {
	_, ok := c.isoKeys[key]
	if !ok {
		return "key not present", false
	}
	delete(c.isoKeys, key)
	delete(c.isoMode, key)
	return "key deleted", true
}
