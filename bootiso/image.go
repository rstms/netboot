package bootiso

import (
	"bytes"
	"fmt"
	diskfs "github.com/diskfs/go-diskfs"
	"github.com/diskfs/go-diskfs/disk"
	"github.com/diskfs/go-diskfs/filesystem"
	"github.com/diskfs/go-diskfs/filesystem/iso9660"
	"io"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
)

const (
	ISO_PAD_BYTES                = 1024
	ISO_LOGICAL_BLOCK_SIZE       = 2048
	EFI_DISK_SIZE          int64 = 1440 * 1024
	MINIMUM_ISO_SIZE             = 1024 * 48
)

func normalizePathname(pathname string) string {
	return strings.ReplaceAll(pathname, "\\", "/")
}

func walkFS(fs filesystem.FileSystem, dir string) ([]string, error) {
	entries, err := fs.ReadDir(dir)
	if err != nil {
		return []string{}, Fatal(err)
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
					return []string{}, Fatal(err)
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
		return nil, Fatal(err)
	}
	//log.Printf("opened disk: %+v\n", disk)

	fs, err := disk.GetFilesystem(0)
	if err != nil {
		return nil, Fatal(err)
	}
	//log.Printf("opened filesystem: %+v\n", fs)

	return fs, nil
}

func copyFileToImage(imageFS filesystem.FileSystem, dstPath string, srcPath string) error {
	log.Printf("copyFileToImage(%s %s)\n", dstPath, srcPath)
	dstPath = normalizePathname(dstPath)
	ifp, err := os.Open(srcPath)
	if err != nil {
		return Fatal(err)
	}
	defer ifp.Close()
	ofp, err := imageFS.OpenFile(dstPath, os.O_CREATE|os.O_RDWR)
	if err != nil {
		return Fatal(err)
	}
	defer ofp.Close()
	count, err := io.Copy(ofp, ifp)
	if err != nil {
		return Fatal(err)
	}
	log.Printf("wrote %d bytes to %s\n", count, dstPath)
	return nil
}

func copyFileFromImage(imageFS filesystem.FileSystem, dstPath string, srcPath string) error {
	//srcPath = strings.TrimLeft(srcPath, "/")
	log.Printf("copyFileFromImage(%s %s)\n", dstPath, srcPath)
	srcPath = normalizePathname(srcPath)
	ifp, err := imageFS.OpenFile(srcPath, os.O_RDONLY)
	if err != nil {
		return Fatal(err)
	}
	defer ifp.Close()
	//log.Printf("opened src: %v\n", ifp)
	ofp, err := os.Create(dstPath)
	if err != nil {
		return Fatal(err)
	}
	defer ofp.Close()
	//log.Printf("opened dst: %v\n", ifp)
	_, err = io.Copy(ofp, ifp)
	if err != nil {
		return Fatal(err)
	}
	return nil
}

func copyFileInterImage(dstFS filesystem.FileSystem, dstPath string, srcFS filesystem.FileSystem, srcPath string) error {
	log.Printf("copyFileInterImage(%s %s)\n", dstPath, srcPath)
	srcPath = normalizePathname(srcPath)
	dstPath = normalizePathname(dstPath)
	if strings.HasSuffix(dstPath, "/") {
		err := dstFS.Mkdir(dstPath)
		if err != nil {
			return Fatal(err)
		}
		return nil
	}
	ifp, err := srcFS.OpenFile(srcPath, os.O_RDONLY)
	if err != nil {
		return Fatal(err)
	}
	defer ifp.Close()
	ofp, err := dstFS.OpenFile(dstPath, os.O_CREATE|os.O_RDWR)
	if err != nil {
		return Fatal(err)
	}
	defer ofp.Close()
	_, err = io.Copy(ofp, ifp)
	if err != nil {
		return Fatal(err)
	}
	return nil
}

func ImageInfo(imageFile string) (string, int64, error) {
	stat, err := os.Stat(imageFile)
	if err != nil {
		return "", 0, Fatal(err)
	}
	size := stat.Size()
	fs, err := openImageFS(imageFile)
	if err != nil {
		return "", 0, Fatal(err)
	}
	//fmt.Printf("%+v\n", fs)
	name := strings.TrimSpace(fs.Label())
	return name, size, nil
}

func ListImageFiles(imageFilename string) ([]string, error) {

	fs, err := openImageFS(imageFilename)
	if err != nil {
		return []string{}, Fatal(err)
	}
	files, err := walkFS(fs, "/")
	if err != nil {
		return []string{}, Fatal(err)
	}
	return files, nil
}

