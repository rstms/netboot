package bootiso

import (
	common "github.com/rstms/go-common"
	"github.com/rstms/netboot/template"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	"io"
	"os"
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

func TestNetbootBootISOCreate(t *testing.T) {
	outputImage := mkTestFile(t, "output.iso")
	sourceImage := templateFile(t, "netboot.xyz.iso")
	efiImage := templateFile(t, "netboot.xyz.efi")
	autoexecFile := templateFile(t, "openbsd-autoexec.ipxe")
	rootFiles := []string{autoexecFile}
	err := CreateNetbootISOImage(outputImage, sourceImage, efiImage, rootFiles)
	require.Nil(t, err)
}
