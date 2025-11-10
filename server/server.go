package server

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"embed"
	"fmt"
	"github.com/rstms/systray/menu"
	"github.com/spf13/viper"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

const Version = "1.2.31"

const DEFAULT_SERVER_NAME = "localhost"
const DEFAULT_BIND_ADDRESS = "127.0.0.1"
const DEFAULT_HTTPS_PORT = "4443"
const DEFAULT_HTTP_PORT = "4444"
const DEFAULT_SHUTDOWN_TIMEOUT_SECONDS = 10

type Template struct {
	Certs  embed.FS
	Ipxe   embed.FS
	Dist   embed.FS
	Mkboot embed.FS
}

type NetbootServer struct {
	Name                   string
	ServerName             string
	Address                string
	HttpsPort              int
	HttpPort               int
	verbose                bool
	debug                  bool
	proxy                  bool
	hosts                  *HostCache
	wg                     sync.WaitGroup
	shutdown               chan struct{}
	NetbootDir             string
	caFile                 string
	certFile               string
	keyFile                string
	shutdownTimeoutSeconds int
	enableMenu             bool
}

var InternalShutdownRequest chan struct{}

func expandFilename(filename string) (string, error) {
	filename = os.ExpandEnv(filename)
	if strings.HasPrefix(filename, "~") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		filename = filepath.Join(homeDir, filename[1:])
	}
	return filepath.Clean(filename), nil
}

func getViperPrefix() string {
	prefix := "netboot.server."
	if ProgramName() == "netboot" {
		prefix = "server."
	}
	return prefix
}

func NewNetbootServer() (*NetbootServer, error) {
	userConfigDir, err := os.UserConfigDir()
	if err != nil {
		return nil, Fatal(err)
	}
	userCacheDir, err := os.UserCacheDir()
	if err != nil {
		return nil, err
	}
	viperPrefix := getViperPrefix()
	ViperSetDefault(viperPrefix+"server_name", DEFAULT_SERVER_NAME)
	ViperSetDefault(viperPrefix+"bind_address", DEFAULT_BIND_ADDRESS)
	ViperSetDefault(viperPrefix+"https_port", DEFAULT_HTTPS_PORT)
	ViperSetDefault(viperPrefix+"http_port", DEFAULT_HTTP_PORT)
	ViperSetDefault(viperPrefix+"shutdown_timeout_seconds", DEFAULT_SHUTDOWN_TIMEOUT_SECONDS)
	ViperSetDefault(viperPrefix+"ca", filepath.Join(userConfigDir, ProgramName(), "keymaster.pem"))
	ViperSetDefault(viperPrefix+"cert", filepath.Join(userConfigDir, ProgramName(), "netboot-server-cert.pem"))
	ViperSetDefault(viperPrefix+"key", filepath.Join(userConfigDir, ProgramName(), "netboot-server-key.pem"))
	ViperSetDefault(viperPrefix+"cache_dir", filepath.Join(userCacheDir, ProgramName()))

	s := NetbootServer{
		Name:                   "netboot",
		ServerName:             ViperGetString(viperPrefix + "server_name"),
		Address:                ViperGetString(viperPrefix + "bind_address"),
		HttpPort:               ViperGetInt(viperPrefix + "http_port"),
		HttpsPort:              ViperGetInt(viperPrefix + "https_port"),
		verbose:                ViperGetBool(viperPrefix + "verbose"),
		debug:                  ViperGetBool(viperPrefix + "debug"),
		shutdown:               make(chan struct{}, 1),
		NetbootDir:             ViperGetString(viperPrefix + "cache_dir"),
		shutdownTimeoutSeconds: ViperGetInt(viperPrefix + "shutdown_timeout_seconds"),
		proxy:                  ViperGetBool(viperPrefix + "enable_proxy"),
		caFile:                 ViperGetString(viperPrefix + "ca"),
		certFile:               ViperGetString(viperPrefix + "cert"),
		keyFile:                ViperGetString(viperPrefix + "key"),
		enableMenu:             ViperGetBool(viperPrefix + "menu"),
	}

	hostCache, err := NewHostCache(s.NetbootDir, s.HttpPort, s.HttpsPort, s.proxy)
	if err != nil {
		return nil, err
	}
	s.hosts = hostCache

	InternalShutdownRequest = make(chan struct{}, 1)

	if s.debug {
		log.Printf("[%s] config: %s\n", s.Name, FormatJSON(s.GetConfig()))
	}

	return &s, nil
}

func (s *NetbootServer) GetConfig() map[string]any {
	prefix := ViperKey(getViperPrefix())
	cfg := make(map[string]any)
	for _, key := range viper.AllKeys() {
		if strings.HasPrefix(key, prefix) {
			cfg[key] = viper.Get(key)
		}
	}
	return cfg
}

