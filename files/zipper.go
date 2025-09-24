package files

import (
	"compress/gzip"
	"io"
	"log"
	"os"
	"strings"
)

func UnzipFile(filename string) (string, error) {
	log.Printf("UnzipFile(%s)\n", filename)
	if !strings.HasSuffix(filename, ".gz") {
		return "", Fatalf("not zipped: %s", filename)
	}
	dstName := filename[:len(filename)-3]
	err := Unzip(dstName, filename)
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
	log.Printf("ZipFile(%s)\n", filename)
	if strings.HasSuffix(filename, ".gz") {
		return "", Fatalf("already zipped: %s", filename)
	}
	dstName := filename + ".gz"
	err := Zip(dstName, filename)
	if err != nil {
		return "", Fatal(err)
	}
	err = os.Remove(filename)
	if err != nil {
		return "", Fatal(err)
	}
	return dstName, nil
}

func Zip(dstName, srcName string) error {

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

func Unzip(dstName, srcName string) error {

	dstFile, err := os.Create(dstName)
	if err != nil {
		return Fatal(err)
	}
	defer dstFile.Close()

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

	_, err = io.Copy(dstFile, unzipper)
	if err != nil {
		return Fatal(err)
	}
	return nil
}
