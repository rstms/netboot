package bootiso

import (
	"fmt"
	diskfs "github.com/diskfs/go-diskfs"
	"github.com/diskfs/go-diskfs/disk"
	"github.com/diskfs/go-diskfs/filesystem"
	"github.com/diskfs/go-diskfs/filesystem/iso9660"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
)

const (
	ISO_PAD_BYTES                = 1024
	ISO_LOGICAL_BLOCK_SIZE       = 2048
	EFI_DISK_SIZE          int64 = 1440 * 1024
)

func walkFS(fs filesystem.FileSystem, dir string) ([]string, error) {
	entries, err := fs.ReadDir(dir)
	if err != nil {
		return []string{}, err
	}
	//fmt.Printf("walkFS: %s %v\n", dir, entries)
	files := []string{}
	for _, entry := range entries {
		//fmt.Printf("entry Name=%s isDir=%v %+v\n", entry.Name(), entry.IsDir(), entry)
		if entry.Name() == "NO NAME" {
			continue
		}
		name := filepath.Join(dir, entry.Name())
		if entry.IsDir() {
			if entry.Name() != "." && entry.Name() != ".." {
				files = append(files, name+"/")
				dirFiles, err := walkFS(fs, filepath.Join(dir, entry.Name()))
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
		}
	}
	return files, nil
}

func openImageFS(imageFilename string) (filesystem.FileSystem, error) {
	log.Printf("openImageFS(%s)\n", imageFilename)
	disk, err := diskfs.Open(imageFilename)
	if err != nil {
		return nil, err
	}
	//log.Printf("opened disk: %+v\n", disk)

	fs, err := disk.GetFilesystem(0)
	if err != nil {
		return nil, err
	}
	//log.Printf("opened filesystem: %+v\n", fs)

	return fs, nil
}

func copyFileToImage(imageFS filesystem.FileSystem, dstPath string, srcPath string) error {
	log.Printf("copyFileToImage(%s %s)\n", dstPath, srcPath)
	ifp, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer ifp.Close()
	ofp, err := imageFS.OpenFile(dstPath, os.O_CREATE|os.O_RDWR)
	if err != nil {
		return err
	}
	defer ofp.Close()
	_, err = io.Copy(ofp, ifp)
	if err != nil {
		return err
	}
	return nil
}

func copyFileFromImage(imageFS filesystem.FileSystem, dstPath string, srcPath string) error {
	//srcPath = strings.TrimLeft(srcPath, "/")
	log.Printf("copyFileFromImage(%s %s)\n", dstPath, srcPath)
	ifp, err := imageFS.OpenFile(srcPath, os.O_RDONLY)
	if err != nil {
		return err
	}
	defer ifp.Close()
	//log.Printf("opened src: %v\n", ifp)
	ofp, err := os.Create(dstPath)
	if err != nil {
		return err
	}
	defer ofp.Close()
	//log.Printf("opened dst: %v\n", ifp)
	_, err = io.Copy(ofp, ifp)
	if err != nil {
		return err
	}
	return nil
}

func copyFileInterImage(dstFS filesystem.FileSystem, dstPath string, srcFS filesystem.FileSystem, srcPath string) error {
	log.Printf("copyFileInterImage(%s %s)\n", dstPath, srcPath)
	ifp, err := srcFS.OpenFile(srcPath, os.O_RDONLY)
	if err != nil {
		return err
	}
	defer ifp.Close()
	ofp, err := dstFS.OpenFile(dstPath, os.O_CREATE|os.O_RDWR)
	if err != nil {
		return err
	}
	defer ofp.Close()
	_, err = io.Copy(ofp, ifp)
	if err != nil {
		return err
	}
	return nil
}

func ImageInfo(imageFile string) (string, int64, error) {
	stat, err := os.Stat(imageFile)
	if err != nil {
		return "", 0, err
	}
	size := stat.Size()
	fs, err := openImageFS(imageFile)
	if err != nil {
		return "", 0, err
	}
	//fmt.Printf("%+v\n", fs)
	name := strings.TrimSpace(fs.Label())
	return name, size, nil
}

func ListImageFiles(imageFilename string) ([]string, error) {

	fs, err := openImageFS(imageFilename)
	if err != nil {
		return []string{}, err
	}
	files, err := walkFS(fs, "/")
	if err != nil {
		return []string{}, err
	}
	return files, nil
}

func fileSize(label, filename string) (int64, error) {
	stat, err := os.Stat(filename)
	if err != nil {
		return 0, err
	}
	size := stat.Size()
	log.Printf("%s size (%s): %d\n", label, filename, size)
	return size, nil
}

func CreateNetbootISOImage(dstImage, srcImage, efiImage, autoexec string) error {

	log.Printf("CreateNetbootIsoImage: dst=%s src=%s efi=%s autoexec=%s\n", dstImage, srcImage, efiImage, autoexec)

	autoexecSize, err := fileSize("autoexec", autoexec)
	if err != nil {
		return err
	}

	srcImageName, srcImageSize, err := ImageInfo(srcImage)

	log.Printf("srcImageName: %s\n", srcImageName)
	log.Printf("srcImageSize: %d\n", srcImageSize)

	tmpDir, err := os.MkdirTemp("", "netboot_isobuild_*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	efiImageSize, err := fileSize("EFI Image", efiImage)
	if err != nil {
		return err
	}

	// open the source ISO filesystem
	srcFS, err := openImageFS(srcImage)
	if err != nil {
		return err
	}

	// ensure enough space by lamely adding the length of what we're adding
	// FIXME: this isn't frugal with the iso size
	outputIsoSize := srcImageSize + efiImageSize + autoexecSize + ISO_PAD_BYTES

	isoDisk, err := diskfs.Create(dstImage, outputIsoSize, diskfs.SectorSizeDefault)
	if err != nil {
		return err
	}

	//log.Printf("output ISO disk: %+v\n", isoDisk)

	isoDisk.LogicalBlocksize = ISO_LOGICAL_BLOCK_SIZE
	spec := disk.FilesystemSpec{
		FSType:      filesystem.TypeISO9660,
		VolumeLabel: srcImageName,
	}
	dstFS, err := isoDisk.CreateFilesystem(spec)
	if err != nil {
		return err
	}

	//log.Printf("output ISO filesystem: %+v\n", dstFS)

	isoFiles, err := ListImageFiles(srcImage)
	if err != nil {
		return err
	}

	var isoLinuxBin string
	// copy src ISO files to dest ISO
	for _, file := range isoFiles {
		switch file {
		case "/autoexec.ipxe":
			// copy the modified autoexec
			log.Printf("writing modified autoexec.ipxe: %s", autoexec)
			err = copyFileToImage(dstFS, "autoexec.ipxe", autoexec)
			if err != nil {
				return err
			}
		case "/esp.img":
			log.Printf("writing generated EFI image: %s", efiImage)
			err = copyFileToImage(dstFS, "efi.img", efiImage)
			if err != nil {
				return err
			}
		case "/boot.catalog":
			// don't copy (autogenerated)
		default:
			log.Printf("copying: %s\n", file)
			err = copyFileInterImage(dstFS, file, srcFS, file)
			if err != nil {
				return err
			}
		}
	}

	log.Printf("isoLinuxBin: %s\n", isoLinuxBin)
	log.Printf("efiImage: %s\n", efiImage)

	options := iso9660.FinalizeOptions{
		VolumeIdentifier: srcImageName,
		RockRidge:        true,
		ElTorito: &iso9660.ElTorito{
			Entries: []*iso9660.ElToritoEntry{
				{
					Platform:  iso9660.BIOS,
					Emulation: iso9660.NoEmulation,
					BootFile:  "isolinux.bin",
					BootTable: true,
					LoadSize:  4,
				},
				{
					Platform:  iso9660.EFI,
					Emulation: iso9660.NoEmulation,
					BootFile:  "efi.img",
				},
			},
		},
	}
	iso, ok := dstFS.(*iso9660.FileSystem)
	if !ok {
		return fmt.Errorf("filesystem is not iso9660")
	}
	log.Printf("finalizing: %+v\n", options)
	err = iso.Finalize(options)
	if err != nil {
		return err
	}
	log.Printf("finalized iso: %s\n", dstImage)
	return nil
}
