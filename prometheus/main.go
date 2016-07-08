package prometheus

import (
	"flag"

	"github.com/golang/glog"
	prom "github.com/prometheus/client_golang/api/prometheus"
	"github.com/tcolgate/hugot"
)

func init() {
	h, bh := New(
		flag.String("prom.url", "http://localhost:9090", "prometheus URL"),
		flag.String("prom.am.url", "http://localhost:9093", "prometheus alert manager URL"),
		flag.String("prom.alertChan", "alerts", "channel to post new alerts to"),
		nil,
	)

	hugot.AddCommandHandler(h)
	hugot.AddWebHookHandler(bh)
}

type promH struct {
	client    *prom.Client
	amURL     *string
	alertChan *string
}

// New prometheus handler, returns a command and a webhook handler
func New(purl, amurl, achan *string, ptx prom.CancelableTransport) (hugot.CommandHandler, hugot.WebHookHandler) {
	c, err := prom.New(prom.Config{*purl, ptx})
	if err != nil {
		glog.Errorf("could not create prom client, %s", err.Error())
		return nil, nil
	}

	ph := &promH{&c, amurl, achan}

	cs := hugot.NewCommandSet()
	cs.AddCommandHandler(hugot.NewCommandHandler("graph", "graph a query", ph.graphCmd, nil))
	cs.AddCommandHandler(hugot.NewCommandHandler("explain", "explains the meaning of an alert rule name", ph.explainCmd, nil))

	ch := hugot.NewCommandHandler("prometheus", "manage the prometheus monitoring system", nil, cs)
	bh := hugot.NewWebHookHandler("prometheus", "reports alerts from alertmanager", ph.watchAlerts)
	return ch, bh
}
