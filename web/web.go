package web

import (
	"embed"
	"io/fs"
	"net/http"

	"github.com/suserc0de/rhui"
)

//go:embed views/*.rhui views/**/*.rhui
var viewsFS embed.FS

//go:embed static/*
var staticFS embed.FS

// GetRHUIEngine membuat instance RHUI template engine yang membaca langsung dari memory binary (embed.FS)
func GetRHUIEngine() (*rhui.Engine, error) {
	return rhui.New(
		rhui.WithFS(viewsFS),
		rhui.WithDir("views"),
		rhui.WithExtension(".rhui"),
		rhui.WithMaxRenderDepth(64), // Security & Recursion depth limit hardening
	)
}

// GetStaticFS mengembalikan http.FileSystem untuk melayani file CSS, JS, dan Fonts dari memory binary
func GetStaticFS() (http.FileSystem, error) {
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		return nil, err
	}
	return http.FS(sub), nil
}
