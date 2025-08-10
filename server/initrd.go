package server

import (
	"bytes"
	"github.com/cavaliergopher/cpio"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type InitFile struct {
	DstName string
	SrcName string
	Mode    fs.FileMode
	UID     int
	GID     int
}

type Record struct {
	Header cpio.Header
	Data   []byte
}

func GenerateInitrd(outputName, inputName string, fileNames []string) error {

	rchan := make(chan Record)
	erchan := make(chan error, 1)
	ewchan := make(chan error, 1)
	writerExit := make(chan struct{})

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(erchan)
		defer close(rchan)
		defer log.Println("cpio reader returning")

		srcFile, err := os.Open(inputName)
		if err != nil {
			erchan <- Fatal(err)
			return
		}

		var bytecount, filecount, headercount int64

		reader := cpio.NewReader(srcFile)

		log.Printf("cpio reader started: %s\n", inputName)

		for {
			select {
			case _, ok := <-writerExit:
				if !ok {
					erchan <- Fatalf("writer has exited")
					return
				}
			default:
			}

			header, err := reader.Next()
			if err != nil {
				if err == io.EOF {
					break
				}
				erchan <- Fatal(err)
			}
			record := Record{Header: *header}

			if !header.Mode.IsRegular() && header.Size != 0 {
				log.Printf("header %d mode=%d Size=%d\n", headercount, header.Mode, header.Size)
				log.Println(FormatJSON(header))
			}

			headercount += 1

			//log.Printf("header: %s type=%s, size=%d\n", header.Name, header.Mode, header.Size)
			//log.Println(FormatJSON(*header))
			if header.Size > 0 {
				filecount += 1
				data, err := readCPIO(reader, header.Size)
				if err != nil {
					erchan <- Fatal(err)
					return
				}
				bytecount += int64(len(data))
				record.Data = data
			}
			//log.Printf("reader sending %s %d %s\n", record.Header.Name, record.Header.Size, record.Header.Mode)
			rchan <- record
		}

		for _, fileName := range fileNames {

			select {
			case _, ok := <-writerExit:
				if !ok {
					erchan <- Fatalf("writer has exited")
					return
				}
			default:
			}

			fileInfo, err := os.Stat(fileName)
			if err != nil {
				erchan <- Fatal(err)
				return
			}
			_, name := filepath.Split(fileName)
			record := Record{
				Header: cpio.Header{
					Name:    name,
					Size:    fileInfo.Size(),
					Mode:    cpio.TypeReg | 0600,
					Uid:     0,
					Guid:    0,
					ModTime: time.Now(),
				},
			}
			data, err := os.ReadFile(fileName)
			if err != nil {
				erchan <- Fatal(err)
				return
			}
			record.Data = data
			rchan <- record
		}

		log.Printf("cpio reader: bytes=%d files=%d headers=%d\n", bytecount, filecount, headercount)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(ewchan)
		defer func() {
			// drain the record channel to unblock the reader
			for _ = range rchan {
			}
		}()
		defer close(writerExit)
		defer log.Println("cpio writer returning")

		log.Printf("cpio writer started: %s\n", outputName)

		/*
			records := []Record{}
			for record := range rchan {
				records = append(records, record)
			}

			log.Printf("cpio writer received %d records", len(records))

			for i, record := range records {
				if record.Header.Size != int64(len(record.Data)) {
					log.Printf("record %d size mismatch: header.Size=%d data=(%d bytes)\n", i, record.Header.Size, len(record.Data))
					log.Println(FormatJSON(record.Header))
				}
			}
		*/

		dstFile, err := os.Create(outputName)
		if err != nil {
			ewchan <- Fatal(err)
			return
		}
		defer dstFile.Close()

		writer := cpio.NewWriter(dstFile)
		var filecount, headercount, bytecount int64

		for record := range rchan {

			//log.Printf("cpio writer received: %s %d %s\n", record.Header.Name, record.Header.Size, record.Header.Mode)

			if record.Header.Size != int64(len(record.Data)) {
				log.Println(FormatJSON(record.Header))
				ewchan <- Fatalf("record size mismatch: header.Size=%d data=(%d bytes)\n", record.Header.Size, len(record.Data))
				return
			}

			headercount += 1

			err := writer.WriteHeader(&record.Header)
			if err != nil {
				ewchan <- Fatal(err)
				return
			}

			if record.Header.Mode.IsRegular() {
				filecount += 1
			}

			datacount := len(record.Data)
			err = writeCPIO(writer, record.Data)
			if err != nil {
				ewchan <- Fatal(err)
				return
			}
			bytecount += int64(datacount)
		}

		err = writer.Close()
		if err != nil {
			ewchan <- Fatal(err)
			return
		}

		log.Printf("cpio writer: bytes=%d files=%d headers=%d\n", bytecount, filecount, headercount)

	}()

	log.Println("waiting for initrd cpio processing...")
	wg.Wait()
	log.Println("initrd complete")

	//log.Println("reading error channels...")
	ebuf := make([]error, 0)
	for err := range erchan {
		ebuf = append(ebuf, err)
	}
	for err := range ewchan {
		ebuf = append(ebuf, err)
	}
	//log.Println("error channels drained")
	if len(ebuf) > 0 {
		return Fatalf("%v", ebuf)
	}
	return nil
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

/*
// out-of the cpio archive
func copyOut(reader *cpio.Reader, header cpio.Header) ([]byte, error) {

	//log.Printf("copyOut(%s, %v, %+v)\n", dstDir, reader, header)

	if !header.Mode.IsRegular() {
		return []byte{}, nil
	}

	if header.Size == 0 {
		return []byte{}, nil
	}

	var buf bytes.Buffer

	count, err := io.Copy(&buf, reader)
	if err != nil {
		return []byte{}, Fatal(err)
	}

	if count != header.Size {
		return []byte{}, Fatalf("copy count (%d) mismatches header size (%d)\n", count, header.Size)
	}

	data := buf.Bytes()

	if count != int64(len(data)) {
		return []byte{}, Fatalf("copy count (%d) mismatches buffer size (%d)\n", count, len(data))
	}

	return data, nil
}
*/

func readCPIO(reader *cpio.Reader, size int64) ([]byte, error) {

	if size == 0 {
		return []byte{}, nil
	}

	var obuf bytes.Buffer
	remaining := size
	if size > 0 {
		var readSize int64
		var writeSize int64
		for readSize < size {
			buf := make([]byte, remaining)
			chunk, err := reader.Read(buf)
			if err != nil {
				if err != io.EOF {
					return []byte{}, Fatal(err)
				}
			}
			if int64(chunk) != remaining {
				log.Printf("partial read: wanted %d, got %d\n", remaining, chunk)
			}
			readSize += int64(chunk)
			remaining -= int64(chunk)

			count, err := obuf.Write(buf[:chunk])
			if err != nil {
				return []byte{}, Fatal(err)
			}
			if count != chunk {
				return []byte{}, Fatalf("output buffer write count mismatch: wanted %d, got %d", chunk, count)
			}
		}

		if readSize != size {
			return []byte{}, Fatalf("read total length (%d) mismatches header size (%d)\n", readSize, size)
		}

		if int64(obuf.Len()) != size {
			return []byte{}, Fatalf("output buffer length (%d) mismatches header size (%d)\n", writeSize, size)
		}
	}
	return obuf.Bytes(), nil
}

func writeCPIO(writer *cpio.Writer, data []byte) error {

	size := len(data)
	if size == 0 {
		return nil
	}
	var written int
	remaining := size
	for written < size {
		if len(data) != remaining {
			return Fatalf("buffer length (%d) mismatches remaining (%d)\n", len(data), remaining)
		}
		chunk, err := writer.Write(data)
		if err != nil {
			return Fatal(err)
		}
		written += chunk
		if chunk != remaining {
			log.Printf("partial write: expected %d, got %d\n", remaining, chunk)
			remaining -= chunk
			data = data[:chunk]
		} else {
			if written != size {
				panic("expected written==size")
			}
		}
	}

	if written != size {
		return Fatalf("write count (%d) mismatches size (%d)\n", written, size)
	}

	/*
		err := writer.Flush()
		if err != nil {
			return Fatal(err)
		}
	*/

	return nil
}

/*
// in-to the cpio archive
func copyIn(writer *cpio.Writer, header cpio.Header, data []byte) (int64, error) {

	//fmt.Printf("adding: %s\n", header.Name)
	err := writer.WriteHeader(&header)
	if err != nil {
		return 0, Fatal(err)
	}

	//fmt.Printf("\twrote header: %s type=%v, size=%d\n", header.Name, header.Mode, header.Size)

	var count int64

	if header.Mode.IsRegular() && header.Size > 0 {
		buf := bytes.NewBuffer(data)
		count, err = io.Copy(writer, buf)
		if err != nil {
			return 0, Fatal(err)
		}
		if count != header.Size {
			return 0, Fatalf("write count (%d) mismatches header size (%d)\n", count, header.Size)
		}
	}
	return count, nil
}
*/
