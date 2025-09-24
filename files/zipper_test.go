package files

import (
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"testing"
)

func TestFilesZip(t *testing.T) {
	file := filepath.Join("testdata", "zipme")
	zfile := filepath.Join("testdata", "zipme.gz")
	if IsFile(file) {
		err := os.Remove(file)
		require.Nil(t, err)
	}
	if IsFile(zfile) {
		err := os.Remove(zfile)
		require.Nil(t, err)
	}

	data := []byte("howdy, howdy, howdy\n")

	err := os.WriteFile(file, data, 0600)
	require.Nil(t, err)

	name, err := ZipFile(file)
	require.Nil(t, err)
	require.Equal(t, zfile, name)
	require.False(t, IsFile(file))
	require.True(t, IsFile(zfile))

	name, err = UnzipFile(zfile)
	require.Nil(t, err)
	require.Equal(t, file, name)

	require.False(t, IsFile(zfile))
	require.True(t, IsFile(file))

	check, err := os.ReadFile(file)
	require.Nil(t, err)

	require.Equal(t, data, check)
}
