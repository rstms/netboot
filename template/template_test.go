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
