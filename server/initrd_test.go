package server

import (
	"github.com/rstms/netboot/template"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"testing"
)

const distInitrd = "debian/dists/bookworm/main/installer-amd64/current/images/netboot/debian-installer/amd64/initrd.gz"

func TestInitrdGenerate(t *testing.T) {

	err := copyFileFS("testdata", "initrd.gz", template.Debian, distInitrd)
	require.Nil(t, err)

	initrd, err := UnzipFile(filepath.Join("testdata", "initrd.gz"))
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
