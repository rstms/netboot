package server

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"embed"
	"fmt"
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

const Version = "1.0.0"
const DEFAULT_HOSTNAME = "localhost"
const DEFAULT_ADDRESS = "127.0.0.1"
const DEFAULT_HTTPS_PORT = "4443"
const DEFAULT_HTTP_PORT = "4444"
const DEFAULT_SHUTDOWN_TIMEOUT_SECONDS = 10

type Template struct {
	Certs  embed.FS
	Ipxe   embed.FS
	Dist   embed.FS
	Mkboot embed.FS
}

type Server struct {
	Hostname               string
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
	viperPrefix            string
}

type NetbootOption int

const (
	NetbootOptionDefault NetbootOption = iota
	NetbootOptionEnable
	NetbootOptionDisable
)

type Options struct {
	Hostname  string
	Address   string
	HttpPort  int
	HttpsPort int
	CacheDir  string
	Proxy     NetbootOption
	Template  *Template
}

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

func NewServer(viperPrefix string, options *Options) (*Server, error) {
	if viperPrefix == "" {
		viperPrefix = "netboot_server"
	}
	viper.SetDefault(viperPrefix+".hostname", DEFAULT_HOSTNAME)
	viper.SetDefault(viperPrefix+".address", DEFAULT_ADDRESS)
	viper.SetDefault(viperPrefix+".https_port", DEFAULT_HTTPS_PORT)
	viper.SetDefault(viperPrefix+".http_port", DEFAULT_HTTP_PORT)
	viper.SetDefault(viperPrefix+".shutdown_timeout_seconds", DEFAULT_SHUTDOWN_TIMEOUT_SECONDS)
	userCache, err := os.UserCacheDir()
	if err != nil {
		return nil, err
	}
	viper.SetDefault(viperPrefix+".cache_dir", filepath.Join(userCache, "netboot"))
	if options == nil {
		options = &Options{}
	}

	if options.Address == "" {
		options.Address = viper.GetString(viperPrefix + ".address")
	}
	if options.HttpsPort == 0 {
		options.HttpsPort = viper.GetInt(viperPrefix + ".https_port")
	}
	if options.HttpPort == 0 {
		options.HttpPort = viper.GetInt(viperPrefix + ".http_port")
	}
	if options.CacheDir == "" {
		options.CacheDir = viper.GetString(viperPrefix + ".cache_dir")
	}
	hostCache, err := NewHostCache(options.CacheDir, options.Template)
	if err != nil {
		return nil, err
	}
	s := Server{
		Address:                options.Address,
		HttpPort:               options.HttpPort,
		HttpsPort:              options.HttpsPort,
		verbose:                viper.GetBool(viperPrefix + ".verbose"),
		debug:                  viper.GetBool(viperPrefix + ".debug"),
		hosts:                  hostCache,
		shutdown:               make(chan struct{}, 1),
		NetbootDir:             options.CacheDir,
		shutdownTimeoutSeconds: viper.GetInt(viperPrefix + ".shutdown_timeout_seconds"),
		viperPrefix:            viperPrefix,
	}

	s.caFile, err = expandFilename(viper.GetString(viperPrefix + ".ca"))
	if err != nil {
		return nil, err
	}
	s.certFile, err = expandFilename(viper.GetString(viperPrefix + ".cert"))
	if err != nil {
		return nil, err
	}
	s.keyFile, err = expandFilename(viper.GetString(viperPrefix + ".key"))
	if err != nil {
		return nil, err
	}

	switch options.Proxy {
	case NetbootOptionEnable:
		s.proxy = true
	case NetbootOptionDisable:
		s.proxy = false
	default:
		s.proxy = viper.GetBool(viperPrefix + ".enable_proxy")
	}

	s.hosts.httpPort = s.HttpPort
	s.hosts.httpsPort = s.HttpsPort
	s.hosts.proxy = s.proxy

	return &s, nil
}

func (s *Server) GetConfig() map[string]any {
	cfg := make(map[string]any)
	for _, key := range viper.AllKeys() {
		if strings.HasPrefix(key, s.viperPrefix+".") {
			cfg[key] = viper.Get(key)
		}
	}
	return cfg
}

func (s *Server) Stop() error {
	log.Println("requesting shutdown")
	s.shutdown <- struct{}{}
	log.Println("waiting for shutdown")
	s.wg.Wait()
	log.Println("shutdown complete")
	return nil
}

func (s *Server) Start() error {

	httpMux := http.NewServeMux()
	httpMux.HandleFunc("GET /", s.hosts.RootHandler)
	httpMux.HandleFunc("GET /san/", s.hosts.SanBootHandler)

	if s.proxy {
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
	httpsMux.HandleFunc("GET /netboot.png", s.hosts.PNGHandlerTLS)
	httpsMux.HandleFunc("GET /api/hosts/", s.hosts.ListHostsHandlerTLS)
	httpsMux.HandleFunc("GET /api/booted/{mac}/{ip}/", s.hosts.HostBootedHandlerTLS)
	httpsMux.HandleFunc("GET /api/address/", s.hosts.HostAddressQueryHandlerTLS)
	httpsMux.HandleFunc("PUT /api/host/", s.hosts.AddHostHandlerTLS)
	httpsMux.HandleFunc("DELETE /api/host/", s.hosts.DeleteHostHandlerTLS)
	httpsMux.HandleFunc("POST /api/tarball/", s.hosts.UploadPackageHandlerTLS)

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
			Certificates: []tls.Certificate{cert},
			ClientAuth:   tls.VerifyClientCertIfGiven,
			ClientCAs:    clientCertPool,
		}
		//fmt.Printf("configured TLS: %s %s %s\n", caFile, certFile, keyFile)
	}

	log.Printf("netboot v%s started as PID %d\n", Version, os.Getpid())

	s.wg.Add(1)
	go func() {
		defer log.Println("HTTPS server exiting")
		defer s.wg.Done()
		log.Printf("HTTPS server listening on %s\n", httpsServer.Addr)
		err := httpsServer.ListenAndServeTLS("", "")
		if err != nil && err != http.ErrServerClosed {
			log.Fatalln("ListenAndServeTLS failed: ", err)
		}
		//log.Println("returned from ListenAndServeTLS")
	}()

	s.wg.Add(1)
	go func() {
		defer log.Println("HTTP server exiting")
		defer s.wg.Done()
		log.Printf("HTTP server listening on %s\n", httpServer.Addr)
		err := httpServer.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			log.Fatalln("ListenAndServe failed: ", err)
		}
		//log.Println("returned from ListenAndServe")
	}()

	s.wg.Add(1)
	go func() {
		//defer log.Println("exiting closer")
		defer s.wg.Done()
		<-s.shutdown
		log.Println("received shutdown request")
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(s.shutdownTimeoutSeconds)*time.Second)
		defer cancel()

		log.Println("shutting down HTTPS server")
		err := httpsServer.Shutdown(ctx)
		if err != nil {
			log.Fatalln("HTTPS Server Shutdown failed: ", err)
		}

		log.Println("shutting down HTTP server")
		err = httpServer.Shutdown(ctx)
		if err != nil {
			log.Fatalln("HTTP Server Shutdown failed: ", err)
		}
	}()

	return nil
}

func (s *Server) Run(message string) error {
	err := s.Start()
	if err != nil {
		return err
	}
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	if message != "" {
		fmt.Println(message)
	}
	<-sigs
	fmt.Println("received shutdown signal")
	return s.Stop()
}
