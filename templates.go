package main

import (
	"embed"
	"fmt"
	"html/template"
)

//go:embed templates/*.html
var templateFS embed.FS

type templates struct {
	index    *template.Template
	edit     *template.Template
	settings *template.Template
	logs     *template.Template
}

func loadTemplates() (*templates, error) {
	parse := func(page string) (*template.Template, error) {
		t, err := template.ParseFS(templateFS, "templates/base.html", "templates/"+page)
		if err != nil {
			return nil, fmt.Errorf("parsing %s: %w", page, err)
		}
		return t, nil
	}

	var (
		t   templates
		err error
	)
	if t.index, err = parse("index.html"); err != nil {
		return nil, err
	}
	if t.edit, err = parse("edit.html"); err != nil {
		return nil, err
	}
	if t.settings, err = parse("settings.html"); err != nil {
		return nil, err
	}
	if t.logs, err = parse("logs.html"); err != nil {
		return nil, err
	}
	return &t, nil
}
