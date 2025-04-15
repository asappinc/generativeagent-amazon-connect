package quickstart

import (
	_ "embed"
	"io"
	"text/template"

	"github.com/asappinc/generativeagent-amazon-connect/pkg/config"
)

//go:embed attributesToInputVariables.tmpl
var attributesToInputVariablesTemplate string

//go:embed ssmlConversions.tmpl
var ssmlConversionsTemplate string

func writeAttributesToInputVariablesFile(outputFile io.Writer, attributesToInputVariables map[string]string) error {

	var fns = template.FuncMap{
		"plus1": func(x int) int {
			return x + 1
		},
	}

	tmpl, err := template.New("templates").Funcs(fns).Parse(attributesToInputVariablesTemplate)
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

func writeSSMLConversionsFile(outputFile io.Writer, ssmlConversions []config.SSMLConversion) error {

	tmpl, err := template.New("templates").Parse(ssmlConversionsTemplate)
	if err != nil {
		return err
	}

	// Execute the template with the data
	err = tmpl.Execute(outputFile, ssmlConversions)
	if err != nil {
		return err
	}

	return nil
}
