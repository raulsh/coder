package render

import (
	html "html/template"
	"strings"

	"golang.org/x/xerrors"

	"github.com/coder/coder/v2/coderd/notifications/types"
)

const HTML = "html"

type HTMLRenderer struct{}

func (h *HTMLRenderer) Name() string {
	return HTML
}

// Render parses the given template as an HTML template and renders it using Go's html/template package.
// TODO: consider performance impact
func (h *HTMLRenderer) Render(template string, input types.Labels) (string, error) {
	tmpl, err := html.New("html").Parse(template)
	if err != nil {
		return "", xerrors.Errorf("template parse: %w", err)
	}

	var out strings.Builder
	if err = tmpl.Execute(&out, input); err != nil {
		return "", xerrors.Errorf("template execute: %w", err)
	}

	return out.String(), nil
}
