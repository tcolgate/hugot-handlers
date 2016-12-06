package grafana

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"context"

	"github.com/golang/glog"
	"github.com/tcolgate/hugot"
)

func (h *grafH) graphCmd(ctx context.Context, w hugot.ResponseWriter, m *hugot.Message) error {
	dur := m.Duration("d", 15*time.Minute, "how far back to render")

	if err := m.Parse(); err != nil {
		return err
	}

	if len(m.Args()) == 0 {
		return fmt.Errorf("you need to give a query")
	}
	q := strings.Join(m.Args(), " ")
	s := time.Now().Add(-1 * *dur)
	e := time.Now()
	nu := *h.URL()
	nu.Path = nu.Path + "graph/thing.png"
	qs := nu.Query()
	qs.Set("q", q)
	qs.Set("s", fmt.Sprintf("%d", s.Unix()))
	qs.Set("e", fmt.Sprintf("%d", e.Unix()))
	nu.RawQuery = qs.Encode()

	om := hugot.Message{
		Channel: m.Channel,
		Attachments: []hugot.Attachment{
			{
				Fallback: "fallback",
				ImageURL: nu.String(),
			},
		},
	}
	w.Send(ctx, &om)
	return nil
}

func (h *grafH) graphHook(w http.ResponseWriter, r *http.Request) {
	q, ok := r.URL.Query()["q"]
	if !ok || len(q) != 1 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	s, ok := r.URL.Query()["s"]
	if !ok || len(s) != 1 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	e, ok := r.URL.Query()["e"]
	if !ok || len(e) != 1 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	st, _ := strconv.Atoi(s[0])
	et, _ := strconv.Atoi(e[0])

	glog.Infof("%v %v %v", st, et)

	ctx := r.Context()

	hr, err := http.NewRequest("GET", fmt.Sprintf(`%s/render/dashboard-solo/db/prometheus-stats?from=%s&to=%s&panelId=3&width=%s&height=%s`, h.url, st, et, 1000, 600), nil)
	if err != nil {
		glog.Infof("%v", err.Error())
		return
	}

	hr = hr.WithContext(ctx)

	hr.Header.Set("Authorization", "Bearer "+h.token)

	resp, err := h.c.Do(hr)
	if err != nil {
		glog.Infof("%v", err.Error())
		return
	}

	io.Copy(w, resp.Body)
}
