package web

import (
	"embed"
	"html/template"
	"io/fs"
	"net/http"
)

//go:embed views/*.tmpl
var viewsFS embed.FS

//go:embed static/*
var staticFS embed.FS

// GetTemplates mem-parsing seluruh file template .tmpl yang disematkan di dalam binary
func GetTemplates() (*template.Template, error) {
	return template.ParseFS(viewsFS, "views/*.tmpl")
}

// GetStaticFS mengembalikan http.FileSystem untuk melayani file CSS, JS, dan Fonts dari memory binary
func GetStaticFS() (http.FileSystem, error) {
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		return nil, err
	}
	return http.FS(sub), nil
}
