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

	"context"

	"github.com/golang/glog"
	"github.com/tcolgate/hugot"
	"github.com/tcolgate/hugot/handlers/command"

	lvst "github.com/tcolgate/go-livestatus"
)

type host struct {
	lvsBase
	services []service
}

func (h *host) Link() string {
	return hostLink(h.name)
}

type svcState struct {
	name    string
	state   int64
	checked bool
}

func serviceState(sl interface{}) (svcState, error) {
	bad := errors.New("invalid service state slice")

	st := svcState{}

	sls, ok := sl.([]interface{})
	glog.Infof("%#v %#v %#v\n", sl, sls, ok)
	if !ok {
		return st, bad
	}

	if len(sls) != 3 {
		return st, bad
	}

	if st.name, ok = sls[0].(string); !ok {
		return st, bad
	}

	var fl float64
	if fl, ok = sls[1].(float64); !ok {
		return st, bad
	}
	st.state = int64(fl)

	if fl, ok = sls[2].(float64); !ok {
		return st, bad
	}
	if fl != 0.0 {
		st.checked = true
	}

	return st, nil
}

var hostsColumns = []string{"services_with_state"}

func hostsQuery(extcols ...string) *lvst.Query {
	cols := baseColumns
	cols = append(cols, hostsColumns...)
	cols = append(cols, extcols...)
	l := lvst.NewLivestatusWithDialer(dial)
	q := l.Query("hosts")
	q.Columns(cols...)
	return q
}

func hostFromRecord(r *lvst.Record) (*host, error) {
	b, err := lvsBaseFromRecord(r)
	if err != nil {
		return nil, err
	}

	h := host{lvsBase: *b}

	return &h, nil
}

func handleHosts(ctx context.Context, w hugot.ResponseWriter, m *command.Message) error {
	v := m.Bool("v", false, "verbose")
	if err := m.Parse(); err != nil {
		return err
	}

	if len(m.Args()) != 1 {
		return errors.New("you must provide a regex for the hosts")
	}

	q := hostsQuery()
	q.Filter(fmt.Sprintf("host_name ~ %s", m.Args()[0]))
	q.Limit(10)

	resp, err := q.Exec()
	if err != nil {
		glog.Infoln(err)
		return err
	}

	if len(resp.Records) == 0 {
		fmt.Fprintf(w, "No matching hosts found")
		return nil
	}

	for _, r := range resp.Records {
		h, err := hostFromRecord(&r)
		if err != nil {
			glog.Infoln(err)
			continue
		}
		glog.Infof("%#v", h)

		rep := m.Reply("")
		text := fmt.Sprintf("%v %v", h.Link(), host_state_names[h.state])

		rep.Attachments = []hugot.Attachment{attachment(text, "", "", nil)}

		if h.last_hard_state == 0 {
			svcs, err := r.GetSlice("services_with_state")
			if err != nil {
				glog.Infoln(err)
				continue
			}
			for i := range svcs {
				svc, err := serviceState(svcs[i])
				if err != nil {
					glog.Infoln(err)
					continue
				}
				if *v || svc.state > 0 {
					text := fmt.Sprintf("%v %v",
						serviceLink(h.name, svc.name, svc.name),
						service_state_names[svc.state])

					rep.Attachments = append(rep.Attachments, attachment("", text, state_colours[svc.state], nil))
				}
			}
		}

		w.Send(ctx, rep)
	}
	return nil
}
