package template

import (
	"embed"
)

//go:embed certs
var Certs embed.FS

//go:embed ipxe
var Ipxe embed.FS

//go:embed mkboot
var Mkboot embed.FS

//go:embed dist
var Dist embed.FS
