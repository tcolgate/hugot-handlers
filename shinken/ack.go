package shinken

import (
	"fmt"

	"context"

	"github.com/golang/glog"
	lvst "github.com/tcolgate/go-livestatus"
	nag "github.com/tcolgate/go-livestatus/nagios"
	"github.com/tcolgate/hugot"
	"github.com/tcolgate/hugot/handlers/command"
)

func handleAck(ctx context.Context, w hugot.ResponseWriter, m *command.Message) error {
	v := m.Bool("v", false, "verbose")
	comm := m.String("c", "Acknowledged via hugot", "comment")
	svcs := m.String("s", ".", "only ack the matching services on the matching hosts")
	if err := m.Parse(); err != nil {
		return err
	}

	if len(m.Args()) != 1 {
		fmt.Fprint(w, "you must provide one pattern of host names to ack")
	}

	lv := lvst.NewLivestatusWithDialer(dial)
	defer lv.Close()

	q := servicesQuery()
	q.Filter(fmt.Sprintf("display_name ~ ^%s", *svcs))
	q.Filter(fmt.Sprintf("service_description ~ ^%s", *svcs))
	q.Or(2)
	q.Filter(fmt.Sprintf("host_name ~ ^%s", m.Args()[0]))
	q.Filter(fmt.Sprintf("state != 0"))

	resp, err := q.Exec()
	if err != nil {
		glog.Infoln(err)
		return err
	}

	if len(resp.Records) == 0 {
		fmt.Fprintf(w, "No matching hosts found")
		return nil
	}

	hc := 0
	sc := 0
	hostAcked := map[string]bool{}
	for _, r := range resp.Records {
		s, err := serviceFromRecord(&r)
		if err != nil {
			glog.Infoln(err)
			continue
		}

		c := lv.Command()
		c.Op(nag.AcknowledgeSvcProblem(s.host.name, s.description, true, true, true, m.From, *comm))
		c.Exec()

		if _, ok := hostAcked[s.host.name]; !ok {
			c := lv.Command()
			c.Op(nag.AcknowledgeHostProblem(s.host.name, true, true, true, m.From, "Acknolwdged via hugot"))
			c.Exec()
			hc++
		}

		c = lv.Command()
		c.Op(nag.AcknowledgeSvcProblem(s.host.name, s.description, true, true, true, m.From, *comm))
		c.Exec()
		sc++

		if *v {
			fmt.Fprintf(w, "Acking.. %s on %s", s.Link(), s.host.Link())
		}
	}
	fmt.Fprintf(w, "Ack'd %d services on %d hosts", sc, hc)
	return nil
}
