package server

import (
	"embed"
	"io/fs"
)

//go:embed all:web
var embeddedAssets embed.FS

// staticAssets provides the embedded web assets as an fs.FS
var staticAssets fs.FS

func init() {
	var err error
	staticAssets, err = fs.Sub(embeddedAssets, "web")
	if err != nil {
		panic("failed to initialize static assets: " + err.Error())
	}
}
