package files

import (
	"crypto/sha512"
	"encoding/hex"
	"io"
	"os"
)

func CalculateSHA512(filename string) (string, error) {
	ifp, err := os.Open(filename)
	if err != nil {
		return "", Fatal(err)
	}
	defer ifp.Close()
	hash := sha512.New()
	_, err = io.Copy(hash, ifp)
	if err != nil {
		return "", Fatal(err)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
