package prometheus

import (
	"context"
	"net/http"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig"
	prom "github.com/prometheus/client_golang/api/prometheus"
	"github.com/tcolgate/hugot"
	"github.com/tcolgate/hugot-handlers/prometheus/am"
	"github.com/tcolgate/hugot/bot"
	"github.com/tcolgate/hugot/handlers/command"
)

func init() {
}

type promH struct {
	command.Commander
	cs    command.Set
	wh    hugot.WebHookHandler
	hmux  *http.ServeMux
	amURL string

	client   *prom.Client
	amclient am.Client
	tmpls    *template.Template
}

var defTmpls = map[string]string{
	"channel":    `alerts`,
	"color":      `{{ if eq .Status "firing" }}#ff0000{{ else }}#00ff00{{ end }}`,
	"title":      `[{{ .Status | upper }}{{ if eq .Status "firing" }}:{{ .Alerts.Firing | len }}{{ end }}] {{ .GroupLabels.SortedPairs.Values | join " " }} {{ if gt (len .CommonLabels) (len .GroupLabels) }}({{ with .CommonLabels.Remove .GroupLabels.Names }}{{ .Values | join " " }}{{ end }}){{ end }}`,
	"title_link": `{{ .ExternalURL }}/#/alerts?receiver={{ .Receiver }}`,
	"pretext":    ``,
	"text":       ``,
	"fallback":   `[{{ .Status | upper }}{{ if eq .Status "firing" }}:{{ .Alerts.Firing | len }}{{ end }}] {{ .GroupLabels.SortedPairs.Values | join " " }} {{ if gt (len .CommonLabels) (len .GroupLabels) }}({{ with .CommonLabels.Remove .GroupLabels.Names }}{{ .Values | join " " }}{{ end }}){{ end }}`,
}

// TemplateFuncs
var TemplateFuncs template.FuncMap

func init() {
	TemplateFuncs = template.FuncMap{}

	for k, v := range sprig.TxtFuncMap() {
		TemplateFuncs[k] = v
	}
	for k, v := range (template.FuncMap{
		"toUpper": strings.ToUpper,
		"toLower": strings.ToLower,
		"title":   strings.Title,
		// join is equal to strings.Join but inverts the argument order
		// for easier pipelining in templates.
		"join": func(sep string, s []string) string {
			return strings.Join(s, sep)
		},
	}) {
		TemplateFuncs[k] = v
	}
}

// New prometheus handler, returns a command and a webhook handler
func New(c *prom.Client, amURL string, tmpls *template.Template) *promH {
	tmpls = defaultTmpls(tmpls)
	amc, _ := am.New(am.Config{Address: amURL})

	h := &promH{
		nil,
		nil,
		nil,
		http.NewServeMux(),
		amURL,
		c,
		amc,
		tmpls}

	h.cs = command.NewSet(
		command.New("alerts", "list alerts", h.alertsCmd),
		command.New("silences", "list silences", h.silencesCmd),
		command.New("graph", "graph a query", h.graphCmd),
	)

	h.Commander = command.New("prometheus", "manage the prometheus monitoring tool", h.Command)

	h.hmux.HandleFunc("/", http.NotFound)
	h.hmux.HandleFunc("/alerts", h.alertsHook)
	h.hmux.HandleFunc("/alerts/", h.alertsHook)
	h.hmux.HandleFunc("/graph", h.graphHook)
	h.hmux.HandleFunc("/graph/", h.graphHook)

	h.wh = hugot.NewWebHookHandler("prometheus", "", h.webHook)

	return h
}

func (h *promH) Command(ctx context.Context, w hugot.ResponseWriter, m *command.Message) error {
	if err := m.Parse(); err != nil {
		return err
	}

	return h.cs.Command(ctx, w, m)
}

func defaultTmpls(tmpls *template.Template) *template.Template {
	if tmpls == nil {
		tmpls = template.New("defaultTmpls").Funcs(TemplateFuncs)
	}

	for tn := range defTmpls {
		if tmpls.Lookup(tn) == nil {
			template.Must(tmpls.New(tn).Parse(defTmpls[tn]))
		}
	}
	return tmpls
}

func Register(c *prom.Client, amURL string, tmpls *template.Template) {
	h := New(c, amURL, tmpls)
	bot.Command(h)
	bot.HandleHTTP(h.wh)
}
