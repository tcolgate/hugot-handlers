package prometheus

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/golang/glog"
	"github.com/prometheus/alertmanager/notify"
	"github.com/tcolgate/hugot"
	"golang.org/x/net/context"
)

func (*promH) watchAlerts(ctx context.Context, hw hugot.ResponseWriter, w http.ResponseWriter, r *http.Request) {
	hm := notify.WebhookMessage{}
	fmt.Fprintf(os.Stderr, "%#v", r.Body)

	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&hm); err != io.EOF || err != nil {
		glog.Error(err.Error())
		return
	}

	fmt.Fprintf(os.Stderr, "%#v", hm)
}
