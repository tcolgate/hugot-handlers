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
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"html/template"
	"io/ioutil"
	"net"
	"os"
	"time"

	"context"

	"github.com/nlopes/slack"

	lvst "github.com/tcolgate/go-livestatus"
	"github.com/tcolgate/hugot"
	"github.com/tcolgate/hugot/bot"
	"github.com/tcolgate/hugot/handlers/command"
)

func init() {
	hostUrlTmpl = template.Must(template.New("name").Parse(*hostUrlTemplStr))
	serviceUrlTmpl = template.Must(template.New("name").Parse(*serviceUrlTemplStr))
}

var (
	serverAddr         = flag.String("lv.addr", os.Getenv("LV_HOST"), "Location of private client ssl key")
	serverName         = flag.String("lv.name", os.Getenv("LV_NAME"), "Name used in the cert of the lv host")
	keyPath            = flag.String("lv.key", "/opt/hugot/config/key.pem", "Location of private client ssl key")
	certPath           = flag.String("lv.cert", "/opt/hugot/config/cert.pem", "Location of private client ssl cert")
	caPath             = flag.String("lv.ca", "/opt/hugot/config/ca.pem", "Location of ca certs file")
	alertsChan         = flag.String("lv.alertschan", os.Getenv("LV_ALERTS_CHANNEL"), "Channel to report alerts to")
	hostUrlTemplStr    = flag.String("lv.hosturl", os.Getenv("LV_HOST_LINK"), "A go template for the host link")
	serviceUrlTemplStr = flag.String("lv.serviceurl", os.Getenv("LV_SERVICE_LINK"), "A go template for the service link")
)

func New() (command.Commander, hugot.BackgroundHandler) {
	cs := command.NewSet()
	cs.Add(command.New("a", "alerts", handleAlerts))
	cs.Add(command.New("d", "downtimes", handleDowntimes))
	cs.Add(command.New("h", "hosts", handleHosts))
	cs.Add(command.New("s", "hosts", handleServices))
	cs.Add(command.New("ack", "acknowledge hosts or alerts", handleAck))
	cs.Add(command.New("down", "set downtimes on hosts", handleDown))

	ch := command.New("shinken", "interact with shinken livestatus",
		func(ctx context.Context, w hugot.ResponseWriter, m *command.Message) error {
			if err := m.Parse(); err != nil {
				return err
			}

			return cs.Command(ctx, w, m)
		})

	bgh := hugot.NewBackgroundHandler("shin", "watches service and host states", startBackground)

	return ch, bgh
}

func Register() {
	ch, bgh := New()
	bot.Command(ch)
	bot.Background(bgh)
}

func startBackground(ctx context.Context, w hugot.ResponseWriter) {
	go servicesWatcher(ctx, w)
	go hostsWatcher(ctx, w)
}

var service_states = map[string]int64{
	"OK":       0,
	"WARNING":  1,
	"CRITICAL": 2,
	"UNKNOWN":  3,
	"PENDING":  4,
}

var service_state_names = []string{
	"OK",
	"WARNING",
	"CRITICAL",
	"UNKNOWN",
	"PENDING",
}

var state_colours = []string{
	"good",
	"warning",
	"danger",
	"#808080",
	"#8a2be2", // Purple for P for Pending
}

var host_state_names = []string{
	"UP",
	"DOWN",
}

type field struct {
	short bool
	key   string
	value interface{}
}

type urlData struct {
	Hostname    string
	Servicename string
}

var business_impact_emojis = []string{
	"",
	"",
	":bell:",
	":fire:",
	":fire::fire:",
}

func urlIfy(title, url string) string {
	return fmt.Sprintf("<%s|%s>", url, title)
}

func hostURL(hostname string) string {
	buf := &bytes.Buffer{}
	hostUrlTmpl.Execute(buf, urlData{hostname, ""})
	return buf.String()
}

