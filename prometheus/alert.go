package prometheus

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/golang/glog"
	"github.com/prometheus/alertmanager/notify"
	am "github.com/tcolgate/client_golang/api/alertmanager"
	"github.com/tcolgate/hugot"
)

func (p *promH) alertsCmd(ctx context.Context, w hugot.ResponseWriter, m *hugot.Message) error {
	if err := m.Parse(); err != nil {
		return err
	}
	as, err := am.NewAlertAPI(p.amclient).List(ctx)
	if err != nil {
		return err
	}

	if len(as) == 0 {
		fmt.Fprint(w, "There are no outstanding alerts")
		return nil
	}

	for _, a := range as {
		fmt.Fprintf(w, "%#v", a)
	}
	return nil
}

func (p *promH) silencesCmd(ctx context.Context, w hugot.ResponseWriter, m *hugot.Message) error {
	if err := m.Parse(); err != nil {
		return err
	}
	ss, err := am.NewSilenceAPI(p.amclient).List(ctx)
	if err != nil {
		return err
	}

	if len(ss) == 0 {
		fmt.Fprint(w, "There are no active silences")
		return nil
	}

	for _, s := range ss {
		fmt.Fprintf(w, "%#v", s)
	}
	return nil
}

func (*promH) alertsHook(w http.ResponseWriter, r *http.Request) {
	rw, ok := hugot.ResponseWriterFromContext(r.Context())
	if !ok {
		http.NotFound(w, r)
	}

	if glog.V(3) {
		glog.Infof("%s %s", r.Method, r.URL)
	}

	hm := notify.WebhookMessage{}
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&hm); err != io.EOF && err != nil {
		glog.Error(err.Error())
		return
	}
	// Get rid of any trailing space after decode
	io.Copy(ioutil.Discard, r.Body)

	fmt.Fprintf(rw, "%#v", hm)
}
