package server

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"embed"
	"fmt"
	"github.com/spf13/viper"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

const Version = "1.0.0"
const DEFAULT_HOSTNAME = "netboot.local"
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
	Hostname   string
	Address    string
	HttpsPort  int
	HttpPort   int
	verbose    bool
	debug      bool
	proxy      bool
	hosts      *HostCache
	wg         sync.WaitGroup
	shutdown   chan struct{}
	NetbootDir string
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

func NewServer(options *Options) (*Server, error) {
	viper.SetDefault("netboot_server.shutdown_timeout_seconds", DEFAULT_SHUTDOWN_TIMEOUT_SECONDS)
	viper.SetDefault("netboot_server.hostname", DEFAULT_HOSTNAME)
	viper.SetDefault("netboot_server.address", DEFAULT_ADDRESS)
	viper.SetDefault("netboot_server.https_port", DEFAULT_HTTPS_PORT)
	viper.SetDefault("netboot_server.http_port", DEFAULT_HTTP_PORT)
	userCache, err := os.UserCacheDir()
	if err != nil {
		return nil, err
	}
	viper.SetDefault("netboot_server.cache_dir", filepath.Join(userCache, "netboot"))
	if options == nil {
		options = &Options{}
	}
	if options.Hostname == "" {
		options.Hostname = viper.GetString("netboot_server.hostname")
	}
	if options.Address == "" {
		options.Address = viper.GetString("netboot_server.address")
	}
	if options.HttpsPort == 0 {
		options.HttpsPort = viper.GetInt("netboot_server.https_port")
	}
	if options.HttpPort == 0 {
		options.HttpPort = viper.GetInt("netboot_server.http_port")
	}
	if options.CacheDir == "" {
		options.CacheDir = viper.GetString("netboot_server.cache_dir")
	}
	hostCache, err := NewHostCache(options.CacheDir, options.Template)
	if err != nil {
		return nil, err
	}
	s := Server{
		Hostname:   options.Hostname,
		Address:    options.Address,
		HttpPort:   options.HttpPort,
		HttpsPort:  options.HttpsPort,
		verbose:    viper.GetBool("netboot_server.verbose"),
		debug:      viper.GetBool("netboot_server.debug"),
		hosts:      hostCache,
		shutdown:   make(chan struct{}, 1),
		NetbootDir: options.CacheDir,
	}

	switch options.Proxy {
	case NetbootOptionEnable:
		s.proxy = true
	case NetbootOptionDisable:
		s.proxy = false
	default:
		s.proxy = viper.GetBool("netboot_server.enable_proxy")
	}

	s.hosts.httpPort = s.HttpPort
	s.hosts.httpsPort = s.HttpsPort
	s.hosts.proxy = s.proxy

	return &s, nil
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

	certFile := viper.GetString("netboot_server.cert")
	keyFile := viper.GetString("netboot_server.key")
	caFile := viper.GetString("netboot_server.ca")

	if certFile != "" || keyFile != "" || caFile != "" {
		if certFile == "" || keyFile == "" || caFile == "" {
			return fmt.Errorf("incomplete TLS config: cert=%s key=%s ca=%s\n", certFile, keyFile, caFile)
		}

		cert, err := tls.LoadX509KeyPair(os.ExpandEnv(certFile), os.ExpandEnv(keyFile))
		if err != nil {
			return fmt.Errorf("error loading client certificate pair: %v", err)
		}

		caCerts, err := ioutil.ReadFile(os.ExpandEnv(caFile))
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
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(viper.GetInt64("netboot_server.shutdown_timeout_seconds"))*time.Second)
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

func (s *Server) Wait() error {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	<-sigs
	fmt.Println("received shutdown signal")
	return s.Stop()
}