func (s *NetbootServer) Stop() error {
	log.Printf("[%s] requesting shutdown", s.Name)
	s.shutdown <- struct{}{}
	log.Printf("[%s] waiting for shutdown", s.Name)
	s.wg.Wait()
	log.Printf("[%s] shutdown complete", s.Name)
	return nil
}

func (s *NetbootServer) Start() error {

	httpMux := http.NewServeMux()
	httpMux.HandleFunc("GET /", s.hosts.RootHandler)
	httpMux.HandleFunc("GET /san/", s.hosts.SanBootHandler)
	if s.proxy {
		httpMux.HandleFunc("GET /alpine/", s.hosts.AlpineHandler)
		httpMux.HandleFunc("GET /debian/", s.hosts.DebianHandler)
		httpMux.HandleFunc("GET /debian-security/", s.hosts.DebianSecurityHandler)
		httpMux.HandleFunc("GET /pub/OpenBSD/", s.hosts.OpenBSDHandler)
	}

	httpsMux := http.NewServeMux()
	httpsMux.HandleFunc("GET /", s.hosts.RootHandlerTLS)
	httpsMux.HandleFunc("GET /version/", s.hosts.VersionHandlerTLS)
	httpsMux.HandleFunc("GET /utc", s.hosts.UTCHandlerTLS)
	httpsMux.HandleFunc("GET /gdl/{version}/{arch}/{file}", s.hosts.GDLHandlerTLS)
	httpsMux.HandleFunc("GET /ipxe/", s.hosts.IPXEHandlerTLS)
	httpsMux.HandleFunc("GET /iso/{key}/", s.hosts.ISOHandlerTLS)
	httpsMux.HandleFunc("GET /netboot.png", s.hosts.PNGHandlerTLS)
	httpsMux.HandleFunc("GET /api/hosts/", s.hosts.ListHostsHandlerTLS)
	httpsMux.HandleFunc("GET /api/address/{mac}/", s.hosts.HostAddressQueryHandlerTLS)
	httpsMux.HandleFunc("PUT /api/host/", s.hosts.AddHostHandlerTLS)
	httpsMux.HandleFunc("PUT /api/whitelist/{ip}/", s.hosts.AddWhitelistAddressHandlerTLS)
	httpsMux.HandleFunc("PUT /api/isokey/{key}/{mac}/{mode}/", s.hosts.AddIsoKeyHandlerTLS)
	httpsMux.HandleFunc("PUT /api/status/{status}/{mac}/{ip}/", s.hosts.PutHostStatusHandlerTLS)
	httpsMux.HandleFunc("GET /api/set_status/{status}/{mac}/{ip}/", s.hosts.GetHostSetStatusHandlerTLS)
	httpsMux.HandleFunc("PUT /api/bootstrap/{id}/{mac}/{ip}/", s.hosts.PutBootstrapHandlerTLS)
	httpsMux.HandleFunc("DELETE /api/isokey/{key}/", s.hosts.DeleteIsoKeyHandlerTLS)
	httpsMux.HandleFunc("DELETE /api/whitelist/{ip}/", s.hosts.DeleteWhitelistAddressHandlerTLS)
	httpsMux.HandleFunc("DELETE /api/host/", s.hosts.DeleteHostHandlerTLS)
	httpsMux.HandleFunc("POST /api/tarball/", s.hosts.UploadPackageHandlerTLS)
	httpsMux.HandleFunc("POST /api/dist/", s.hosts.UploadDistFileHandlerTLS)
	httpsMux.HandleFunc("POST /api/shutdown/", s.hosts.ShutdownHandlerTLS)

	if s.proxy {
		httpsMux.HandleFunc("GET /alpine/", s.hosts.AlpineHandlerTLS)
		httpsMux.HandleFunc("GET /debian/", s.hosts.DebianHandlerTLS)
		httpsMux.HandleFunc("GET /debian-security/", s.hosts.DebianSecurityHandlerTLS)
		httpsMux.HandleFunc("GET /pub/OpenBSD/", s.hosts.OpenBSDHandlerTLS)
	}

	httpsServer := &http.Server{
		Addr:    fmt.Sprintf("%s:%d", s.Address, s.HttpsPort),
		Handler: httpsMux,
	}

	httpServer := &http.Server{
		Addr:    fmt.Sprintf("%s:%d", s.Address, s.HttpPort),
		Handler: httpMux,
	}

	if s.certFile != "" || s.keyFile != "" || s.caFile != "" {
		if s.certFile == "" || s.keyFile == "" || s.caFile == "" {
			return fmt.Errorf("incomplete TLS config: cert=%s key=%s ca=%s\n", s.certFile, s.keyFile, s.caFile)
		}

		cert, err := tls.LoadX509KeyPair(s.certFile, s.keyFile)
		if err != nil {
			return fmt.Errorf("error loading client certificate pair: %v", err)
		}

		caCerts, err := os.ReadFile(s.caFile)
		if err != nil {
			return fmt.Errorf("error loading certificate authority file: %v", err)
		}

		clientCertPool := x509.NewCertPool()
		ok := clientCertPool.AppendCertsFromPEM(caCerts)
		if !ok {
			return fmt.Errorf("error loading client validation certificate authority file: %v", err)
		}

		httpsServer.TLSConfig = &tls.Config{
			ServerName:   s.ServerName,
			Certificates: []tls.Certificate{cert},
			ClientAuth:   tls.VerifyClientCertIfGiven,
			ClientCAs:    clientCertPool,
		}
		//fmt.Printf("configured TLS: %s %s %s\n", caFile, certFile, keyFile)
	}

	log.Printf("[%s] v%s started as PID %d\n", s.Name, Version, os.Getpid())

	s.wg.Add(1)
	go func() {
		defer log.Printf("[%s] HTTPS server exiting", s.Name)
		defer s.wg.Done()
		log.Printf("[%s] HTTPS server listening on %s\n", s.Name, httpsServer.Addr)
		err := httpsServer.ListenAndServeTLS("", "")
		if err != nil && err != http.ErrServerClosed {
			log.Fatalf("[%s] ListenAndServeTLS failed: %v", s.Name, err)
		}
	}()

	s.wg.Add(1)
	go func() {
		defer log.Printf("[%s] HTTP server exiting", s.Name)
		defer s.wg.Done()
		log.Printf("[%s] HTTP server listening on %s\n", s.Name, httpServer.Addr)
		err := httpServer.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			log.Fatalf("[%s] ListenAndServe failed: %v", s.Name, err)
		}
	}()

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		<-s.shutdown
		log.Printf("[%s] received shutdown request", s.Name)
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(s.shutdownTimeoutSeconds)*time.Second)
		defer cancel()

		log.Printf("[%s] shutting down HTTPS server", s.Name)
		err := httpsServer.Shutdown(ctx)
		if err != nil {
			log.Fatalf("[%s] HTTPS Server Shutdown failed: %v", s.Name, err)
		}

		log.Printf("[%s] shutting down HTTP server", s.Name)
		err = httpServer.Shutdown(ctx)
		if err != nil {
			log.Fatalf("[%s] HTTP Server Shutdown failed: %v", s.Name, err)
		}
	}()

	// FIXME: detect listening server port here instead of just sleeping
	time.Sleep(1 * time.Second)

	return nil
}

