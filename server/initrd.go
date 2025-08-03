package server

import (
	"compress/gzip"
	"fmt"
	"github.com/cavaliergopher/cpio"
	"io"
	"io/fs"
)

type InitFile struct {
	Name string
	Data []byte
	Mode fs.FileMode
	UID  int
	GID  int
}

type Initrd struct {
	files     []InitFile
	reader    *cpio.Reader
	zipReader *gzip.Reader
	writer    *cpio.Writer
	zipWriter *gzip.Writer
	written   bool
}

func NewInitrd(r io.Reader) (*Initrd, error) {
	i := Initrd{
		files: []InitFile{},
	}
	var err error
	i.zipReader, err = gzip.NewReader(r)
	if err != nil {
		return nil, err
	}
	i.reader = cpio.NewReader(i.zipReader)
	return &i, nil
}

func (i *Initrd) Write(w io.Writer) error {
	i.zipWriter = gzip.NewWriter(w)
	i.writer = cpio.NewWriter(i.zipWriter)
	for {
		header, err := i.reader.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		//fmt.Printf("read header: %s type=%v, size=%d\n", header.Name, header.Mode, header.Size)

		size := header.Size

		err = i.writer.WriteHeader(header)
		if err != nil {
			return err
		}
		//fmt.Printf("\twrote header: %s type=%v, size=%d\n", header.Name, header.Mode, header.Size)

		if size > 0 {

			var readSize int64
			var writeSize int64
			for readSize < size {
				buf := make([]byte, header.Size)
				rChunk, err := i.reader.Read(buf)
				if err != nil {
					if err != io.EOF {
						return err
					}
				}
				//fmt.Printf("\tread data: %d bytes\n", rChunk)
				readSize += int64(rChunk)

				wChunk, err := i.writer.Write(buf[:rChunk])
				if err != nil {
					return err
				}
				//fmt.Printf("\twrote data: %d bytes\n", wChunk)
				writeSize += int64(wChunk)
			}

			if readSize != header.Size {
				return fmt.Errorf("read total length (%d) mismatches header size (%d)\n", readSize, header.Size)
			}

			if writeSize != header.Size {
				return fmt.Errorf("write total length (%d) mismatches header size (%d)\n", writeSize, header.Size)
			}
		}
	}
	for _, file := range i.files {
		header := cpio.Header{
			Name: file.Name,
			Size: int64(len(file.Data)),
			Mode: cpio.FileMode(file.Mode),
			Uid:  file.UID,
			Guid: file.GID,
		}
		//fmt.Printf("adding: %s\n", header.Name)
		err := i.writer.WriteHeader(&header)
		if err != nil {
			return err
		}
		_, err = i.writer.Write(file.Data)
		if err != nil {
			return err
		}
	}
	i.written = true
	return nil
}

func (i *Initrd) AddFile(name string, data []byte, mode fs.FileMode, uid, gid int) error {
	if i.written {
		return fmt.Errorf("cannot add; already written")
	}
	file := InitFile{name, data, mode, uid, gid}
	i.files = append(i.files, file)
	return nil
}

func (i *Initrd) Close() error {
	if i.reader != nil {
		i.reader = nil
	}
	if i.zipReader != nil {
		err := i.zipReader.Close()
		if err != nil {
			return err
		}
		i.zipReader = nil
	}
	if i.writer != nil {
		err := i.writer.Close()
		if err != nil {
			return err
		}
		i.writer = nil
	}
	if i.zipWriter != nil {
		err := i.zipWriter.Close()
		if err != nil {
			return err
		}
		i.zipWriter = nil
	}
	return nil
}
