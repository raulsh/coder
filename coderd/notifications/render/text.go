package render

import (
	"strings"
	text "text/template"

	"golang.org/x/xerrors"

	"github.com/coder/coder/v2/coderd/notifications/types"
)

const Text = "text"

type TextRenderer struct{}

func (t *TextRenderer) Name() string {
	return Text
}

// Render parses the given template as a text template and renders it using Go's text/template package.
// TODO: consider performance impact
func (t *TextRenderer) Render(template string, input types.Labels) (string, error) {
	tmpl, err := text.New("text").Parse(template)
	if err != nil {
		return "", xerrors.Errorf("template parse: %w", err)
	}

	var out strings.Builder
	if err = tmpl.Execute(&out, input); err != nil {
		return "", xerrors.Errorf("template execute: %w", err)
	}

	return out.String(), nil
}
