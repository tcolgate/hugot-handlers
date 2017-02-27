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

func processServiceChange(ctx context.Context, w hugot.ResponseWriter, resp *lvst.Response, t time.Time) time.Time {
	nt := t

	for _, r := range resp.Records {
		s, err := serviceFromRecord(&r)
		if err != nil {
			glog.Infoln("Could not read record as service, ", err.Error())
		}

		if s.last_hard_state_change.After(nt) {
			nt = s.last_hard_state_change
		}

		// Skip if the state hasn't changed
		if s.state == s.last_hard_state {
			continue
		}

		text := fmt.Sprintf("Service %v / %v - %v (was %v)",
			hostLink(s.host.name),
			s.Link(),
			service_state_names[s.state],
			service_state_names[s.last_hard_state])

		if s.state != 0 {
			text = fmt.Sprintf("%s %s (%s)", text, business_impact_emojis[s.business_impact-1], urlIfy("fix", s.action_url))
		}

		if false && s.state == 2 && s.business_impact >= 4 {
			text = fmt.Sprintf("%s <-- <!here|here>", text)
		}

		w.SetChannel(*alertsChan)
		newm := &hugot.Message{Channel: *alertsChan,
			Attachments: []hugot.Attachment{attachment(
				"",
				text,
				state_colours[s.state],
				[]field{})}}
		w.Send(ctx, newm)

		glog.Infof("&+v\n", newm)
	}

	return nt
}

func servicesWatcher(ctx context.Context, w hugot.ResponseWriter) error {
	t := time.Now()

	for {
		q := servicesQuery()
		q.Filter("state_type = 1")
		q.Filter("checks_enabled = 1")
		q.Filter("scheduled_downtime_depth = 0")
		q.Filter("is_flapping = 0")
		q.Filter("host_state = 0")
		q.Filter("host_last_hard_state = 0")
		q.Filter("host_acknowledged != 1")
		q.Filter("host_is_flapping = 0")
		q.Filter("host_checks_enabled = 1")
		q.Filter("host_scheduled_downtime_depth = 0")
		q.Filter(fmt.Sprintf("last_hard_state_change > %d", t.Unix()))
		q.WaitCondition("state_type = 1")
		q.WaitCondition("checks_enabled = 1")
		q.WaitCondition("scheduled_downtime_depth = 0")
		q.WaitCondition("is_flapping = 0")
		q.WaitCondition("host_state = 0")
		q.WaitCondition("host_is_flapping = 0")
		q.WaitCondition("host_last_hard_state = 0")
		q.WaitCondition("host_acknowledged != 1")
		q.WaitCondition("host_checks_enabled = 1")
		q.WaitCondition("host_scheduled_downtime_depth = 0")
		q.WaitCondition(fmt.Sprintf("last_hard_state_change > %d", t.Unix()))
		q.WaitTrigger("state")

		resp, err := q.Exec()
		if err != nil {
			continue
		}
		t = processServiceChange(ctx, w, resp, t)
	}
}
