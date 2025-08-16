package server

import (
	"github.com/rstms/netboot/template"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"testing"
)

func TestInitrdGenerate(t *testing.T) {

	srcInitrd := filepath.Join("debian", "bookworm", "amd64", "initrd.gz")
	dstInitrd := filepath.Join("testdata", "initrd.gz")

	err := CopyFileFromFS(dstInitrd, srcInitrd, template.Dist)
	require.Nil(t, err)

	initrd, err := UnzipFile(dstInitrd)
	require.Nil(t, err)

	preseed := filepath.Join("testdata", "preseed.cfg")
	err = os.WriteFile(preseed, []byte("preseed file\n"), 0600)
	require.Nil(t, err)

	tarball := filepath.Join("testdata", "package.tgz")
	err = os.WriteFile(tarball, []byte("tarball file\n"), 0600)
	require.Nil(t, err)

	err = GenerateInitrd(initrd+".out", initrd, []string{preseed, tarball})
	require.Nil(t, err)
}
