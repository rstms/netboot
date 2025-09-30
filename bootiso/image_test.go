package bootiso

import (
	"github.com/rstms/netboot/files"
	"github.com/rstms/netboot/template"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
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

/*
func mkTestFile(t *testing.T, name string) string {
	testFile := filepath.Join("testdata", name)
	if common.IsFile(testFile) {
		err := os.Remove(testFile)
		require.Nil(t, err)
	}
	return testFile
}

func templateFile(t *testing.T, name string) string {
	testFile := filepath.Join("testdata", name)
	if !common.IsFile(testFile) {
		src, err := template.Ipxe.Open(path.Join("ipxe", name))
		require.Nil(t, err)
		defer src.Close()
		dst, err := os.Create(testFile)
		require.Nil(t, err)
		defer dst.Close()
		_, err = io.Copy(dst, src)
		require.Nil(t, err)
	}
	return testFile
}
*/

func TestNetbootBootISOCreate(t *testing.T) {
	outputImage := filepath.Join("testdata", "output.iso")
	sourceImage := filepath.Join("testdata", "netboot.xyz.iso")
	err := files.UnzipFileFromFS(sourceImage, path.Join("ipxe", "netboot.xyz.iso.gz"), template.Ipxe)
	require.Nil(t, err)
	efiImage := filepath.Join("testdata", "BOOTX64.EFI")
	err = files.UnzipFileFromFS(efiImage, path.Join("ipxe", "netboot.xyz.efi.gz"), template.Ipxe)
	require.Nil(t, err)
	autoexecFile := filepath.Join("testdata", "autoexec.ipxe")
	err = files.CopyFileFromFS(autoexecFile, path.Join("ipxe", "openbsd-autoexec.ipxe"), template.Ipxe)
	rootFiles := []string{autoexecFile}
	err = CreateNetbootISOImage(outputImage, sourceImage, efiImage, rootFiles)
	require.Nil(t, err)
}
