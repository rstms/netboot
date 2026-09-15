package template

import (
	"github.com/stretchr/testify/require"
	"log"
	"os"
	"path/filepath"
	"testing"
)

func TestTemplateDistInitFiles(t *testing.T) {
	files, err := DistInitFiles()
	require.Nil(t, err)
	require.NotZero(t, len(files))
	for i, file := range files {
		log.Printf("%d\n%+v\n", i, file)
	}
}

func distDir(t *testing.T) string {
	cache, err := os.UserCacheDir()
	require.Nil(t, err)
	return filepath.Join(cache, "netboot", "dist")
}

func TestTemplateDistNames(t *testing.T) {
	names, err := DistNames(distDir(t))
	require.Nil(t, err)
	require.Len(t, names, 5)
	require.Contains(t, names, "debian")
	require.Contains(t, names, "openbsd")
	require.Contains(t, names, "alpine")
	require.Contains(t, names, "windows")
	log.Printf("DistNames: %v\n", names)
}

func TestTemplateDistVersions(t *testing.T) {
	names, err := DistNames(distDir(t))
	require.Nil(t, err)
	for _, name := range names {
		versions, err := DistVersions(distDir(t), name)
		require.Nil(t, err)
		switch name {
		case "debian":
			require.Equal(t, []string{"bookworm", "trixie"}, versions)
		case "openbsd":
			require.Equal(t, []string{"7.7", "7.8", "7.9"}, versions)
		case "alpine":
			require.Equal(t, []string{"3.22.1", "3.23.4"}, versions)
		case "windows":
			require.Equal(t, []string{"11"}, versions)
		default:
			require.Truef(t, false, "unexpected OS name: %s", name)
		}
		log.Printf("os=%s versions=%v\n", name, versions)
	}
}
