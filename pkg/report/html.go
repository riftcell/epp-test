package report

import (
	_ "embed"
	"fmt"
	"html/template"
	"io"
	"time"

	"github.com/riftcell/epp-test/pkg/runner"
)

//go:embed templates/report.html.tmpl
var htmlTemplate string

var tmpl = template.Must(template.New("report").Funcs(template.FuncMap{
	"formatDuration": func(d time.Duration) string {
		return fmt.Sprintf("%.3fs", d.Seconds())
	},
	"add": func(a, b int) int { return a + b },
	"sub": func(a, b int) int { return a - b },
}).Parse(htmlTemplate))

// writeHTML renders the HTML report template to w using results.
//
// html/template provides context-aware escaping so scenario names containing
// angle brackets (e.g., "<domain:check>") are automatically escaped to
// "&lt;domain:check&gt;" — preventing broken HTML and XSS (RESEARCH Pitfall 5).
func writeHTML(w io.Writer, results []runner.ScenarioResult) error {
	data := struct {
		Results   []runner.ScenarioResult
		Generated time.Time
	}{
		Results:   results,
		Generated: time.Now(),
	}
	if err := tmpl.Execute(w, data); err != nil {
		return fmt.Errorf("report: render html template: %w", err)
	}
	return nil
}
