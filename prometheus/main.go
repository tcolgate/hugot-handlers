package prometheus

import (
	"net/http"

	prom "github.com/prometheus/client_golang/api/prometheus"
	"github.com/tcolgate/hugot"
)

func init() {
}

type promH struct {
	client    *prom.Client
	amURL     string
	alertChan string

	hugot.CommandHandler
	hugot.WebHookHandler

	hmux *http.ServeMux
}

// New prometheus handler, returns a command and a webhook handler
func New(c *prom.Client, amurl, achan string) *promH {
	h := &promH{c, amurl, achan, nil, nil, http.NewServeMux()}

	cs := hugot.NewCommandSet()
	cs.AddCommandHandler(hugot.NewCommandHandler("alerts", "alertsCmd", hugot.CommandFunc(h.alertCmd), nil))
	cs.AddCommandHandler(hugot.NewCommandHandler("graph", "graph a query", hugot.CommandFunc(h.graphCmd), nil))
	cs.AddCommandHandler(hugot.NewCommandHandler("explain", "explains the meaning of an alert rule name", h.explainCmd, nil))

	h.CommandHandler = hugot.NewCommandHandler("prometheus", "manage the prometheus monitoring tool", nil, cs)

	h.hmux.HandleFunc("/", http.NotFound)
	h.hmux.HandleFunc("/alerts", h.alertsHook)
	h.hmux.HandleFunc("/alerts/", h.alertsHook)
	h.hmux.HandleFunc("/graph", h.graphHook)
	h.hmux.HandleFunc("/graph/", h.graphHook)

	h.WebHookHandler = hugot.NewWebHookHandler("prometheus", "", h.webHook)

	return h
}

func (p *promH) Describe() (string, string) {
	return p.CommandHandler.Describe()
}
