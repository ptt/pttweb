package main

import (
	"errors"
	"path/filepath"
	"text/template"
)

type TemplateMap map[string]*template.Template

var (
	templateFiles = [][]string{
		{"notfound.html", "layout.html", "common.html"},
		{"classlist.html", "layout.html", "common.html"},
		{"bbsindex.html", "layout.html", "common.html"},
		{"bbsarticle.html", "layout.html", "common.html"},
	}
)

func loadTemplates(dir string, filenames [][]string) (TemplateMap, error) {
	new_tmpl := make(TemplateMap)

	for _, fns := range filenames {
		t := template.New("")
		t.Funcs(templateFuncMap())

		paths := make([]string, len(fns), len(fns))
		for i, fn := range fns {
			paths[i] = filepath.Join(dir, fn)
		}

		if _, err := t.ParseFiles(paths...); err != nil {
			return nil, err
		}

		name := fns[0]
		root := t.Lookup("ROOT")
		if root == nil {
			return nil, errors.New("No ROOT template defined")
		}
		new_tmpl[name] = root
	}

	return new_tmpl, nil
}
