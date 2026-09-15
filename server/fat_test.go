package server

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestServerFatIsFat(t *testing.T) {
	fatImage := filepath.Join("testdata", "src.img")
	result, err := IsFAT(fatImage)
	require.Nil(t, err)
	require.True(t, result)
	partImage := filepath.Join("testdata", "partitioned.img")
	result, err = IsFAT(partImage)
	require.Nil(t, err)
	require.False(t, result)
}

func TestServerFatReadPartitions(t *testing.T) {
	partImage := filepath.Join("testdata", "partitioned.img")
	partitions, err := ReadMBRPartitions(partImage)
	require.Nil(t, err)
	require.Equal(t, 4, len(partitions))

	for i, partition := range partitions {
		fmt.Printf("%d: %+v\n", i, partition)
	}
}

func TestServerInjectBootFiles(t *testing.T) {
	srcImage := filepath.Join("testdata", "partitioned.img")
	testImage := filepath.Join("testdata", "injected.img")
	err := exec.Command("cp", srcImage, testImage).Run()
	require.Nil(t, err)
	files := []string{
		filepath.Join("testdata", "foo"),
		filepath.Join("testdata", "bar"),
		filepath.Join("testdata", "baz"),
	}
	err = InjectBootFiles(testImage, files)
	require.Nil(t, err)

	err = InjectBootFiles(testImage, []string{filepath.Join("testdata", "foo")})
	require.Nil(t, err)
}
