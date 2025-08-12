package server

import (
	"io/fs"
)

func downloadToFS(filesystem fs.FS, url, path string) error {
	return Fatalf("downloadToFS(<fs>, %s, %s)\n", url, path)
}
