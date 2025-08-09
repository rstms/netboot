package server

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"github.com/cavaliergopher/cpio"
	"io"
	"io/fs"
	"log"
	"os"
)

type InitFile struct {
	DstName string
	SrcName string
	Mode    fs.FileMode
	UID     int
	GID     int
}

func GenerateInitrd(dstFilename string, srcData []byte, files []InitFile) error {
	echan := make(chan error)

	tempFile, err := os.CreateTemp("", "initrd*")
	if err != nil {
		return Fatal(err)
	}
	err = generate(tempFile, srcData, files, echan)
	if err != nil {
		return Fatal(err)
	}
	var messages string
	for done := false; !done; {
		select {
		case err := <-echan:
			messages += fmt.Sprintf("%v\n", err)
		default:
			done = true
		}
	}

	if messages != "" {
		return Fatalf("close failures: %s", messages)
	}

	err = zipOutput(dstFilename, tempFile.Name())
	if err != nil {
		return Fatal(err)
	}
	return nil
}

func zipOutput(dstFilename, srcFilename string) error {
	dstFile, err := os.Create(dstFilename)
	if err != nil {
		return Fatal(err)
	}
	defer dstFile.Close()

	zipper := gzip.NewWriter(dstFile)
	defer zipper.Close()

	srcFile, err := os.Open(srcFilename)
	if err != nil {
		return Fatal(err)
	}
	defer srcFile.Close()

	_, err = io.Copy(zipper, srcFile)
	if err != nil {
		return Fatal(err)
	}
	return nil
}

func generate(dstFile *os.File, srcData []byte, files []InitFile, echan chan error) error {

	unzipper, err := gzip.NewReader(bytes.NewBuffer(srcData))
	if err != nil {
		return Fatal(err)
	}
	defer func() {
		err := unzipper.Close()
		if err != nil {
			echan <- Fatalf("failed closing unzipper: %v", err)
		}
	}()

	reader := cpio.NewReader(unzipper)

	defer func() {
		err := dstFile.Close()
		if err != nil {
			echan <- Fatalf("failed closing output file: %v", err)
		}
	}()

	writer := cpio.NewWriter(dstFile)
	defer func() {
		err := writer.Close()
		if err != nil {
			echan <- Fatalf("failed closing cpio writer: %v", err)
		}
	}()

	for {
		header, err := reader.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			return Fatal(err)
		}
		//fmt.Printf("read header: %s type=%v, size=%d\n", header.Name, header.Mode, header.Size)

		size := header.Size

		err = writer.WriteHeader(header)
		if err != nil {
			return Fatal(err)
		}
		//fmt.Printf("\twrote header: %s type=%v, size=%d\n", header.Name, header.Mode, header.Size)

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
	}

	for _, file := range files {
		err := copyIn(writer, &file)
		if err != nil {
			return Fatal(err)
		}
	}

	log.Println("Initrd.Write complete")
	return nil
}

func copyIn(writer *cpio.Writer, file *InitFile) error {

	stat, err := os.Stat(file.SrcName)
	if err != nil {
		return Fatal(err)
	}

	header := cpio.Header{
		Name: file.DstName,
		Size: stat.Size(),
		Mode: cpio.FileMode(file.Mode),
		Uid:  file.UID,
		Guid: file.GID,
	}
	//fmt.Printf("adding: %s\n", header.Name)

	err = writer.WriteHeader(&header)
	if err != nil {
		return Fatal(err)
	}

	fp, err := os.Open(file.SrcName)
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
