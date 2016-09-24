package prometheus

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
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
		fmt.Fprintf(w, "%s: Started at %s, %#v, %#v", a.Labels["alertname"], a.StartsAt, a.Labels, a.Annotations)
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

func (p *promH) alertsHook(w http.ResponseWriter, r *http.Request) {
	rw, ok := hugot.ResponseWriterFromContext(r.Context())
	if !ok {
		http.NotFound(w, r)
	}

	if glog.V(2) {
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

	log.Println(rw)
	//rw.SetChannel(p.alertChan)
	//status := strings.ToUpper(hm.Data.Status)
	//fmt.Fprintf(rw, "%s: %#v", status, hm.Data)
}
