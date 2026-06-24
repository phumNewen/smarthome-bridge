package engine

import (
	"bytes"
	"fmt"
	"html"
	"sync"
	"text/template"
	"time"
)

// TemplateData is the context passed to message templates.
type TemplateData struct {
	RuleName    string
	Description string
	Device      string
	Topic       string
	Fields      map[string]any
	Time        time.Time
	Payload     string // raw JSON payload as string
}

// TemplateCache stores pre-parsed templates keyed by rule name.
type TemplateCache struct {
	mu   sync.Mutex
	tmpl map[string]*template.Template
}

// NewTemplateCache creates a new template cache.
func NewTemplateCache() *TemplateCache {
	return &TemplateCache{
		tmpl: make(map[string]*template.Template),
	}
}

// GetOrParse returns the parsed template for a rule, caching it on first access.
func (tc *TemplateCache) GetOrParse(ruleName, templateStr string) (*template.Template, error) {
	// Fast path: already cached.
	tc.mu.Lock()
	tmpl, ok := tc.tmpl[ruleName]
	tc.mu.Unlock()
	if ok {
		return tmpl, nil
	}

	// Parse and cache.
	funcMap := template.FuncMap{
		"escapeHTML": html.EscapeString,
		"formatTime": func(t time.Time, layout string) string {
			return t.Format(layout)
		},
	}

	parsed, err := template.New(ruleName).Funcs(funcMap).Parse(templateStr)
	if err != nil {
		return nil, fmt.Errorf("parse template for rule %q: %w", ruleName, err)
	}

	tc.mu.Lock()
	tc.tmpl[ruleName] = parsed
	tc.mu.Unlock()

	return parsed, nil
}

// Render executes a parsed template with the given data and returns the result.
func Render(tmpl *template.Template, data TemplateData) (string, error) {
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("render template: %w", err)
	}
	return buf.String(), nil
}

// Purge removes all cached templates (useful for testing).
func (tc *TemplateCache) Purge() {
	tc.mu.Lock()
	tc.tmpl = make(map[string]*template.Template)
	tc.mu.Unlock()
}
