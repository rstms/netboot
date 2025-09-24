package files

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

func IsFileFS(filesystem fs.FS, pathname string) bool {
	pathname = strings.ReplaceAll(pathname, "\\", "/")
	fileInfo, err := fs.Stat(filesystem, pathname)
	if err != nil {
		return false
	}
	return fileInfo.Mode().IsRegular()
}

func IsDirFS(filesystem fs.FS, pathname string) bool {
	pathname = strings.ReplaceAll(pathname, "\\", "/")
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

func UnzipFileFromFS(dstPathname, srcPathname string, srcFS fs.FS) error {

	tempDir, err := os.MkdirTemp("", "unzip-*")
	if err != nil {
		return Fatal(err)
	}
	defer os.RemoveAll(tempDir)
	tempPathname := filepath.Join(tempDir, "unzipped")
	err = CopyFileFromFS(tempPathname, srcPathname, srcFS)
	if err != nil {
		return Fatal(err)
	}
	err = Unzip(dstPathname, tempPathname)
	if err != nil {
		return Fatal(err)
	}

	return nil
}

func CopyFileFromFS(dstPathname, srcPathname string, srcFS fs.FS) error {
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

type LinkInfo struct {
	name    string
	size    int64
	modtime time.Time
	mode    fs.FileMode
}

func (i *LinkInfo) Name() string {
	return i.name
}

func (i *LinkInfo) Size() int64 {
	return i.size
}

func (i *LinkInfo) Mode() fs.FileMode {
	return i.mode
}

func (i *LinkInfo) ModTime() time.Time {
	return i.modtime
}

func (i *LinkInfo) IsDir() bool {
	return false
}

func (i *LinkInfo) Sys() any {
	return nil
}

func WriteTarball(dstPath, srcDir string, chownRoot bool, symlinks []string, modes map[string]fs.FileMode) error {
	log.Printf("WriteTarball(%s, %s %v [%v])\n", dstPath, srcDir, chownRoot, symlinks)
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
		tarpath = strings.ReplaceAll(tarpath, "\\", "/")
		info, err := entry.Info()
		if err != nil {
			return Fatal(err)
		}
		var linkTarget string
		if slices.Contains(symlinks, path) {
			log.Printf("link detected: %s %s\n", filepath.Join(srcDir, path, entry.Name()), tarpath)
			linkData, err := os.ReadFile(path)
			if err != nil {
				return Fatal(err)
			}
			linkTarget = string(linkData)
			// create simulated fs.FileInfo for the link
			linkInfo := LinkInfo{
				name:    info.Name(),
				modtime: info.ModTime(),
				mode:    (info.Mode() & fs.ModePerm) | fs.ModeSymlink,
			}
			info = &linkInfo
		}
		/*
			if entry.Type()&fs.ModeSymlink != 0 {
				link, err = os.Readlink(path)
				if err != nil {
					return Fatal(err)
				}
			}
		*/
		header, err := tar.FileInfoHeader(info, linkTarget)
		if err != nil {
			return Fatal(err)
		}
		mode, ok := modes[path]
		if ok {
			hmode := fs.FileMode(header.Mode)
			bmode := hmode
			hmode &= ^fs.ModePerm
			hmode |= mode & fs.ModePerm
			header.Mode = int64(hmode)
			log.Printf("header mode b=%x a=%x %s\n", bmode, header.Mode, path)
		}
		header.Name = tarpath
		if chownRoot {
			header.Uid = 0
			header.Uname = "root"
			header.Gid = 0
			header.Gname = "wheel"
		}
		log.Printf("writing tar header: %s %+v\n", tarpath, header)
		err = tarWriter.WriteHeader(header)
		if err != nil {
			return Fatal(err)
		}
		//entry.Type().IsRegular() {
		if info.Mode().IsRegular() {
			log.Printf("writing tar data: %s\n", tarpath)
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
		} else {
			log.Printf("not writing tar data: %s\n", tarpath)
		}

		return nil
	})
	if err != nil {
		return Fatal(err)
	}
	return nil
}
