package server

import (
	"github.com/rstms/netboot/files"
	"github.com/rstms/netboot/template"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"testing"
)

func TestNetbootServerInitrdGenerate(t *testing.T) {

	srcInitrd := filepath.Join("dist", "debian", "bookworm", "amd64", "initrd.gz")
	dstInitrd := filepath.Join("testdata", "initrd.gz")

	err := files.CopyFileFromFS(dstInitrd, srcInitrd, template.Dist)
	require.Nil(t, err)

	initrd, err := files.UnzipFile(dstInitrd)
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
