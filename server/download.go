package server

import (
	"crypto/tls"
	"crypto/x509"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

func IsDirRoot(root *os.Root, pathname string) bool {
	fileInfo, err := root.Stat(pathname)
	if err != nil {
		return false
	}
	return fileInfo.IsDir()
}

func IsFileRoot(root *os.Root, pathname string) bool {
	fileInfo, err := root.Stat(pathname)
	if err != nil {
		return false
	}
	return fileInfo.Mode().IsRegular()
}

func MkdirAllRoot(root *os.Root, pathname string) error {
	dirs := []string{}
	var dir string
	for pathname != "." {
		pathname, dir = filepath.Split(pathname)
		dirs = append(dirs, dir)
		pathname = filepath.Clean(pathname)
	}
	slices.Reverse(dirs)
	pathname = ""
	for _, dir := range dirs {
		pathname = filepath.Join(pathname, dir)
		if !IsDirRoot(root, pathname) {
			log.Printf("creating cache directory: %s\n", pathname)
			err := root.Mkdir(pathname, 0700)
			if err != nil {
				return Fatal(err)
			}
		}
	}
	return nil
}

func DownloadFileRoot(root *os.Root, mirrorUrl, requestPath string) (string, error) {

	//log.Printf("DownloadFileRoot(<root>, %s, %s)\n", mirrorUrl, requestPath)

	pathname := filepath.FromSlash(strings.TrimLeft(requestPath, "/"))
	if IsFileRoot(root, pathname) {
		log.Printf("cached: %s\n", pathname)
		return pathname, nil
	}
	if IsDirRoot(root, pathname) {
		return "", Fatalf("path is directory: %s", pathname)
	}
	dir, _ := filepath.Split(pathname)
	if dir != "" {
		err := MkdirAllRoot(root, dir)
		if err != nil {
			return "", Fatal(err)
		}
	}
	file, err := root.Create(pathname)
	if err != nil {
		return "", Fatal(err)
	}
	defer file.Close()
	parsed, err := url.Parse(mirrorUrl)
	if err != nil {
		return "", Fatal(err)
	}
	client := &http.Client{}
	switch parsed.Scheme {
	case "http":
	case "https":
		certPool, err := x509.SystemCertPool()
		if err != nil {
			return "", Fatal(err)
		}
		client.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{RootCAs: certPool},
		}
	default:
		return "", Fatalf("unexpected scheme: %s", mirrorUrl)
	}
	fileUrl := mirrorUrl + requestPath
	log.Printf("downloading %s\n", fileUrl)
	response, err := client.Get(fileUrl)
	if err != nil {
		return "", Fatal(err)
	}
	defer response.Body.Close()

	count, err := io.Copy(file, response.Body)
	if err != nil {
		return "", Fatal(err)
	}
	log.Printf("ok (%d bytes)\n", count)
	return pathname, nil
}
