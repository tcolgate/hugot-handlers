package grafana

import (
	"net/http"

	"github.com/tcolgate/hugot"
)

func init() {
}

type grafH struct {
	c     *http.Client
	url   string
	token string

	hmux *http.ServeMux
	hugot.CommandWithSubsHandler
	hugot.WebHookHandler
}

// New prometheus handler, returns a command and a webhook handler
func New(c *http.Client, url, token string) *grafH {
	h := &grafH{c, url, token, http.NewServeMux(), nil, nil}

	cs := hugot.NewCommandSet()
	cs.AddCommandHandler(hugot.NewCommandHandler("graph", "graph a query", hugot.CommandFunc(h.graphCmd), nil))

	h.CommandWithSubsHandler = hugot.NewCommandHandler("grafana", "grafana integration", nil, cs)

	h.hmux.HandleFunc("/", http.NotFound)
	h.hmux.HandleFunc("/graph", h.graphHook)
	h.hmux.HandleFunc("/graph/", h.graphHook)

	h.WebHookHandler = hugot.NewWebHookHandler("grafana", "", h.webHook)

	return h
}

func (p *grafH) Describe() (string, string) {
	return p.CommandWithSubsHandler.Describe()
}
