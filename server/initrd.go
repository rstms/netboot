package server

import (
	"compress/gzip"
	"github.com/cavaliergopher/cpio"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type InitFile struct {
	DstName string
	SrcName string
	Mode    fs.FileMode
	UID     int
	GID     int
}

func GenerateInitrd(inputName, outputName string, fileNames []string) error {

	dir, err := os.MkdirTemp("", "cpio.*")
	if err != nil {
		return Fatal(err)
	}
	// fixme
	// defer os.RemoveAll(dir)

	headers, err := ExtractInitrdFiles(dir, inputName)
	if err != nil {
		return err
	}

	for _, fileName := range fileNames {
		_, name := filepath.Split(fileName)
		dstName := filepath.Join(dir, name)
		header := cpio.Header{
			Name:    dstName,
			Mode:    cpio.FileMode(0600),
			Uid:     0,
			Guid:    0,
			ModTime: time.Now(),
		}
		headers[dstName] = &header
		err := copyFile(dstName, fileName)
		if err != nil {
			Fatal(err)
		}
	}

	err = ArchiveInitrdFiles(outputName, dir, headers)
	if err != nil {
		Fatal(err)
	}

	return nil
}

func UnzipFile(filename string) (string, error) {
	if !strings.HasSuffix(filename, ".gz") {
		return "", Fatalf("not zipped: %s", filename)
	}
	dstName := filename[:len(filename)-3] + ".gz"
	err := unzip(dstName, filename)
	if err != nil {
		return "", Fatal(err)
	}
	err = os.Remove(filename)
	if err != nil {
		return "", Fatal(err)
	}
	return dstName, nil

}

func ZipFile(filename string) (string, error) {
	if strings.HasSuffix(filename, ".gz") {
		return "", Fatalf("already zipped: %s", filename)
	}
	dstName := filename + ".gz"
	err := zip(dstName, filename)
	if err != nil {
		return "", Fatal(err)
	}
	err = os.Remove(filename)
	if err != nil {
		return "", Fatal(err)
	}
	return dstName, nil
}

func zip(dstName, srcName string) error {

	srcFile, err := os.Open(srcName)
	if err != nil {
		return Fatal(err)
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dstName)
	if err != nil {
		return Fatal(err)
	}
	defer dstFile.Close()

	zipper := gzip.NewWriter(dstFile)
	defer zipper.Close()

	_, err = io.Copy(zipper, srcFile)
	if err != nil {
		return Fatal(err)
	}
	return nil
}

func unzip(dstName, srcName string) error {

	srcFile, err := os.Open(srcName)
	if err != nil {
		return Fatal(err)
	}
	defer srcFile.Close()

	unzipper, err := gzip.NewReader(srcFile)
	if err != nil {
		return Fatal(err)
	}
	defer unzipper.Close()

	dstFile, err := os.Create(dstName)
	if err != nil {
		return Fatal(err)
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, unzipper)
	if err != nil {
		return Fatal(err)
	}
	return nil
}

func ExtractInitrdFiles(dir, filename string) (map[string]*cpio.Header, error) {

	headers := make(map[string]*cpio.Header)

	srcFile, err := os.Open(filename)
	if err != nil {
		return headers, Fatal(err)
	}

	reader := cpio.NewReader(srcFile)

	for {
		header, err := reader.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			return headers, Fatal(err)
		}
		//fmt.Printf("read header: %s type=%v, size=%d\n", header.Name, header.Mode, header.Size)
		headers[header.Name] = header
		err = copyOut(dir, reader, header)
		if err != nil {
			return headers, Fatal(err)
		}
	}
	return headers, nil
}

/*
   Name     string	// Name of the file entry
   Linkname string	// Target name of link (valid for TypeLink or TypeSymlink)
   Links    int	// Number of inbound links
   Size int64		// Size in bytes
   Mode FileMode	// Permission and mode bits
   Uid  int		// User id of the owner
   Guid int		// Group id of the owner
   ModTime time.Time	// Modification time
*/

