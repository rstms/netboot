package server

import (
	"github.com/stretchr/testify/require"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestNetbootServerCreateEFIImage(t *testing.T) {
	dstImage := filepath.Join("testdata", "efi.img")
	efiBin := filepath.Join("testdata", "BOOTX64.EFI")
	autoexec := filepath.Join("testdata", "autoexec.ipxe")
	err := CreateEFIImage(dstImage, efiBin, autoexec)
	require.Nil(t, err)

	cmd := exec.Command("mdir", "-i", dstImage)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	err = cmd.Run()
	require.Nil(t, err)

}
