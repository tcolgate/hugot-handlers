package prometheus

import (
	"net/http"
	"text/template"

	"github.com/Masterminds/sprig"
	am "github.com/prometheus/client_golang/api/alertmanager"
	prom "github.com/prometheus/client_golang/api/prometheus"
	"github.com/tcolgate/hugot"
)

func init() {
}

type promH struct {
	client   *prom.Client
	amclient am.Client
	tmpls    *template.Template

	hugot.CommandWithSubsHandler
	hugot.WebHookHandler

	hmux *http.ServeMux
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

	h := &promH{c, amc, tmpls, nil, nil, http.NewServeMux()}

	cs := hugot.NewCommandSet()
	cs.AddCommandHandler(hugot.NewCommandHandler("alerts", "list alerts", hugot.CommandFunc(h.alertsCmd), nil))
	cs.AddCommandHandler(hugot.NewCommandHandler("silences", "list silences", hugot.CommandFunc(h.silencesCmd), nil))
	cs.AddCommandHandler(hugot.NewCommandHandler("graph", "graph a query", hugot.CommandFunc(h.graphCmd), nil))
	cs.AddCommandHandler(hugot.NewCommandHandler("explain", "explains the meaning of an alert rule name", h.explainCmd, nil))

	h.CommandWithSubsHandler = hugot.NewCommandHandler("prometheus", "manage the prometheus monitoring tool", nil, cs)

	h.hmux.HandleFunc("/", http.NotFound)
	h.hmux.HandleFunc("/alerts", h.alertsHook)
	h.hmux.HandleFunc("/alerts/", h.alertsHook)
	h.hmux.HandleFunc("/graph", h.graphHook)
	h.hmux.HandleFunc("/graph/", h.graphHook)

	h.WebHookHandler = hugot.NewWebHookHandler("prometheus", "", h.webHook)

	return h
}

func (p *promH) Describe() (string, string) {
	return p.CommandWithSubsHandler.Describe()
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
