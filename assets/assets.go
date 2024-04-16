package assets

import (
	"embed"
	"io/fs"
)

//go:embed templates/*
var templateFS embed.FS

//go:embed emails/*.tmpl
var emailFS embed.FS

var (
	TemplateFS fs.FS
	EmailFS    fs.FS
)

func init() {
	var err error

	TemplateFS, err = fs.Sub(templateFS, "templates")
	if err != nil {
		panic("failed to subtree template FS " + err.Error())
	}

	EmailFS, err = fs.Sub(emailFS, "emails")
	if err != nil {
		panic("failed to subtree template FS " + err.Error())
	}
}
