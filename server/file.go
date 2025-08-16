package server

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"io/fs"
	"log"
	"os"
)

func IsFileFS(filesystem fs.FS, pathname string) bool {
	fileInfo, err := fs.Stat(filesystem, pathname)
	if err != nil {
		return false
	}
	return fileInfo.Mode().IsRegular()
}

func IsDirFS(filesystem fs.FS, pathname string) bool {
	fileInfo, err := fs.Stat(filesystem, pathname)
	if err != nil {
		return false
	}
	return fileInfo.IsDir()
}

func CopyFile(dstPathname, srcPathname string) error {
	srcFile, err := os.Open(srcPathname)
	if err != nil {
		return Fatal(err)
	}
	defer srcFile.Close()
	dstFile, err := os.Create(dstPathname)
	if err != nil {
		return Fatal(err)
	}
	defer dstFile.Close()
	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return Fatal(err)
	}
	return nil
}

func CopyFileFromFS(dstPathname, srcPathname string, srcFS fs.FS) error {
	srcFile, err := srcFS.Open(srcPathname)
	if err != nil {
		return Fatal(err)
	}
	defer srcFile.Close()
	dstFile, err := os.Create(dstPathname)
	if err != nil {
		return Fatal(err)
	}
	defer dstFile.Close()
	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return Fatal(err)
	}
	return nil
}

func ExtractTarballFile(dstPath, srcPath, tarPath string) error {
	log.Printf("ExtractTarballFile(%s, %s, %s)\n", dstPath, srcPath, tarPath)
	tarFile, err := os.Open(tarPath)
	if err != nil {
		return Fatal(err)
	}
	defer tarFile.Close()
	gzReader, err := gzip.NewReader(tarFile)
	if err != nil {
		return Fatal(err)
	}
	defer gzReader.Close()
	tarReader := tar.NewReader(gzReader)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return Fatal(err)
		}
		if header.Name == srcPath {
			dstFile, err := os.Create(dstPath)
			if err != nil {
				return Fatal(err)
			}
			defer dstFile.Close()
			_, err = io.Copy(dstFile, tarReader)
			if err != nil {
				return Fatal(err)
			}
			return nil
		}
	}
	return Fatal(os.ErrNotExist)
}
