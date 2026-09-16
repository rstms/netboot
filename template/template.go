package template

import (
	"embed"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

//go:embed ipxe
var Ipxe embed.FS

//go:embed mkboot
var Mkboot embed.FS

func distDir(distRoot, distName string) (string, error) {
	name := strings.ToLower(distName)
	var dir string
	if name == "openbsd" {
		dir = filepath.Join(distRoot, "pub", "OpenBSD")
	} else {
		dir = filepath.Join(distRoot, name)
	}
	if !IsDir(dir) {
		return "", Fatalf("uknown OS: %s", distName)
	}
	return dir, nil
}

func DistNames(distRoot string) ([]string, error) {
	paths, err := fs.Glob(os.DirFS(distRoot), "*")
	if err != nil {
		return nil, Fatal(err)
	}
	slices.Sort(paths)
	names := []string{}
	for _, path := range paths {
		_, name := filepath.Split(path)
		if name == "pub" {
			name = "openbsd"
		}
		names = append(names, name)
	}
	return names, nil
}

func DistVersions(distRoot, distName string) ([]string, error) {
	dir, err := distDir(distRoot, distName)
	if err != nil {
		return nil, Fatal(err)
	}
	paths, err := fs.Glob(os.DirFS(dir), "*")
	if err != nil {
		return nil, Fatal(err)
	}
	if len(paths) == 0 {
		return nil, Fatalf("no %s versions found", distName)
	}
	slices.Sort(paths)
	versions := []string{}
	for _, path := range paths {
		_, version := filepath.Split(path)
		versions = append(versions, version)
	}
	return versions, nil
}

func DistArchs(distRoot, distName, distVersion string) ([]string, error) {
	dir, err := distDir(distRoot, distName)
	if err != nil {
		return nil, Fatal(err)
	}
	dir = filepath.Join(dir, distVersion)
	if !IsDir(dir) {
		return nil, Fatalf("%s version %s not found", distName, distVersion)
	}
	paths, err := fs.Glob(os.DirFS(dir), "*")
	if err != nil {
		return nil, Fatal(err)
	}
	if len(paths) == 0 {
		return nil, Fatalf("no architectures found for %s %s", distName, distVersion)
	}
	slices.Sort(paths)
	archs := []string{}
	for _, path := range paths {
		_, arch := filepath.Split(path)
		archs = append(archs, arch)
	}
	return archs, nil
}

func DistPath(distRoot, distName, distVersion, distArch string) (string, error) {
	dir, err := distDir(distRoot, distName)
	if err != nil {
		return "", Fatal(err)
	}
	archs, err := DistArchs(distRoot, distName, distVersion)
	if err != nil {
		return "", Fatal(err)
	}
	if slices.Contains(archs, distArch) {
		dir = filepath.Join(dir, distVersion, distArch)
		if IsDir(dir) {
			return dir, nil
		}
	}
	return "", Fatalf("%s %s architecture %s not found", distName, distVersion, distArch)
}
