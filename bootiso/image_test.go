package bootiso

import (
	"github.com/rstms/boxen-template/template"
	common "github.com/rstms/go-common"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	"io"
	"os"
	"path/filepath"
	"testing"
)

var imageFile string

func initTestConfig(t *testing.T) {
	viper.SetConfigFile("testdata/config.yaml")
	err := viper.ReadInConfig()
	require.Nil(t, err)
}

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
		src, err := template.Ipxe.Open(filepath.Join("ipxe", name))
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

func TestISOCreate(t *testing.T) {
	outputImage := mkTestFile(t, "output.iso")
	sourceImage := templateFile(t, "netboot.xyz.iso")
	efiImage := templateFile(t, "efi.img")
	autoexecFile := templateFile(t, "autoexec.ipxe")
	rootFiles := []string{}
	err := CreateNetbootISOImage(outputImage, sourceImage, efiImage, autoexecFile, rootFiles)
	require.Nil(t, err)
}
