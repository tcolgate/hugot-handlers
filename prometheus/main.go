package prometheus

import (
	"net/http"
	"text/template"

	"github.com/Masterminds/sprig"
	am "github.com/prometheus/client_golang/api/alertmanager"
	prom "github.com/prometheus/client_golang/api/prometheus"
	"github.com/tcolgate/hugot"
	"github.com/tcolgate/hugot/bot"
	"github.com/tcolgate/hugot/handlers/command"
)

func init() {
}

type promH struct {
	command.Commander
	cs   command.Set
	wh   hugot.WebHookHandler
	hmux *http.ServeMux

	client   *prom.Client
	amclient am.Client
	tmpls    *template.Template
}

var defTmpls = map[string]string{
	"channel":    `x9b9ybtztjge3p745sadxxc5ih`,
	"color":      `{{ if eq .Status "firing" }}#ff0000{{ else }}#00ff00{{ end }}`,
	"title":      `[{{ .Status | upper }}{{ if eq .Status "firing" }}:{{ .Alerts.Firing | len }}{{ end }}] {{ .GroupLabels.SortedPairs.Values | join " " }} {{ if gt (len .CommonLabels) (len .GroupLabels) }}({{ with .CommonLabels.Remove .GroupLabels.Names }}{{ .Values | join " " }}{{ end }}){{ end }}`,
	"title_link": `{{ .ExternalURL }}/#/alerts?receiver={{ .Receiver }}`,
	"pretext":    ``,
	"text":       ``,
	"fallback":   `[{{ .Status | upper }}{{ if eq .Status "firing" }}:{{ .Alerts.Firing | len }}{{ end }}] {{ .GroupLabels.SortedPairs.Values | join " " }} {{ if gt (len .CommonLabels) (len .GroupLabels) }}({{ with .CommonLabels.Remove .GroupLabels.Names }}{{ .Values | join " " }}{{ end }}){{ end }}`,
}

// New prometheus handler, returns a command and a webhook handler
func New(c *prom.Client, amc am.Client, tmpls *template.Template) *promH {
	tmpls = defaultTmpls(tmpls)

	h := &promH{nil, nil, nil, http.NewServeMux(), c, amc, tmpls}

	h.cs = command.NewSet(
		command.New("alerts", "list alerts", h.alertsCmd),
		command.New("silences", "list silences", h.silencesCmd),
		command.New("graph", "graph a query", h.graphCmd),
	)

	h.Commander = command.New("prometheus", "manage the prometheus monitoring tool", h.cs.Command)

	h.hmux.HandleFunc("/", http.NotFound)
	h.hmux.HandleFunc("/alerts", h.alertsHook)
	h.hmux.HandleFunc("/alerts/", h.alertsHook)
	h.hmux.HandleFunc("/graph", h.graphHook)
	h.hmux.HandleFunc("/graph/", h.graphHook)

	h.wh = hugot.NewWebHookHandler("prometheus", "", h.webHook)

	return h
}

func defaultTmpls(tmpls *template.Template) *template.Template {
	if tmpls == nil {
		tmpls = template.New("defaultTmpls").Funcs(sprig.TxtFuncMap())
	}

	for tn := range defTmpls {
		if tmpls.Lookup(tn) == nil {
			template.Must(tmpls.New(tn).Parse(defTmpls[tn]))
		}
	}
	return tmpls
}

func Register(c *prom.Client, amc am.Client, tmpls *template.Template) {
	h := New(c, amc, tmpls)
	bot.Command(h)
	bot.HandleHTTP(h.wh)
}
