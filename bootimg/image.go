package bootimg

import (
	fs "github.com/rstms/go-fs"
	"github.com/rstms/go-fs/fat"
	"io"
	"log"
	"os"
	"path/filepath"
)

/*
func walk(dir string, entries []fs.DirectoryEntry, target *string) ([]string, error) {
	//log.Printf("walkFS: %+v\n", entries)
	files := []string{}
	for _, entry := range entries {
		//log.Printf("entry Name=%s isDir=%v %+v\n", entry.Name(), entry.IsDir(), entry)
		name := path.Join(dir, entry.Name())
		if entry.IsDir() {
			if entry.Name() != "." && entry.Name() != ".." {
				if target != nil {
					targetDir := filepath.Join(*target, name)
					log.Printf("MKDIR %s\n", targetDir)
					err := os.Mkdir(targetDir, 0700)
					if err != nil {
						return []string{}, err
					}
				}
				files = append(files, name+"/")
				entryDir, err := entry.Dir()
				if err != nil {
					return []string{}, err
				}
				dirFiles, err := walk(name, entryDir.Entries(), target)
				if err != nil {
					return []string{}, err
				}
				for _, dirFile := range dirFiles {
					if dirFile != name {
						files = append(files, dirFile)
					}
				}
			}
		} else {
			files = append(files, name)
			if target != nil {
				targetName := filepath.Join(*target, name)
				log.Printf("COPY %v -> %s\n", name, targetName)
				entryFile, err := entry.File()
				if err != nil {
					return []string{}, err
				}
				err = copyFile(targetName, entryFile, entry.Size())
				if err != nil {
					return []string{}, err
				}
			}
		}
	}
	return files, nil
}

func copyFile(dstName string, src fs.File, size int64) error {
	log.Printf("copyFile(%s, %+v, %d)\n", dstName, src, size)
	dst, err := os.Create(dstName)
	if err != nil {
		return err
	}
	defer dst.Close()
	count, err := io.Copy(dst, src)
	if err != nil {
		return err
	}
	if count != size {
		return fmt.Errorf("write count mismatch size=%d written=%d\n", size, count)
	}
	return nil
}

func scanImage(imageFilename string, target *string) ([]string, error) {

	imageFile, err := os.Open(imageFilename)
	if err != nil {
		return []string{}, err
	}
	defer imageFile.Close()

	// BlockDevice backed by the file for our filesystem
	device, err := fs.NewFileDisk(imageFile)
	if err != nil {
		return []string{}, err
	}

	// The actual FAT filesystem
	ffs, err := fat.New(device)
	if err != nil {
		return []string{}, err
	}

	// Get the root directory to the filesystem
	rootDir, err := ffs.RootDir()
	if err != nil {
		return []string{}, err
	}

	entries := rootDir.Entries()
	if len(entries) < 2 {
		return []string{}, nil
	}
	files, err := walk("/", entries[1:], target)
	if err != nil {
		return []string{}, err
	}

	return files, nil
}

func ListFiles(imageFilename string) ([]string, error) {
	return scanImage(imageFilename, nil)
}

func ExtractFiles(imageFilename, outputDirectory string) error {
	err := os.Mkdir(outputDirectory, 0700)
	if err != nil {
		return err
	}
	_, err = scanImage(imageFilename, &outputDirectory)
	if err != nil {
		return err
	}
	return nil
}

func copyImage(dstImage, srcImage string) error {
	src, err := os.Open(srcImage)
	if err != nil {
		return err
	}
	defer src.Close()
	dst, err := os.Create(dstImage)
	if err != nil {
		return err
	}
	defer dst.Close()
	_, err = io.Copy(dst, src)
	if err != nil {
		return err
	}
	return nil
}

func UpdateAutoexec(dstImage, srcImage, autoexec string) error {
	err := copyImage(dstImage, srcImage)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(dstImage, os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	device, err := fs.NewFileDisk(f)
	if err != nil {
		return err
	}
	filesys, err := fat.New(device)
	if err != nil {
		return err
	}
	rootDir, err := filesys.RootDir()
	if err != nil {
		return err
	}
	entry, err := rootDir.AddFile("autoexec.ipxe")
	if err != nil {
		return err
	}
	aeDst, err := entry.File()
	if err != nil {
		return err
	}
	aeSrc, err := os.Open(autoexec)
	if err != nil {
		return err
	}
	defer aeSrc.Close()
	_, err = io.Copy(aeDst, aeSrc)
	if err != nil {
		return err
	}
	return nil
}
*/

func createImageFile(filename string) error {
	df, err := os.Create(filename)
	if err != nil {
		return err
	}
	df.Close()
	err = os.Truncate(filename, 1440*1024)
	if err != nil {
		return err
	}
	return nil
}

func addFile(dir fs.Directory, filename string) error {
	entry, err := dir.AddFile(filepath.Base(filename))
	if err != nil {
		return err
	}
	dst, err := entry.File()
	if err != nil {
		return err
	}
	src, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer src.Close()
	_, err = io.Copy(dst, src)
	if err != nil {
		return err
	}
	return nil
}

func CreateEFIImage(dstImage, efiBin, autoexec string) error {
	log.Printf("CreateEFIImage(%s, %s, %s)\n", dstImage, efiBin, autoexec)
	err := createImageFile(dstImage)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(dstImage, os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	device, err := fs.NewFileDisk(f)
	if err != nil {
		f.Close()
		return err
	}
	defer device.Close()
	cfg := fat.SuperFloppyConfig{
		FATType: fat.FAT12,
		Label:   "IPXE",
	}
	err = fat.FormatSuperFloppy(device, &cfg)
	if err != nil {
		return err
	}
	ffs, err := fat.New(device)
	if err != nil {
		return err
	}
	rootDir, err := ffs.RootDir()
	if err != nil {
		return err
	}
	err = addFile(rootDir, autoexec)
	if err != nil {
		return err
	}
	efiEntry, err := rootDir.AddDirectory("EFI")
	if err != nil {
		return err
	}
	efiDir, err := efiEntry.Dir()
	if err != nil {
		return err
	}
	bootEntry, err := efiDir.AddDirectory("BOOT")
	if err != nil {
		return err
	}
	bootDir, err := bootEntry.Dir()
	if err != nil {
		return err
	}
	err = addFile(bootDir, efiBin)
	if err != nil {
		return err
	}
	log.Printf("EFI image: %s\n", dstImage)
	return nil
}
