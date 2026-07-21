package template

import (
	"embed"
	"io/fs"
	"os"
	"path"
	"slices"
	"strings"
)

//go:embed ipxe
var Ipxe embed.FS

//go:embed mkboot
var Mkboot embed.FS

//go:embed dist_init_files
var distReadme []byte

type DistFile struct {
	URL      string
	Pathname string
}

func DistInitFiles() ([]DistFile, error) {
	lines := strings.Split(string(distReadme), "\n")
	files := []DistFile{}
	for _, line := range lines {
		if strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		url, filename, ok := strings.Cut(line, " ")
		if !ok {
			return nil, Fatalf("failed parsing dist/README.md: %s\n", line)
		}
		_, target, ok := strings.Cut(filename, "/")
		if !ok {
			return nil, Fatalf("failed parsing dist filename: %s\n", filename)
		}
		files = append(files, DistFile{URL: url, Pathname: target})
	}
	return files, nil
}

func DistNames(distDir string) ([]string, error) {
	paths, err := fs.Glob(os.DirFS(distDir), "*")
	if err != nil {
		return []string{}, Fatal(err)
	}
	slices.Sort(paths)
	osList := []string{}
	for _, path := range paths {
		fields := strings.Split(path, "/")
		if len(fields) > 0 {
			osList = append(osList, fields[len(fields)-1])
		}
	}
	return osList, nil
}

func DistVersions(distDir, distName string) ([]string, error) {
	versionPaths, err := fs.Glob(os.DirFS(distDir), path.Join(distName, "*"))
	if err != nil {
		return []string{}, Fatal(err)
	}
	if len(versionPaths) == 0 {
		return []string{}, Fatalf("unknown OS: %s", distName)
	}
	slices.Sort(versionPaths)
	versionList := []string{}
	for _, path := range versionPaths {
		fields := strings.Split(path, "/")
		if len(fields) > 0 {
			versionList = append(versionList, fields[len(fields)-1])
		}
	}
	return versionList, nil
}