func fileSize(filename string) (int64, error) {
	stat, err := os.Stat(filename)
	if err != nil {
		return 0, Fatal(err)
	}
	size := stat.Size()
	log.Printf("%s size: %d\n", filename, size)
	return size, nil
}

func CreateNetbootISO(dstImage, srcImage, efiImage string, rootFiles []string) error {

	log.Printf("CreateNetbootIsoImage: dst=%s src=%s efi=%s files=%v\n", dstImage, srcImage, efiImage, rootFiles)

	if IsFile(dstImage) {
		err := os.Remove(dstImage)
		if err != nil {
			return Fatal(err)
		}
	}

	var outputIsoSize int64 = ISO_PAD_BYTES
	var err error

	// add sizes of files to be added to ISO root dir
	for _, rootFile := range rootFiles {
		size, err := fileSize(rootFile)
		if err != nil {
			return Fatal(err)
		}
		outputIsoSize += size
	}

	srcImageName, srcImageSize, err := ImageInfo(srcImage)
	if err != nil {
		return Fatal(err)
	}
	outputIsoSize += srcImageSize

	log.Printf("srcImageName: %s\n", srcImageName)
	log.Printf("srcImageSize: %d\n", srcImageSize)

	tmpDir, err := os.MkdirTemp("", "netboot_isobuild_*")
	if err != nil {
		return Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	if efiImage != "" {
		efiImageSize, err := fileSize(efiImage)
		if err != nil {
			return Fatal(err)
		}
		outputIsoSize += efiImageSize
	}

	// open the source ISO filesystem
	srcFS, err := openImageFS(srcImage)
	if err != nil {
		return Fatal(err)
	}

	isoDisk, err := diskfs.Create(dstImage, outputIsoSize, diskfs.SectorSizeDefault)
	if err != nil {
		return Fatal(err)
	}

	//log.Printf("output ISO disk: %+v\n", isoDisk)

	isoDisk.LogicalBlocksize = ISO_LOGICAL_BLOCK_SIZE
	spec := disk.FilesystemSpec{
		FSType:      filesystem.TypeISO9660,
		VolumeLabel: srcImageName,
	}
	dstFS, err := isoDisk.CreateFilesystem(spec)
	if err != nil {
		return Fatal(err)
	}

	defer func() {
		err = dstFS.Close()
		if err != nil {
			Warning("%v", Fatalf("failure closing ISO9660 filesystem: %v", err))
		}
	}()

	//log.Printf("output ISO filesystem: %+v\n", dstFS)

	isoFiles, err := ListImageFiles(srcImage)
	if err != nil {
		return Fatal(err)
	}

	var elToritoBootFile string
	var elToritoLoadSize uint16
	// copy src ISO files to dest ISO
	for _, file := range isoFiles {
		log.Printf("isoFile: %s\n", file)
		pathname := normalizePathname(file)
		_, name := path.Split(pathname)
		switch name {
		case "autoexec.ipxe", "TRANS.TBL", "esp.img", "boot.catalog":
			log.Printf("skipping src: %s\n", pathname)
		default:
			switch name {
			case "cdbr":
				// use presence of cdbr to detect the OpenBSD installer
				elToritoBootFile = pathname
				elToritoLoadSize = 2
				// don't write the EFI image for OpenBSD
				efiImage = ""
			case "isolinux.bin":
				elToritoBootFile = pathname
				elToritoLoadSize = 4
			}
			log.Printf("copying: %s\n", file)
			err = copyFileInterImage(dstFS, file, srcFS, file)
			if err != nil {
				return Fatal(err)
			}
		}
	}

	var autoexecPresent bool

	// add rootFiles to ISO root directory
	for _, pathname := range rootFiles {
		_, name := filepath.Split(pathname)

		writeFile := true
		switch name {
		case "autoexec.ipxe.img":
			writeFile = false
		case "autoexec.ipxe":
			autoexecPresent = true
		case "autoexec.ipxe.iso":
			name = "autoexec.ipxe"
			autoexecPresent = true
		}
		if writeFile {
			log.Printf("adding: %s -> %s\n", pathname, name)
			err = copyFileToImage(dstFS, name, pathname)
			if err != nil {
				return Fatal(err)
			}
		}
	}

	if !autoexecPresent {
		return Fatalf("missing autoexec.ipxe in rootFiles: %+v", rootFiles)
	}

	log.Printf("efiImage: %s\n", efiImage)
	log.Printf("elToritoBootFile: %s\n", elToritoBootFile)

	elTorito := iso9660.ElTorito{Entries: []*iso9660.ElToritoEntry{}}

	if elToritoBootFile != "" {
		log.Printf("writing elTorito BIOS boot file: %s\n", elToritoBootFile)
		entry := iso9660.ElToritoEntry{
			Platform:  iso9660.BIOS,
			Emulation: iso9660.NoEmulation,
			BootFile:  elToritoBootFile,
			BootTable: true,
			LoadSize:  elToritoLoadSize,
		}
		elTorito.Entries = append(elTorito.Entries, &entry)
	}

	if efiImage != "" {
		log.Printf("writing generated EFI image: %s", efiImage)
		err = copyFileToImage(dstFS, "efi.img", efiImage)
		if err != nil {
			return Fatal(err)
		}
		entry := iso9660.ElToritoEntry{
			Platform:  iso9660.EFI,
			Emulation: iso9660.NoEmulation,
			BootFile:  "efi.img",
		}
		elTorito.Entries = append(elTorito.Entries, &entry)

	}

	options := iso9660.FinalizeOptions{
		VolumeIdentifier: srcImageName,
		RockRidge:        true,
		ElTorito:         &elTorito,
		DeepDirectories:  true,
	}

	/*
		iso9660.ElTorito{
			Entries: []*iso9660.ElToritoEntry{
				{
					Platform:  iso9660.BIOS,
					Emulation: iso9660.NoEmulation,
					BootFile:  elToritoBootfile,
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
	*/

	iso, ok := dstFS.(*iso9660.FileSystem)
	if !ok {
		return fmt.Errorf("filesystem is not iso9660")
	}
	log.Printf("finalizing: %+v\n", options)
	err = iso.Finalize(options)
	if err != nil {
		return Fatal(err)
	}
	log.Printf("finalized iso: %s\n", dstImage)
	return nil
}

func DirSize(srcDir string) (int64, error) {
	var size int64
	err := filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size, err
}

func createJolietISO(outFile, srcDir, volumeLabel string) error {

	cmd := exec.Command(
		"mkisofs",
		"-V", volumeLabel,
		"-r",
		"-J",
		"-o", outFile,
		srcDir,
	)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	log.Println(cmd)
	err := cmd.Run()
	if err != nil {
		return Fatal(err)
	}
	ostr := strings.TrimSpace(stdout.String())
	if ostr != "" {
		log.Printf("stdout: %s\n", ostr)
	}
	estr := strings.TrimSpace(stderr.String())
	if estr != "" {
		log.Printf("stderr: %s\n", estr)
	}
	return nil
}

func CreateISO(outFile, srcDir, volumeLabel string, joliet bool) error {

	log.Printf("CreateISO: out=%s src=%s label=%s joliet=%v\n", outFile, srcDir, volumeLabel, joliet)

	if joliet {
		return createJolietISO(outFile, srcDir, volumeLabel)
	}

	size, err := DirSize(srcDir)
	if err != nil {
		return Fatal(err)
	}

	if size < MINIMUM_ISO_SIZE {
		size = MINIMUM_ISO_SIZE
	}

	var logicalBlocksize diskfs.SectorSize = 2048

	isoDisk, err := diskfs.Create(outFile, size, logicalBlocksize)
	if err != nil {
		return Fatal(err)
	}

	fspec := disk.FilesystemSpec{
		Partition:   0,
		FSType:      filesystem.TypeISO9660,
		VolumeLabel: volumeLabel,
	}
	isofs, err := isoDisk.CreateFilesystem(fspec)
	if err != nil {
		return Fatal(err)
	}

	err = filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return Fatal(err)
		}

		if info.IsDir() {
			err = isofs.Mkdir(relPath)
			if err != nil {
				return Fatal(err)
			}
			return nil
		}

		if !info.IsDir() {
			rw, err := isofs.OpenFile(relPath, os.O_CREATE|os.O_RDWR)
			if err != nil {
				return Fatal(err)
			}

			in, err := os.Open(path)
			if err != nil {
				return Fatal(err)
			}
			defer in.Close()

			_, err = io.Copy(rw, in)
			if err != nil {
				return Fatal(err)
			}
		}
		return nil
	})
	if err != nil {
		return Fatal(err)
	}

	iso, ok := isofs.(*iso9660.FileSystem)
	if !ok {
		return Fatalf("not an iso9660 filesystem")
	}

	err = iso.Finalize(iso9660.FinalizeOptions{
		RockRidge: true,
	})
	if err != nil {
		return Fatal(err)
	}
	return nil
}
