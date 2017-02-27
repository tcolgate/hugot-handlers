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

	human "github.com/dustin/go-humanize"
	"github.com/golang/glog"

	lvst "github.com/tcolgate/go-livestatus"
	"github.com/tcolgate/hugot"
	"github.com/tcolgate/hugot/handlers/command"
)

func downtimesQuery(extcols ...string) *lvst.Query {
	l := lvst.NewLivestatusWithDialer(dial)
	q := l.Query("downtimes")
	q.Columns("host_name", "service_name", "service_display_name", "is_service", "author", "comment", "start_time", "end_time")
	return q
}

type downtime struct {
	host       string
	service    string
	service_dn string
	author     string
	comment    string
	start_time time.Time
	end_time   time.Time
	is_service bool
}

func downtimeFromRecord(r *lvst.Record) (*downtime, error) {
	var err error
	d := downtime{}
	d.host, err = r.GetString("host_name")
	if err != nil {
		return nil, fmt.Errorf("name", err)
	}

	is, err := r.GetInt("is_service")
	if err != nil {
		return nil, fmt.Errorf("is_service", err)
	}
	if is > 1 {
		d.is_service = true
	}

	if d.is_service {
		d.service, err = r.GetString("service_name")
		if err != nil {
			return nil, fmt.Errorf("service_name", err)
		}
	}

	d.author, err = r.GetString("author")
	if err != nil {
		return nil, fmt.Errorf("author", err)
	}

	d.comment, err = r.GetString("comment")
	if err != nil {
		return nil, fmt.Errorf("comment", err)
	}

	st, err := r.GetInt("start_time")
	if err != nil {
		return nil, fmt.Errorf("start_time", err)
	}
	d.start_time = time.Unix(st, 0)

	et, err := r.GetInt("end_time")
	if err != nil {
		return nil, fmt.Errorf("end_time", err)
	}
	d.end_time = time.Unix(et, 0)

	return &d, nil
}

func handleDowntimes(ctx context.Context, w hugot.ResponseWriter, m *command.Message) error {
	v := m.Bool("v", false, "verbose")
	if err := m.Parse(); err != nil {
		return err
	}

	q := downtimesQuery()
	if !*v {
		q.Limit(10)
	}

	resp, err := q.Exec()
	if err != nil {
		glog.Infoln(err)
		return err
	}

	if len(resp.Records) == 0 {
		fmt.Fprintf(w, "No downtimes found")
		return nil
	}

	for _, r := range resp.Records {
		d, err := downtimeFromRecord(&r)
		if err != nil {
			glog.Infoln(err)
			continue
		}
		glog.Infof("%#v", d)

		rep := m.Reply("")

		text := ""
		if d.is_service {
			text = serviceLink(d.host, d.service, d.service_dn)
		} else {
			text = hostLink(d.host)
		}

		rep.Attachments = []hugot.Attachment{attachment(
			"",
			text,
			"",
			[]field{
				field{
					key:   "start/end",
					value: fmt.Sprintf("%s/%s", human.Time(d.start_time), human.Time(d.end_time)),
					short: true,
				},
				field{
					key:   "A,uthor",
					value: d.author,
					short: true,
				},
				field{
					key:   "Comment",
					value: d.comment,
					short: false,
				},
			})}

		w.Send(ctx, rep)
	}
	return nil
}
