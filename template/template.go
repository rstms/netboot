package template

import (
	"embed"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

//go:embed ipxe
var Ipxe embed.FS

//go:embed mkboot
var Mkboot embed.FS

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
	files := []DistFile{}
	for _, line := range DistFiles {
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

func DistDir(distRoot, distName string) (string, error) {
	name, err := NormalizeDistName(distRoot, distName)
	if err != nil {
		return "", Fatal(err)
	}
	dir := filepath.Join(distRoot, name)
	if !IsDir(dir) {
		return "", Fatalf("uknown OS: %s", distName)
	}
	return dir, nil
}

func DistNames(distRoot string) ([]string, error) {
	paths, err := fs.Glob(os.DirFS(distRoot), "*")
	if err != nil {
		return []string{}, Fatal(err)
	}
	log.Printf("paths=%+v\n", paths)
	slices.Sort(paths)
	names := []string{}
	for _, path := range paths {
		fields := strings.Split(path, "/")
		if len(fields) > 0 {
			name := fields[len(fields)-1]
			names = append(names, name)
		}
	}
	return names, nil
}

func DistVersions(distRoot, distName string) ([]string, error) {
	//versionPaths, err := fs.Glob(os.DirFS(distDir), path.Join(distName, "*"))
	distDir, err := DistDir(distRoot, distName)
	if err != nil {
		return nil, Fatal(err)
	}
	paths, err := fs.Glob(os.DirFS(distDir), "*")
	if err != nil {
		return nil, Fatal(err)
	}
	log.Printf("paths: %+v\n", paths)
	if len(paths) == 0 {
		return nil, Fatalf("no versions found for OS: %s", distName)
	}
	slices.Sort(paths)
	versions := []string{}
	for _, path := range paths {
		fields := strings.Split(path, "/")
		if len(fields) > 0 {
			versions = append(versions, fields[len(fields)-1])
		}
	}
	return versions, nil
}

func NormalizeDistName(distRoot, distName string) (string, error) {
	names, err := DistNames(distRoot)
	if err != nil {
		return "", Fatal(err)
	}
	for _, name := range names {
		if strings.ToLower(distName) == strings.ToLower(name) {
			return name, nil
		}
	}
	return "", Fatalf("unknown OS: %s", distName)
}
