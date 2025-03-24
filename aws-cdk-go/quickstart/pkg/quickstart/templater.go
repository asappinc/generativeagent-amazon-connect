package quickstart

import (
	_ "embed"
	"io"
	"text/template"
)

//go:embed attributesToInputVariables.tmpl
var tmplString string

func writeAttributesToInputVariablesFile(outputFile io.Writer, attributesToInputVariables map[string]string) error {

	var fns = template.FuncMap{
		"plus1": func(x int) int {
			return x + 1
		},
	}

	tmpl, err := template.New("templates").Funcs(fns).Parse(tmplString)
	if err != nil {
		return err
	}

	// Execute the template with the data
	err = tmpl.Execute(outputFile, attributesToInputVariables)
	if err != nil {
		return err
	}

	return nil
}
