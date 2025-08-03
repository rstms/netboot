package bootimg

import (
	"bytes"
	"github.com/rstms/netboot/common"
	"github.com/rstms/netboot/template"
	"github.com/stretchr/testify/require"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

const autoexecMarker = "echo ### modified autoexec ###"

func initTestFiles(t *testing.T) (string, string, string) {
	efi := filepath.Join("testdata", "BOOTX64.EFI")
	if !common.IsFile(efi) {
		src, err := template.Ipxe.Open(filepath.Join("ipxe", "BOOTX64.EFI"))
		require.Nil(t, err)
		defer src.Close()
		dst, err := os.Create(efi)
		require.Nil(t, err)
		_, err = io.Copy(dst, src)
		require.Nil(t, err)
	}

	autoexec := filepath.Join("testdata", "autoexec.ipxe")
	if !common.IsFile(autoexec) {
		err := os.WriteFile(autoexec, []byte(autoexecMarker+"\n"), 0600)
		require.Nil(t, err)
	}

	image := filepath.Join("testdata", "efi.img")
	if common.IsFile(image) {
		err := os.Remove(image)
		require.Nil(t, err)
	}

	return efi, autoexec, image
}

func run(t *testing.T, bin string, args ...string) string {
	cmd := exec.Command(bin, args...)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	err := cmd.Run()
	require.Nil(t, err)
	return stdout.String()
}

func TestCreateEFIImage(t *testing.T) {
	efi, autoexec, image := initTestFiles(t)
	err := CreateEFIImage(image, efi, autoexec)
	require.Nil(t, err)
	response := run(t, "mdir", "-i", image)
	log.Println(response)
}
