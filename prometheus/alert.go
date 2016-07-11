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
	"github.com/tcolgate/hugot"
)

func (p *promH) alertCmd(ctx context.Context, w hugot.ResponseWriter, m *hugot.Message) error {
	if err := m.Parse(); err != nil {
		return err
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
	io.Copy(ioutil.Discard, r.Body)

	fmt.Fprintf(rw, "%#v", hm)
}
