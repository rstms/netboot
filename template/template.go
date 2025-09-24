package template

import (
	"embed"
	"io/fs"
	"path"
	"slices"
	"strings"
)

//go:embed ipxe
var Ipxe embed.FS

//go:embed mkboot
var Mkboot embed.FS

//go:embed dist
var Dist embed.FS

func DistNames() ([]string, error) {
	paths, err := fs.Glob(Dist, "dist/*")
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

func DistVersions(distName string) ([]string, error) {
	distPath := path.Join("dist", distName)
	versionPaths, err := fs.Glob(Dist, path.Join(distPath, "*"))
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
