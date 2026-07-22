package template

import (
	"embed"
	"io/fs"
	"os"
	"path"
	"path/filepath"
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

// remove path elements preceeding and including 'dist'
func mungeDistPath(target string) (string, error) {
	found := false
	elements := []string{}
	for _, element := range strings.Split(filepath.Clean(target), string(filepath.Separator)) {
		if found {
			elements = append(elements, element)
		}
		if element == "dist" {
			found = true
		}
	}
	if !found {
		return "", Fatalf("missing 'dist' element: %s", target)
	}
	if len(elements) == 0 {
		return "", Fatalf("unexpected dist path: %s", target)
	}
	return filepath.Join(elements...), nil
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
		url, target, ok := strings.Cut(line, " ")
		if !ok {
			return nil, Fatalf("parse failed: %s\n", line)
		}
		target, err := mungeDistPath(target)
		if err != nil {
			return nil, err
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
	for i := range osList {
		osList[i] = strings.ToLower(osList[i])
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
