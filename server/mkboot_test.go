package server

import (
	"github.com/rstms/ffs/image"
	"github.com/stretchr/testify/require"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestNetbootServerMkbootInjectFiles(t *testing.T) {
	srcFile := filepath.Join("testdata", "src.img")
	dstFile := filepath.Join("testdata", "dst.img")
	if IsFile(dstFile) {
		err := os.Remove(dstFile)
		require.Nil(t, err)
	}
	files := []string{
		filepath.Join("testdata", "foo"),
		filepath.Join("testdata", "bar"),
		filepath.Join("testdata", "baz"),
	}
	for _, file := range files {
		data, err := exec.Command("fortune").Output()
		require.Nil(t, err)
		err = os.WriteFile(file, data, 0600)
		require.Nil(t, err)
	}
	err := image.MungeImage(dstFile, srcFile, "testdata", files)
	require.Nil(t, err)
	cmd := exec.Command("mdir", "-i", dstFile)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	require.Nil(t, err)
}
