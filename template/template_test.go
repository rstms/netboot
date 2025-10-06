package template

import (
	"github.com/stretchr/testify/require"
	"io/fs"
	"log"
	"testing"
)

func TestTemplateDist(t *testing.T) {
	err := fs.WalkDir(Dist, "dist", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		log.Printf("%s, %v\n", path, entry)
		return nil
	})
	require.Nil(t, err)
}

func TestTemplateDistnames(t *testing.T) {
	names, err := DistNames()
	require.Nil(t, err)
	require.Len(t, names, 4)
	require.Contains(t, names, "debian")
	require.Contains(t, names, "openbsd")
	require.Contains(t, names, "alpine")
	require.Contains(t, names, "windows")
	log.Printf("DistNames: %v\n", names)
}

func TestTemplateDistVersions(t *testing.T) {
	names, err := DistNames()
	require.Nil(t, err)
	for _, name := range names {
		versions, err := DistVersions(name)
		require.Nil(t, err)
		switch name {
		case "debian":
			require.Equal(t, []string{"bookworm", "trixie"}, versions)
		case "openbsd":
			require.Equal(t, []string{"7.5", "7.6", "7.7"}, versions)
		case "alpine":
			require.Equal(t, []string{"3.22.1"}, versions)
		case "windows":
			require.Equal(t, []string{"11"}, versions)
		default:
			require.Truef(t, false, "unexpected OS name: %s", name)
		}
		log.Printf("os=%s versions=%v\n", name, versions)
	}
}
