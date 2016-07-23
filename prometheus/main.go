package prometheus

import (
	"net/http"

	am "github.com/tcolgate/client_golang/api/alertmanager"
	prom "github.com/tcolgate/client_golang/api/prometheus"
	"github.com/tcolgate/hugot"
)

func init() {
}

type promH struct {
	client    *prom.Client
	amclient  am.Client
	alertChan string

	hugot.CommandHandler
	hugot.WebHookHandler

	hmux *http.ServeMux
}

// New prometheus handler, returns a command and a webhook handler
func New(c *prom.Client, amc am.Client, achan string) *promH {
	h := &promH{c, amc, achan, nil, nil, http.NewServeMux()}

	cs := hugot.NewCommandSet()
	cs.AddCommandHandler(hugot.NewCommandHandler("alerts", "list alerts", hugot.CommandFunc(h.alertsCmd), nil))
	cs.AddCommandHandler(hugot.NewCommandHandler("silences", "list silences", hugot.CommandFunc(h.silencesCmd), nil))
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
