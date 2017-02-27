// Copyright (c) 2016 Tristan Colgate-McFarlane
//
// This file is part of hugot.
//
// hugot is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// hugot is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with hugot.  If not, see <http://www.gnu.org/licenses/>.

package shinken

import (
	"fmt"

	"context"

	"log"

	"github.com/golang/glog"
	"github.com/tcolgate/hugot"
	"github.com/tcolgate/hugot/handlers/command"
)

func handleAlerts(ctx context.Context, w hugot.ResponseWriter, m *command.Message) error {
	v := m.Bool("v", false, "verbose")
	limit := m.Int("l", 100, "limit of records to return")
	if err := m.Parse(); err != nil {
		return err
	}

	q := servicesQuery()
	q.Filter("state != 0")
	q.Filter("checks_enabled = 1")
	q.Filter("scheduled_downtime_depth = 0")
	q.Filter("host_checks_enabled = 1")
	q.Filter("host_scheduled_downtime_depth = 0")
	q.Limit(*limit)

	resp, err := q.Exec()
	if err != nil {
		glog.Infoln(err)
		return err
	}

	if len(resp.Records) == *limit {
		fmt.Fprintf(w, "Query returned the maximum number of items allowed (%v).", *limit)
	}

	type hkey struct {
		name  string
		state state
	}
	hsts := map[hkey][]*service{}

	ackd := 0
	for _, r := range resp.Records {
		if glog.V(3) {
			glog.Infof("%#v\n", r)
		}
		s, err := serviceFromRecord(&r)
		if err != nil {
			glog.Infoln("Could not read record as service, ", err.Error())
			continue
		}

		if !*v && (s.acknowledged || s.host.acknowledged) {
			ackd++
			continue
		}
		k := hkey{s.host.name, s.host.state}
		hsts[k] = append(hsts[k], s)
	}

	if len(hsts) == 0 && ackd == 0 {
		fmt.Print(w, "There are no service alerts at all! Monitoring is porbably down.")
		return nil
	}

	for h := range hsts {
		rep := m.Reply("")
		text := fmt.Sprintf("%v %v", hostLink(h.name), host_state_names[h.state])
		rep.Attachments = append(rep.Attachments, hugot.Attachment{Pretext: text})
		if h.state == 0 {
			for _, s := range hsts[h] {
				text := fmt.Sprintf("%v %v (%v)",
					s.Link(),
					business_impact_emojis[s.business_impact-1],
					urlIfy("fix", s.action_url))

				log.Println(s.last_hard_state)
				rep.Attachments = append(rep.Attachments, attachment("", text, state_colours[s.state], nil))
			}
		}
		w.Send(ctx, rep)
	}
	fmt.Fprintf(w, "There are %v ack'd service alerts.", ackd)

	return nil
}
