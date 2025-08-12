package server

import (
	"io"
	"io/fs"
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
