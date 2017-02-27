package shinken

import (
	"fmt"
	"time"

	"context"

	"github.com/golang/glog"
	lvst "github.com/tcolgate/go-livestatus"
	nag "github.com/tcolgate/go-livestatus/nagios"
	"github.com/tcolgate/hugot"
	"github.com/tcolgate/hugot/handlers/command"
)

func handleDown(ctx context.Context, w hugot.ResponseWriter, m *command.Message) error {
	v := m.Bool("v", false, "verbose")
	dur := m.Duration("d", time.Hour, "Duration of the downtime")
	comm := m.String("c", "Downed via hugot", "Add a comment")
	if err := m.Parse(); err != nil {
		return err
	}

	if len(m.Args()) != 1 {
		fmt.Fprint(w, "you must provide one pattern of host names to put into downtime")
	}

	lv := lvst.NewLivestatusWithDialer(dial)
	defer lv.Close()

	q := hostsQuery()
	q.Filter(fmt.Sprintf("host_name ~ ^%s", m.Args()[0]))

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
	for _, r := range resp.Records {
		h, err := hostFromRecord(&r)
		if err != nil {
			glog.Infoln(err)
			continue
		}

		c := lv.Command()
		c.Op(nag.ScheduleHostDowntime(h.name, time.Now(), time.Now().Add(*dur), true, 0, *dur, m.From, *comm))
		c.Exec()

		if *v {
			fmt.Fprintf(w, "Setting downtime on.. %s", h.Link())
		}
		hc++
	}
	fmt.Fprintf(w, "Downed %d hosts", hc)
	return nil
}
