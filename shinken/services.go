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
	"errors"
	"fmt"
	"time"

	"context"

	"github.com/golang/glog"
	"github.com/tcolgate/hugot"
	"github.com/tcolgate/hugot/handlers/command"

	lvst "github.com/tcolgate/go-livestatus"
)

type service struct {
	lvsBase
	display_name string
	description  string
	host         host
}

func (s *service) Link() string {
	return serviceLink(s.host.name, s.description, s.display_name)
}

var servicesColumns = []string{"service_description", "host_name", "host_state", "host_last_hard_state_change", "host_last_hard_state", "host_action_url_expanded", "host_business_impact", "host_scheduled_downtime_depth", "host_comments_with_info", "host_acknowledged", "display_name"}

func serviceFromRecord(r *lvst.Record) (*service, error) {
	b, err := lvsBaseFromRecord(r)
	if err != nil {
		return nil, err
	}

	s := service{lvsBase: *b}

	s.display_name, err = r.GetString("display_name")
	if err != nil {
		return nil, fmt.Errorf("display_name", err)
	}

	s.description, err = r.GetString("service_description")
	if err != nil {
		return nil, fmt.Errorf("service_description", err)
	}

	s.host.name, err = r.GetString("host_name")
	if err != nil {
		return nil, fmt.Errorf("host_name", err)
	}

	lsc, err := r.GetInt("host_last_hard_state_change")
	if err != nil {
		return nil, fmt.Errorf("host_last_hard_state_change", err)
	}
	s.host.last_hard_state_change = time.Unix(lsc, 0)

	n, err := r.GetInt("host_state")
	if err != nil {
		return nil, fmt.Errorf("host_state", err)
	}
	s.host.state = state(n)

	n, err = r.GetInt("host_last_hard_state")
	if err != nil {
		last_statestr, err := r.GetString("host_last_hard_state")
		var ok bool
		if n, ok = service_states[last_statestr]; err != nil || !ok {
			return nil, fmt.Errorf("host_last_hard_state", err)
		}
	}
	s.host.last_hard_state = state(n)

	s.host.action_url, err = r.GetString("host_action_url_expanded")
	if err != nil {
		return nil, fmt.Errorf("host_action_url_expanded", err)
	}

	//s.host.business_impact, err = r.GetInt("host_business_impact")
	//if err != nil {
	//		return nil, fmt.Errorf("host_business_impact", err)
	//}

	s.host.scheduled_downtime_depth, err = r.GetInt("host_scheduled_downtime_depth")
	if err != nil {
		return nil, fmt.Errorf("host_scheduled_downtime_depth", err)
	}

	n, err = r.GetInt("host_acknowledged")
	if err != nil {
		return nil, fmt.Errorf("host_acknowledged", err)
	}
	if n > 0 {
		s.host.acknowledged = true
	}

	return &s, nil
}

func handleServices(ctx context.Context, w hugot.ResponseWriter, m *command.Message) error {
	//	v := m.Bool("v", false, "verbose")
	if err := m.Parse(); err != nil {
		return err
	}

	if len(m.Args()) != 1 {
		return errors.New("you must provide a regex for the service")
	}

	q := servicesQuery()
	q.Filter(fmt.Sprintf("service_description ~ %s", m.Args()[0]))
	q.Limit(10)

	resp, err := q.Exec()
	if err != nil {
		glog.Infoln(err)
		return err
	}

	if len(resp.Records) == 0 {
		fmt.Fprintf(w, "No matching services found")
		return nil
	}

	for _, r := range resp.Records {
		s, err := serviceFromRecord(&r)
		if err != nil {
			glog.Infoln("Could not read record as service, ", err.Error())
			continue
		}

		rep := m.Reply("")
		text := fmt.Sprintf("%v %v", s.Link(), service_state_names[s.state])

		rep.Attachments = []hugot.Attachment{attachment(text, "", "", nil)}

		w.Send(ctx, rep)
	}
	return nil
}
