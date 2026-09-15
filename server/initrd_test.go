package server

import (
	"github.com/rstms/netboot/files"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"testing"
)

func TestNetbootServerInitrdGenerate(t *testing.T) {

	cache, err := os.UserCacheDir()
	require.Nil(t, err)

	distFS := os.DirFS(filepath.Join(cache, "netboot", "dist"))

	srcInitrd := filepath.Join("debian", "trixie", "amd64", "initrd.gz")
	dstInitrd := filepath.Join("testdata", "initrd.gz")

	err = files.CopyFileFromFS(dstInitrd, srcInitrd, distFS)
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
