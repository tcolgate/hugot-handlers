package prometheus

import (
	"net/http"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig"
	promC "github.com/prometheus/client_golang/api"
	"github.com/tcolgate/hugot"
	"github.com/tcolgate/hugot-handlers/prometheus/am"
	"github.com/tcolgate/hugot/bot"
	"github.com/tcolgate/hugot/handlers/command"
)

func init() {
}

type promH struct {
	*command.Handler
	wh   hugot.WebHookHandler
	hmux *http.ServeMux

	client   *promC.Client
	amclient am.Client
	tmpls    *template.Template
}

var defTmpls = map[string]string{
	"channel":     `alerts`,
	"color":       `{{ if eq .Status "firing" }}#ff0000{{ else }}#00ff00{{ end }}`,
	"title":       `[{{ .Status | upper }}{{ if eq .Status "firing" }}:{{ .Alerts.Firing | len }}{{ end }}] {{ .GroupLabels.SortedPairs.Values | join " " }} {{ if gt (len .CommonLabels) (len .GroupLabels) }}({{ with .CommonLabels.Remove .GroupLabels.Names }}{{ .Values | join " " }}{{ end }}){{ end }}`,
	"title_link":  `{{ .ExternalURL }}/#/alerts?receiver={{ .Receiver }}`,
	"image_url":   `{{$caQuery := .CommonAnnotations.image_query}}{{ if $caQuery }}http://localhost:8090/hugot/prometheus/graph/thing.png?e={{ now.Unix}}&q={{$caQuery | urlquery}}&s={{ with $start :=  now | date_modify "-15m" }}{{$start.Unix}}{{end}}{{end}}`,
	"text":        `{{$caRB := .CommonAnnotations.runbook_url}}{{$caDash := .CommonAnnotations.dashboard_url}}{{ range .Alerts.Firing }}{{ printf "%s" .Annotations.description }}{{if not $caDash}}{{ if .Annotations.dashboard_url }}{{printf " [:thermometer:](%s)"  .Annotations.dashboard_url }}{{end}}{{end}}{{if not $caRB}}{{ if .Annotations.runbook_url }}{{ printf "[:clipboard:](%s)" .Annotations.runbook_url }}{{end}}{{end}}{{ printf "\n"}}{{end}}{{ if eq .Status "firing" }} {{if $caRB }}[:clipboard:]({{ $caRB }}#{{ lower .GroupLabels.alertname }}){{ end }}{{ if $caDash }} [:thermometer:]({{ $caDash }}){{end}}{{end}}`,
	"fallback":    `[{{ .Status | upper }}{{ if eq .Status "firing" }}:{{ .Alerts.Firing | len }}{{ end }}] {{ .GroupLabels.SortedPairs.Values | join " " }} {{ if gt (len .CommonLabels) (len .GroupLabels) }}({{ with .CommonLabels.Remove .GroupLabels.Names }}{{ .Values | join " " }}{{ end }}){{ end }}`,
	"fields_json": ``,
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
func New(c *promC.Client, amc am.Client, tmpls *template.Template) *promH {
	tmpls = defaultTmpls(tmpls)

	h := &promH{nil, nil, http.NewServeMux(), c, amc, tmpls}

	h.Handler = command.NewFunc(func(root *command.Command) error {
		root.Use = "prometheus"
		root.Short = "manage prometheus"
		h.alertCmd(root)
		h.silenceCmd(root)
		h.graphCmd(root, true)

		return nil
	})

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
		tmpls = template.New("defaultTmpls").Funcs(TemplateFuncs)
	}

	for tn := range defTmpls {
		if tmpls.Lookup(tn) == nil {
			template.Must(tmpls.New(tn).Parse(defTmpls[tn]))
		}
	}
	return tmpls
}

func Register(c *promC.Client, amc am.Client, tmpls *template.Template) {
	h := New(c, amc, tmpls)
	bot.Command(h.Handler)
	bot.HandleHTTP(h.wh)
}
