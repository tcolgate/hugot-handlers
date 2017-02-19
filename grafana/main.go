package grafana

import (
	"context"
	"net/http"

	"github.com/tcolgate/hugot"
	"github.com/tcolgate/hugot/bot"
	"github.com/tcolgate/hugot/handlers/command"
)

func init() {
}

type grafH struct {
	command.Commander
	cs command.Set

	c     *http.Client
	url   string
	token string

	hmux *http.ServeMux
	wh   hugot.WebHookHandler
}

// New prometheus handler, returns a command and a webhook handler
func New(c *http.Client, url, token string) *grafH {
	h := &grafH{nil, nil, c, url, token, http.NewServeMux(), nil}

	h.cs = command.NewSet()
	h.cs.Add(command.New("graph", "graph a query", h.graphCmd))

	h.Commander = command.New("grafana", "grafana integration", h.Command)

	h.hmux.HandleFunc("/", http.NotFound)
	h.hmux.HandleFunc("/graph", h.graphHook)
	h.hmux.HandleFunc("/graph/", h.graphHook)

	h.wh = hugot.NewWebHookHandler("grafana", "", h.webHook)

	return h
}

func (h *grafH) Command(ctx context.Context, w hugot.ResponseWriter, m *command.Message) error {
	if err := m.Parse(); err != nil {
		return err
	}

	return h.cs.Command(ctx, w, m)
}

func Register(c *http.Client, url, token string) {
	h := New(c, url, token)
	bot.Command(h)
	bot.HandleHTTP(h.wh)
}
