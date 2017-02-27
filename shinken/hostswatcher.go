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
	"time"

	"context"

	"github.com/golang/glog"
	"github.com/tcolgate/hugot"

	lvst "github.com/tcolgate/go-livestatus"
)

func processHostChange(ctx context.Context, w hugot.ResponseWriter, resp *lvst.Response, t time.Time) time.Time {
	nt := t

	for _, r := range resp.Records {
		h, err := hostFromRecord(&r)
		if err != nil {
			glog.Infoln("Could not read record as host, ", err.Error())
		}

		if h.last_hard_state_change.After(nt) {
			nt = h.last_hard_state_change
		}

		// Skip if the state hasn't changed
		if h.state == h.last_hard_state {
			continue
		}

		should_here := ""
		if h.state == 2 && h.business_impact >= 4 {
			should_here = " <-- <!here|here>"
		}

		w.SetChannel(*alertsChan)
		w.Send(ctx, &hugot.Message{Channel: *alertsChan,
			Attachments: []hugot.Attachment{attachment(
				"",
				fmt.Sprintf("Host %v %v (was %v) [%v]%v", hostLink(h.name), host_state_names[h.state], host_state_names[h.last_hard_state], business_impact_emojis[h.business_impact-1], should_here),
				state_colours[h.state],
				[]field{})}})
	}

	return nt
}

func hostsWatcher(ctx context.Context, w hugot.ResponseWriter) error {
	t := time.Now()
	for {
		q := hostsQuery()
		q.Filter("state = 0")
		q.Filter("is_flapping = 0")
		q.Filter("last_hard_state = 0")
		q.Filter("acknowledged != 1")
		q.Filter("checks_enabled = 1")
		q.Filter("scheduled_downtime_depth = 0")
		q.Filter(fmt.Sprintf("last_hard_state_change > %d", t.Unix()))
		q.WaitCondition("state = 0")
		q.WaitCondition("is_flapping = 0")
		q.WaitCondition("last_hard_state = 0")
		q.WaitCondition("acknowledged != 1")
		q.WaitCondition("checks_enabled = 1")
		q.WaitCondition("scheduled_downtime_depth = 0")
		q.WaitCondition(fmt.Sprintf("last_hard_state_change > %d", t.Unix()))
		q.WaitTrigger("state")

		resp, err := q.Exec()
		if err != nil {
			continue
		}
		t = processHostChange(ctx, w, resp, t)
	}
}