func serviceURL(hostname, servicename string) string {
	buf := &bytes.Buffer{}
	serviceUrlTmpl.Execute(buf, urlData{hostname, servicename})
	return buf.String()
}

func serviceLink(hostname, servicename, displayname string) string {
	return urlIfy(displayname, serviceURL(hostname, servicename))
}

func hostLink(hostname string) string {
	return urlIfy(hostname, hostURL(hostname))
}

type state int64

type comment struct {
}

type lvsBase struct {
	name                     string
	acknowledged             bool
	comments                 []comment
	last_hard_state          state
	state                    state
	business_impact          int64
	scheduled_downtime_depth int64
	action_url               string
	last_hard_state_change   time.Time
}

var baseColumns = []string{"name", "state", "last_hard_state_change", "last_hard_state", "action_url_expanded", "business_impact", "scheduled_downtime_depth", "comments_with_info", "acknowledged"}

func servicesQuery(extcols ...string) *lvst.Query {
	cols := baseColumns
	cols = append(cols, servicesColumns...)
	cols = append(cols, extcols...)
	l := lvst.NewLivestatusWithDialer(dial)
	q := l.Query("services")
	q.Columns(cols...)
	return q
}

func lvsBaseFromRecord(r *lvst.Record) (*lvsBase, error) {
	var err error
	l := lvsBase{}

	l.name, err = r.GetString("name")
	if err != nil {
		return nil, fmt.Errorf("no name", err)
	}

	lsc, err := r.GetInt("last_hard_state_change")
	if err != nil {
		return nil, fmt.Errorf("no last_hard_state_change", err)
	}
	l.last_hard_state_change = time.Unix(lsc, 0)

	n, err := r.GetInt("state")
	if err != nil {
		return nil, fmt.Errorf("no state", err)
	}
	l.state = state(n)

	n, err = r.GetInt("last_hard_state")
	if err != nil {
		last_statestr, err := r.GetString("last_hard_state")
		var ok bool
		if n, ok = service_states[last_statestr]; err != nil || !ok {
			return nil, fmt.Errorf("no last_hard_state", err)
		}
	}
	l.last_hard_state = state(n)

	l.action_url, err = r.GetString("action_url_expanded")
	if err != nil {
		return nil, fmt.Errorf("no action_url_expanded", err)
	}

	l.business_impact, err = r.GetInt("business_impact")
	if err != nil {
		return nil, fmt.Errorf("no business_impact", err)
	}

	n, err = r.GetInt("acknowledged")
	if err != nil {
		return nil, fmt.Errorf("no acknowledged", err)
	}
	if n > 0 {
		l.acknowledged = true
	}

	l.scheduled_downtime_depth, err = r.GetInt("scheduled_downtime_depth")
	if err != nil {
		return nil, fmt.Errorf("no scheduled_downtime_depth", err)
	}

	return &l, nil
}

func attachment(pretext, text string, color string, fields []field) hugot.Attachment {
	var attch hugot.Attachment
	attch.Text = text
	attch.Pretext = pretext
	attch.MarkdownIn = []string{"text", "pretext", "fields"}
	attch.Color = color
	for _, j := range fields {
		attch.Fields = append(attch.Fields, slack.AttachmentField{
			Title: j.key,
			Value: fmt.Sprintf("%v", j.value),
			Short: j.short,
		})
	}
	return attch
}

var hostUrlTmpl *template.Template
var serviceUrlTmpl *template.Template

func dial() (net.Conn, error) {
	cert, err := tls.LoadX509KeyPair(*certPath, *keyPath)
	if err != nil {
		return nil, err
	}

	// Load CA cert
	caCert, err := ioutil.ReadFile(*caPath)
	if err != nil {
		return nil, err
	}
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	config := tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      caCertPool,
	}

	if *serverName != "" {
		config.ServerName = *serverName
	}

	c, err := tls.Dial("tcp", *serverAddr, &config)
	if err != nil {
		return nil, err
	}

	c.Handshake()
	return c, err
}
