package server

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	//"github.com/rstms/netboot/template"
	"github.com/spf13/viper"
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
const DEFAULT_PORT = "2014"

type Server struct {
	Hostname   string
	Address    string
	Port       int
	verbose    bool
	debug      bool
	hosts      *HostCache
	wg         sync.WaitGroup
	shutdown   chan struct{}
	NetbootDir string
}

type Options struct {
	Hostname string
	Address  string
	Port     int
	CacheDir string
}

func NewServer(options *Options) (*Server, error) {
	viper.SetDefault("netboot.shutdown_timeout_seconds", 10)
	viper.SetDefault("netboot.hostname", DEFAULT_HOSTNAME)
	viper.SetDefault("netboot.address", DEFAULT_ADDRESS)
	viper.SetDefault("netboot.port", DEFAULT_PORT)
	userCache, err := os.UserCacheDir()
	if err != nil {
		return nil, err
	}
	viper.SetDefault("netboot.cache_dir", filepath.Join(userCache, "netboot"))
	if options == nil {
		options = &Options{}
	}
	if options.Hostname == "" {
		options.Hostname = viper.GetString("netboot.hostname")
	}
	if options.Address == "" {
		options.Address = viper.GetString("netboot.address")
	}
	if options.Port == 0 {
		options.Port = viper.GetInt("netboot.port")
	}
	if options.CacheDir == "" {
		options.CacheDir = viper.GetString("netboot.cache_dir")
	}
	hostCache, err := NewHostCache(options.CacheDir)
	if err != nil {
		return nil, err
	}
	s := Server{
		Hostname:   options.Hostname,
		Address:    options.Address,
		Port:       options.Port,
		verbose:    viper.GetBool("netboot.verbose"),
		debug:      viper.GetBool("netboot.verbose"),
		hosts:      hostCache,
		shutdown:   make(chan struct{}, 1),
		NetbootDir: options.CacheDir,
	}
	return &s, nil
}

/*
func (s *Server) handleEndpoints(w http.ResponseWriter, r *http.Request) {

	log.Printf("%s %s %s (%d)\n", r.RemoteAddr, r.Method, r.RequestURI, r.ContentLength)
	switch r.Method {
	case "GET":
		switch r.URL.Path {
		case "/api/hosts/":
			s.hosts.ListHostsHandler(w, r)
			return
		default:
			if strings.HasPrefix(r.URL.Path, "/api/booted/") {
				s.hosts.HostBootedHandler(w, r)
				return
			}
			if strings.HasPrefix(r.URL.Path, "/api/address/") {
				s.hosts.HostAddressQueryHandler(w, r)
				return
			}
			if strings.HasPrefix(r.URL.Path, "/pub/") {



			}
		}
	case "PUT":
		switch r.URL.Path {
		case "/api/host/":
			s.hosts.AddHostHandler(w, r)
			return
		}
	case "DELETE":
		switch r.URL.Path {
		case "/api/host/":
			s.hosts.DeleteHostHandler(w, r)
			return
		}
	case "POST":
		switch r.URL.Path {
		case "/api/tarball/":
			s.hosts.UploadPackageHandler(w, r)
			return
		}
	}
	http.Error(w, "WAT?", http.StatusNotFound)

}
*/

func (s *Server) Stop() error {
	log.Println("sending on shutdown channel")
	s.shutdown <- struct{}{}
	log.Println("waiting for shutdown")
	s.wg.Wait()
	log.Println("wait complete")
	return nil
}

func (s *Server) Start() error {

	netbootFS := os.DirFS(s.NetbootDir)
	http.HandleFunc("GET /", s.hosts.DefaultHandler)
	http.Handle("GET /{basename}", http.FileServer(http.FS(netbootFS)))
	/*
		http.Handle("GET /netboot/{basename}.ipxe{$}", http.FileServer(http.FS(netbootFS)))
		http.Handle("GET /netboot/{basename}.tgz{$}", http.FileServer(http.FS(netbootFS)))
		http.Handle("GET /netboot/{basename}.initrd{$}", http.FileServer(http.FS(netbootFS)))
		http.Handle("GET /netboot/{basename}.png{$}", http.FileServer(http.FS(netbootFS)))
		http.Handle("GET /netboot/{basename}.conf{$}", http.FileServer(http.FS(netbootFS)))
		http.Handle("GET /netboot/{basename}.initrd{$}", http.FileServer(http.FS(netbootFS)))
	*/
	http.HandleFunc("GET /api/hosts/", s.hosts.ListHostsHandler)
	http.HandleFunc("GET /api/booted/", s.hosts.HostBootedHandler)
	http.HandleFunc("GET /api/address/", s.hosts.HostAddressQueryHandler)
	//http.Handle("GET /pub/", http.FileServer(http.FS(template.OpenBSD)))
	//http.Handle("GET /alpine/", http.FileServer(http.FS(template.Alpine)))
	//http.Handle("GET /debian/", http.FileServer(http.FS(template.Debian)))
	http.HandleFunc("PUT /api/host/", s.hosts.AddHostHandler)
	http.HandleFunc("DELETE /api/host/", s.hosts.DeleteHostHandler)
	http.HandleFunc("POST /api/tarball/", s.hosts.UploadPackageHandler)

	listen := fmt.Sprintf("%s:%d", s.Address, s.Port)
	server := &http.Server{
		Addr: listen,
	}

	certFile := viper.GetString("netboot.cert")
	keyFile := viper.GetString("netboot.key")
	caFile := viper.GetString("netboot.ca")

	if certFile != "" || keyFile != "" || caFile != "" {
		tlsConfig := tls.Config{}
		if certFile == "" || keyFile == "" || caFile == "" {
			return fmt.Errorf("incomplete TLS config: cert=%s key=%s ca=%s\n", certFile, keyFile, caFile)
		}

		cert, err := tls.LoadX509KeyPair(os.ExpandEnv(certFile), os.ExpandEnv(keyFile))
		if err != nil {
			return fmt.Errorf("error loading client certificate pair: %v", err)
		}

		caCert, err := ioutil.ReadFile(os.ExpandEnv(caFile))
		if err != nil {
			return fmt.Errorf("error loading certificate authority file: %v", err)
		}

		caCertPool, err := x509.SystemCertPool()
		if err != nil {
			return fmt.Errorf("error opening system certificate pool: %v", err)
		}
		caCertPool.AppendCertsFromPEM(caCert)
		tlsConfig.Certificates = []tls.Certificate{cert}
		tlsConfig.RootCAs = caCertPool
		server.TLSConfig = &tlsConfig
	}

	s.wg.Add(1)
	go func() {
		defer log.Println("exiting listener")
		defer s.wg.Done()
		log.Printf("netboot v%s started as PID %d listening on %s\n", Version, os.Getpid(), listen)
		err := server.ListenAndServeTLS("", "")
		if err != nil && err != http.ErrServerClosed {
			log.Fatalln("ListenAndServe failed: ", err)
		}
		log.Println("ListenAndServerTLS returned")
	}()

	s.wg.Add(1)
	go func() {
		defer log.Println("exiting closer")
		defer s.wg.Done()
		<-s.shutdown
		log.Println("read from shutdown channel")
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(viper.GetInt64("netboot.shutdown_timeout_seconds"))*time.Second)
		defer cancel()
		log.Println("calling server shutdown")
		err := server.Shutdown(ctx)
		if err != nil {
			log.Fatalln("Server Shutdown failed: ", err)
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
