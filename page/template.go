package page

import (
	"errors"
	"html/template"
	"net/http"
	"path/filepath"
)

type TemplateMap map[string]*template.Template

var (
	templateFiles = [][]string{
		{TnameError, TnameLayout, TnameCommon},
		{TnameNotFound, TnameLayout, TnameCommon},
		{TnameClasslist, TnameLayout, TnameCommon},
		{TnameBbsIndex, TnameLayout, TnameCommon},
		{TnameBbsArticle, TnameLayout, TnameCommon},
		{TnameAskOver18, TnameLayout, TnameCommon},
		{TnameManIndex, TnameLayout, TnameCommon},
		{TnameManArticle, TnameLayout, TnameCommon},
		{TnameCaptcha, TnameLayout, TnameCommon},
	}

	tmpl TemplateMap
)

func loadTemplates(dir string, filenames [][]string, funcMap template.FuncMap) (TemplateMap, error) {
	new_tmpl := make(TemplateMap)

	for _, fns := range filenames {
		t := template.New("")
		t.Funcs(funcMap)

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

func LoadTemplates(dir string, funcMap template.FuncMap) error {
	new_tmpl, err := loadTemplates(dir, templateFiles, funcMap)
	if err != nil {
		return err
	}
	tmpl = new_tmpl
	return nil
}

type NeedToWriteHeaders interface {
	WriteHeaders(w http.ResponseWriter) error
}

func ExecuteTemplate(w http.ResponseWriter, name string, arg interface{}) error {
	if p, ok := arg.(NeedToWriteHeaders); ok {
		if err := p.WriteHeaders(w); err != nil {
			return err
		}
	}
	if name == "" {
		return nil
	}
	return tmpl[name].Execute(w, arg)
}

func ExecutePage(w http.ResponseWriter, p Page) error {
	return ExecuteTemplate(w, p.TemplateName(), p)
}