func (s *NetbootServer) Run() error {

	err := s.Start()
	if err != nil {
		return Fatal(err)
	}

	var sysMenu *menu.Menu

	menuClicked := make(chan *menu.MenuItem, 1)
	menuExited := make(chan struct{}, 1)

	if s.enableMenu {
		sysMenu = menu.NewMenu("Netboot Server v"+Version, "Boxen Netboot Server", []byte{}, menuClicked, menuExited)
		sysMenu.AddQuitItem("Shutdown", "Shutdown server and exit")
		sysMenu.AddSeparator()
		sysMenu.AddItem("Ping", "Write message to log")
	}

	sigint := make(chan os.Signal, 1)
	signal.Notify(sigint, syscall.SIGINT)
	sigterm := make(chan os.Signal, 1)
	signal.Notify(sigterm, syscall.SIGTERM)

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		menuStopRequired := s.enableMenu
		defer wg.Done()
		for done := false; !done; {
			select {
			case <-sigint:
				log.Printf("[%s] received SIGINT\n", s.Name)
				done = true
			case <-sigterm:
				log.Printf("[%s] received SIGTERM\n", s.Name)
				done = true
			case <-InternalShutdownRequest:
				log.Printf("[%s] shutdown requested\n", s.Name)
				done = true
			case item := <-menuClicked:
				log.Printf("[%s] menu clicked: %s\n", s.Name, (*item).Title)
			case <-menuExited:
				log.Printf("[%s] tooltray menu exited\n", s.Name)
				done = true
				menuStopRequired = false
			}
		}
		if menuStopRequired {
			err := sysMenu.Stop()
			if err != nil {
				Warning("Menu Stop failed with: %v", err)
			}
		}
	}()

	if s.verbose {
		fmt.Println("CTRL-C to exit")
	}

	if s.enableMenu {
		err = sysMenu.Run()
		if err != nil {
			return Fatal(err)
		}
	}

	wg.Wait()

	err = s.Stop()
	if err != nil {
		return Fatal(err)
	}

	return nil
}
