package iconassets

import (
	"embed"
	"io/fs"
)

// The public demo serves only its pinned service logos, without acquiring the full library.
//
//go:embed selfhst/images/c8aad5cc571f2c234faf6a94f77e30322b2ce9479ce1fc437ca8ded2eb3b553c.png
//go:embed selfhst/images/5a20b7b672da6436fed5ae84a2baae624248b47c37bb635b1cdc4ea4c0dc7e3b.png
//go:embed selfhst/images/dc37c74926336448b0404e232e7f4e7c4dc40174493307c8454ce23f59c49a6b.png
//go:embed selfhst/images/e3b41fe8fe75233f06b081ffb92cc811034c81e66bb00cd8aed15f0ce582fc38.png
//go:embed selfhst/images/f21a007521cec4e72bb083b89778e91bf9e976e8a0900a0b6e93797faf5b1cda.png
//go:embed selfhst/images/c885789b4d0bb39f7662a053282e02682e1a2bb0142c2652d12ff28c6803811f.png
//go:embed selfhst/images/765f50d176e23897074b61ea2a1857c19c023041cf91ffca447dbb8a868c8ef8.png
//go:embed selfhst/images/9a71e87618936e0d4512a8d3151f765c4e3cbe0a38011499153934f685ebb7f4.png
//go:embed selfhst/images/6484f861c0b268e50d068878c79eca61befc7df67442d8efc98977050b0514ed.png
//go:embed selfhst/images/dc09c72dc5bf573142dcc5c0b054271753cdf65be5700f77ae382cfa8b44f6c3.png
//go:embed selfhst/images/f30cbbfd653a34d2586dac8e1c94bc89a85a3fe1b932b326de8cd2e73b1d41c0.png
//go:embed selfhst/images/82c6eb2979db4824b3ab704d504dcd5d34e67f263888a8ba9c3d07b38f88e44b.png
var demoImages embed.FS

// DemoFiles contains the demo service images at their catalog paths.
func DemoFiles() fs.FS {
	files, err := fs.Sub(demoImages, "selfhst")
	if err != nil {
		panic(err)
	}
	return files
}