func ArchiveInitrdFiles(cpioFilename, srcDir string, headers map[string]*cpio.Header) error {
	names, err := walk(srcDir)
	if err != nil {
		return Fatal(err)
	}

	dstFile, err := os.Create(cpioFilename)
	if err != nil {
		return Fatal(err)
	}
	defer dstFile.Close()

	writer := cpio.NewWriter(dstFile)

	for _, name := range names {
		log.Printf("%s\n", name)
		header, ok := headers[name]
		if !ok {
			return Fatalf("no header found for: %s", name)
		}
		err := copyIn(writer, name, header)
		if err != nil {
			return Fatal(err)
		}
	}

	err = writer.Close()
	if err != nil {
		return Fatal(err)
	}

	return nil
}

// out-of the cpio archive
func copyOut(dstDir string, reader *cpio.Reader, header *cpio.Header) error {

	dstName := filepath.Join(dstDir, header.Name)
	dstFile, err := os.Create(dstName)
	if err != nil {
		return Fatal(err)
	}
	defer dstFile.Close()

	count, err := io.Copy(dstFile, reader)
	if err != nil {
		return Fatal(err)
	}

	if count != header.Size {
		return Fatalf("copy count (%d) mismatches header size (%d)\n", count, header.Size)
	}
	return nil
}

/*
	size := header.Size
	if size > 0 {

		var readSize int64
		var writeSize int64
		for readSize < size {
			buf := make([]byte, header.Size)
			rChunk, err := reader.Read(buf)
			if err != nil {
				if err != io.EOF {
					return Fatal(err)
				}
			}
			//fmt.Printf("\tread data: %d bytes\n", rChunk)
			readSize += int64(rChunk)

			wChunk, err := writer.Write(buf[:rChunk])
			if err != nil {
				return Fatal(err)
			}
			//fmt.Printf("\twrote data: %d bytes\n", wChunk)
			writeSize += int64(wChunk)
		}

		if readSize != header.Size {
			return Fatalf("read total length (%d) mismatches header size (%d)\n", readSize, header.Size)
		}

		if writeSize != header.Size {
			return Fatalf("write total length (%d) mismatches header size (%d)\n", writeSize, header.Size)
		}
	}
*/

// in-to the cpio archive
func copyIn(writer *cpio.Writer, srcName string, header *cpio.Header) error {

	stat, err := os.Stat(srcName)
	if err != nil {
		return Fatal(err)
	}

	fileHeader := cpio.Header{
		Name: srcName,
		Size: stat.Size(),
		Mode: cpio.FileMode(0600),
		Uid:  0,
		Guid: 0,
	}
	if fileHeader != *header {
		log.Printf("fileHeader: %s\n", FormatJSON(fileHeader))
		log.Printf("header: %s\n", FormatJSON(*header))
		return Fatalf("fileHeader != header")
	}

	//fmt.Printf("adding: %s\n", header.Name)
	err = writer.WriteHeader(header)
	if err != nil {
		return Fatal(err)
	}
	//fmt.Printf("\twrote header: %s type=%v, size=%d\n", header.Name, header.Mode, header.Size)

	fp, err := os.Open(srcName)
	if err != nil {
		return Fatal(err)
	}
	defer fp.Close()

	_, err = io.Copy(writer, fp)
	if err != nil {
		return Fatal(err)
	}
	return nil
}

func walk(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return []string{}, err
	}
	//fmt.Printf("walkFS: %s %v\n", dir, entries)
	names := []string{}
	for _, entry := range entries {
		//fmt.Printf("entry Name=%s isDir=%v %+v\n", entry.Name(), entry.IsDir(), entry)
		if entry.Name() == "NO NAME" {
			continue
		}
		name := filepath.Join(dir, entry.Name())
		if entry.IsDir() {
			if entry.Name() != "." && entry.Name() != ".." {
				names = append(names, name)
				dirNames, err := walk(filepath.Join(dir, entry.Name()))
				if err != nil {
					return []string{}, err
				}
				for _, dirName := range dirNames {
					if dirName != name {
						names = append(names, dirName)
					}
				}
			}
		} else {
			names = append(names, name)
		}
	}
	return names, nil
}
