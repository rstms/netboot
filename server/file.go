package server

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
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
	log.Printf("CopyFileFromFS(%s, %s, %v)\n", dstPathname, srcPathname, srcFS)
	srcPathname = strings.ReplaceAll(srcPathname, "\\", "/")
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

func WriteTarball(dstPath, srcDir string, chownRoot bool) error {
	log.Printf("WriteTarball(%s, %s)\n", dstPath, srcDir)
	tarFile, err := os.Create(dstPath)
	if err != nil {
		return Fatal(err)
	}
	defer tarFile.Close()
	gzWriter := gzip.NewWriter(tarFile)
	defer gzWriter.Close()
	tarWriter := tar.NewWriter(gzWriter)
	defer tarWriter.Close()
	err = filepath.WalkDir(srcDir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == srcDir {
			return nil
		}
		tarpath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return Fatal(err)
		}
		info, err := entry.Info()
		if err != nil {
			return Fatal(err)
		}
		var link string
		if entry.Type()&fs.ModeSymlink != 0 {
			link, err = os.Readlink(path)
			if err != nil {
				return Fatal(err)
			}
		}
		header, err := tar.FileInfoHeader(info, link)
		if err != nil {
			return Fatal(err)
		}
		header.Name = tarpath
		if chownRoot {
			header.Uid = 0
			header.Uname = "root"
			header.Gid = 0
			header.Gname = "wheel"
		}
		err = tarWriter.WriteHeader(header)
		if err != nil {
			return Fatal(err)
		}
		if entry.Type().IsRegular() {
			err := func() error {
				src, err := os.Open(path)
				if err != nil {
					return Fatal(err)
				}
				defer src.Close()
				_, err = io.Copy(tarWriter, src)
				if err != nil {
					return Fatal(err)
				}
				return nil
			}()
			if err != nil {
				return Fatal(err)
			}
		}
		return nil
	})
	if err != nil {
		return Fatal(err)
	}
	return nil
}
