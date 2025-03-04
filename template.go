package main

import (
	"fmt"
	"net/http"
	"path/filepath"
	"text/template"
)

func NewTemplateCache() (map[string]*template.Template, error) {
	tmplCache := make(map[string]*template.Template)

	pages, err := filepath.Glob("./templates/pages/*.tmpl")
	if err != nil {
		return nil, err
	}

	for _, page := range pages {
		name := filepath.Base(page)
		patterns := []string{
			"./templates/base.html.tmpl",
			page,
		}

		ts, err := template.New(name).ParseFiles(patterns...)
		if err != nil {
			return nil, err
		}

		tmplCache[name] = ts
	}

	return tmplCache, nil
}

func render(w http.ResponseWriter, tmplName string) error {
	tmpl, ok := tc[tmplName]
	if !ok {
		return fmt.Errorf("template %q not in cache", tmplName)
	}

	return tmpl.Execute(w, nil)
}
