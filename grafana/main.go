package grafana

import (
	"net/http"
	"time"

	"github.com/tcolgate/hugot"
	"github.com/tcolgate/hugot/bot"
	"github.com/tcolgate/hugot/handlers/command"
)

func init() {
}

type grafH struct {
	*command.Handler
	c     *http.Client
	url   string
	token string

	hmux *http.ServeMux
	wh   hugot.WebHookHandler
}

// New prometheus handler, returns a command and a webhook handler
func New(c *http.Client, url, token string) *grafH {
	h := &grafH{nil, c, url, token, http.NewServeMux(), nil}

	h.hmux.HandleFunc("/", http.NotFound)
	h.hmux.HandleFunc("/graph", h.graphHook)
	h.hmux.HandleFunc("/graph/", h.graphHook)

	h.wh = hugot.NewWebHookHandler("grafana", "", h.webHook)
	h.Handler = command.NewFunc(h.Setup)

	return h
}

func (h *grafH) Setup(root *command.Command) error {
	root.Use = "grafana"
	root.Short = "render a graph from grafana"

	gCtx := &grafCtx{
		wh: h.wh,
	}
	gCtx.dur = root.Flags().DurationP("duration", "d", 15*time.Minute, "how far back to render")
	root.Run = gCtx.graphCmd

	return nil
}

func Register(c *http.Client, url, token string) {
	h := New(c, url, token)
	bot.Command(h.Handler)
	bot.HandleHTTP(h.wh)
}
