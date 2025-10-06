package bootiso

import (
	"github.com/rstms/netboot/files"
	"github.com/rstms/netboot/template"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"testing"
)

var imageFile string

func initTestConfig(t *testing.T) {
	viper.SetConfigFile("testdata/config.yaml")
	err := viper.ReadInConfig()
	require.Nil(t, err)
}

func TestCreateNetbootISO(t *testing.T) {

	outputImage, err := filepath.Abs(filepath.Join("testdata", "output.iso"))
	require.Nil(t, err)
	if IsFile(outputImage) {
		err = os.Remove(outputImage)
		require.Nil(t, err)
	}

	sourceImage, err := filepath.Abs(filepath.Join("testdata", "netboot.xyz.iso"))
	require.Nil(t, err)
	if IsFile(sourceImage) {
		err = os.Remove(sourceImage)
		require.Nil(t, err)
	}
	err = files.UnzipFileFromFS(sourceImage, path.Join("ipxe", "netboot.xyz.iso.gz"), template.Ipxe)
	require.Nil(t, err)

	efiImage, err := filepath.Abs(filepath.Join("testdata", "BOOTX64.EFI"))
	require.Nil(t, err)
	if IsFile(efiImage) {
		err = os.Remove(efiImage)
		require.Nil(t, err)
	}
	err = files.UnzipFileFromFS(efiImage, path.Join("ipxe", "netboot.xyz.efi.gz"), template.Ipxe)
	require.Nil(t, err)

	autoexecFile, err := filepath.Abs(filepath.Join("testdata", "autoexec.ipxe"))
	require.Nil(t, err)
	if IsFile(autoexecFile) {
		err = os.Remove(autoexecFile)
		require.Nil(t, err)
	}
	err = files.CopyFileFromFS(autoexecFile, path.Join("ipxe", "openbsd-autoexec.ipxe"), template.Ipxe)
	require.Nil(t, err)

	rootFiles := []string{autoexecFile}
	err = CreateNetbootISO(outputImage, sourceImage, efiImage, rootFiles)
	require.Nil(t, err)
}

func TestCreateISO(t *testing.T) {
	srcDir := filepath.Join("testdata", "isofiles")
	isoFile := filepath.Join("testdata", "out.iso")
	if IsFile(isoFile) {
		err := os.Remove(isoFile)
		require.Nil(t, err)
	}
	err := CreateISO(isoFile, srcDir, "test_iso", true)
	require.Nil(t, err)
	info, err := exec.Command("isoinfo", "-d", "-i", isoFile).Output()
	require.Nil(t, err)
	log.Println(string(info))
}
