package prometheus

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/golang/glog"
	"github.com/prometheus/alertmanager/notify"
	"github.com/prometheus/common/model"
	"github.com/tcolgate/hugot"
	"github.com/tcolgate/hugot-handlers/prometheus/am"
	"github.com/tcolgate/hugot/handlers/command"
)

func (p *promH) alertsCmd(ctx context.Context, w hugot.ResponseWriter, m *command.Message) error {
	if err := m.Parse(); err != nil {
		return err
	}
	ags, err := am.NewAlertAPI(p.amclient).ListGroups(ctx)
	if err != nil {
		return err
	}

	if len(ags) == 0 {
		fmt.Fprint(w, "There are no outstanding alerts")
		return nil
	}

	for _, ag := range ags {
		ls := ag.Labels
		for _, b := range ag.Blocks {
			if b.RouteOpts.Receiver != "chat" {
				continue
			}

			as := modelToLocal(b.Alerts)
			active := []alert{}
			for _, a := range as {
				if a.Resolved() {
					continue
				}

				if a.Inhibited {
					continue
				}

				if len(a.Silenced) != 0 {
					continue
				}
				active = append(active, a)
			}

			if len(active) == 0 {
				continue
			}

			d := data(b.RouteOpts.Receiver, p.amURL, ls, active)
			rm, err := p.alertMessage(d)
			if err != nil {
				fmt.Fprintf(w, "error rendering template, %v", err)
				continue
			}

			rm.Channel = m.Channel
			rm.To = m.From
			w.Send(ctx, rm)
		}
	}
	return nil
}

func (p *promH) silencesCmd(ctx context.Context, w hugot.ResponseWriter, m *command.Message) error {
	if err := m.Parse(); err != nil {
		return err
	}
	ss, err := am.NewSilenceAPI(p.amclient).List(ctx)
	if err != nil {
		return err
	}

	if len(ss) == 0 {
		fmt.Fprint(w, "There are no active silences")
		return nil
	}

	for _, s := range ss {
		fmt.Fprintf(w, "%#v", s)
	}
	return nil
}

func (p *promH) alertsHook(w http.ResponseWriter, r *http.Request) {
	rw, ok := hugot.ResponseWriterFromContext(r.Context())
	if !ok {
		http.NotFound(w, r)
	}

	if glog.V(2) {
		glog.Infof("%s %s", r.Method, r.URL)
	}

	hm := notify.WebhookMessage{}
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&hm); err != io.EOF && err != nil {
		glog.Error(err.Error())
		return
	}
	// Get rid of any trailing space after decode
	io.Copy(ioutil.Discard, r.Body)

	channel := bytes.Buffer{}
	err := p.tmpls.ExecuteTemplate(&channel, "channel", &hm.Data)
	if err != nil {
		glog.Infof("error expanding template, ", err.Error())
		return
	}

	title := bytes.Buffer{}
	err = p.tmpls.ExecuteTemplate(&title, "title", &hm.Data)
	if err != nil {
		glog.Infof("error expanding template, ", err.Error())
		return
	}

	titleLink := bytes.Buffer{}
	err = p.tmpls.ExecuteTemplate(&titleLink, "title_link", &hm.Data)
	if err != nil {
		glog.Infof("error expanding template, ", err.Error())
		return
	}

	color := bytes.Buffer{}
	err = p.tmpls.ExecuteTemplate(&color, "color", &hm.Data)
	if err != nil {
		glog.Infof("error expanding template, ", err.Error())
		return
	}

	text := bytes.Buffer{}
	err = p.tmpls.ExecuteTemplate(&text, "text", &hm.Data)
	if err != nil {
		glog.Infof("error expanding template, ", err.Error())
		return
	}

	glog.Infof("channel: %s\n", channel.String())

	m := hugot.Message{}
	m.Channel = channel.String()
	m.Attachments = []hugot.Attachment{
		{
			Title:     title.String(),
			TitleLink: titleLink.String(),
			Color:     color.String(),
			Text:      text.String(),
		},
	}
	rw.Send(context.TODO(), &m)
}

// Alert holds one alert for notification templates.
type alert struct {
	Labels       KV        `json:"labels"`
	Annotations  KV        `json:"annotations"`
	StartsAt     time.Time `json:"startsAt"`
	EndsAt       time.Time `json:"endsAt"`
	GeneratorURL string    `json:"generatorURL"`
	Silenced     string    `json:"silenced"`
	Inhibited    bool      `json:"inhibited"`
}

func modelToLocal(as []am.Alert) alerts {
	las := alerts{}
	for _, a := range as {
		la := alert{}
		la.StartsAt = a.StartsAt
		la.EndsAt = a.EndsAt
		la.GeneratorURL = a.GeneratorURL
		la.Labels = KV{}
		la.Silenced = a.Silenced
		la.Inhibited = a.Inhibited
		for k, v := range a.Labels {
			la.Labels[string(k)] = string(v)
		}
		la.Annotations = KV{}
		for k, v := range a.Annotations {
			la.Annotations[string(k)] = string(v)
		}
		las = append(las, la)
	}

	return las
}

// alerts is a list of Alert objects.
type alerts []alert

// Resolved returns true iff the activity interval ended in the past.
func (a *alert) Resolved() bool {
	return a.ResolvedAt(time.Now())
}

// ResolvedAt returns true off the activity interval ended before
// the given timestamp.
func (a *alert) ResolvedAt(ts time.Time) bool {
	if a.EndsAt.IsZero() {
		return false
	}
	return !a.EndsAt.After(ts)
}

// Status returns the status of the alert.
func (a *alert) Status() string {
	if a.Resolved() {
		return string(model.AlertResolved)
	}
	return string(model.AlertFiring)
}

// Firing returns the subset of alerts that are firing.
func (as alerts) Firing() []alert {
	res := []alert{}
	for _, a := range as {
		if a.Status() == string(model.AlertFiring) {
			res = append(res, a)
		}
	}
	return res
}

func (as alerts) Status() string {
	if len(as.Firing()) > 0 {
		return string(model.AlertFiring)
	}
	return string(model.AlertResolved)
}

// Resolved returns the subset of alerts that are resolved.
func (as alerts) Resolved() []alert {
	res := []alert{}
	for _, a := range as {
		if a.Status() == string(model.AlertResolved) {
			res = append(res, a)
		}
	}
	return res
}

// Pair is a key/value string pair.
type Pair struct {
	Name, Value string
}

// Pairs is a list of key/value string pairs.
type Pairs []Pair

// Names returns a list of names of the pairs.
func (ps Pairs) Names() []string {
	ns := make([]string, 0, len(ps))
	for _, p := range ps {
		ns = append(ns, p.Name)
	}
	return ns
}

// Values returns a list of values of the pairs.
func (ps Pairs) Values() []string {
	vs := make([]string, 0, len(ps))
	for _, p := range ps {
		vs = append(vs, p.Value)
	}
	return vs
}

// KV is a set of key/value string pairs.
type KV map[string]string

// SortedPairs returns a sorted list of key/value pairs.
func (kv KV) SortedPairs() Pairs {
	var (
		pairs     = make([]Pair, 0, len(kv))
		keys      = make([]string, 0, len(kv))
		sortStart = 0
	)
	for k := range kv {
		if k == string(model.AlertNameLabel) {
			keys = append([]string{k}, keys...)
			sortStart = 1
		} else {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys[sortStart:])

	for _, k := range keys {
		pairs = append(pairs, Pair{k, kv[k]})
	}
	return pairs
}

// Remove returns a copy of the key/value set without the given keys.
func (kv KV) Remove(keys []string) KV {
	keySet := make(map[string]struct{}, len(keys))
	for _, k := range keys {
		keySet[k] = struct{}{}
	}

	res := KV{}
	for k, v := range kv {
		if _, ok := keySet[k]; !ok {
			res[k] = v
		}
	}
	return res
}

// Names returns the names of the label names in the LabelSet.
func (kv KV) Names() []string {
	return kv.SortedPairs().Names()
}

// Values returns a list of the values in the LabelSet.
func (kv KV) Values() []string {
	return kv.SortedPairs().Values()
}

func (kv KV) clone() KV {
	res := KV{}
	for k, v := range kv {
		res[k] = v
	}
	return res
}

type tmplData struct {
	Receiver          string
	Status            string
	Alerts            alerts
	GroupLabels       KV
	CommonLabels      KV
	CommonAnnotations KV
	ExternalURL       string
}

func data(recv, extURL string, groupLabels model.LabelSet, as alerts) *tmplData {
	data := &tmplData{
		Receiver:          strings.SplitN(recv, "/", 2)[0],
		Status:            as.Status(),
		Alerts:            as,
		GroupLabels:       map[string]string{},
		CommonLabels:      map[string]string{},
		CommonAnnotations: map[string]string{},
		ExternalURL:       extURL,
	}

	for k, v := range groupLabels {
		data.GroupLabels[string(k)] = string(v)
	}

	if len(as) >= 1 {
		var (
			commonLabels      = as[0].Labels.clone()
			commonAnnotations = as[0].Annotations.clone()
		)
		for _, a := range as[1:] {
			for ln, lv := range commonLabels {
				if a.Labels[ln] != lv {
					delete(commonLabels, ln)
				}
			}
			for an, av := range commonAnnotations {
				if a.Annotations[an] != av {
					delete(commonAnnotations, an)
				}
			}
		}
		for k, v := range commonLabels {
			data.CommonLabels[string(k)] = string(v)
		}
		for k, v := range commonAnnotations {
			data.CommonAnnotations[string(k)] = string(v)
		}
	}

	return data
}

func (p *promH) alertMessage(d interface{}) (*hugot.Message, error) {
	channel := bytes.Buffer{}
	err := p.tmpls.ExecuteTemplate(&channel, "channel", d)
	if err != nil {
		return nil, err
	}

	title := bytes.Buffer{}
	err = p.tmpls.ExecuteTemplate(&title, "title", d)
	if err != nil {
		return nil, err
	}

	titleLink := bytes.Buffer{}
	err = p.tmpls.ExecuteTemplate(&titleLink, "title_link", d)
	if err != nil {
		return nil, err
	}

	color := bytes.Buffer{}
	err = p.tmpls.ExecuteTemplate(&color, "color", d)
	if err != nil {
		return nil, err
	}

	text := bytes.Buffer{}
	err = p.tmpls.ExecuteTemplate(&text, "text", d)
	if err != nil {
		return nil, err
	}

	m := hugot.Message{}
	m.Channel = channel.String()
	m.Attachments = []hugot.Attachment{
		{
			Title:     title.String(),
			TitleLink: titleLink.String(),
			Color:     color.String(),
			Text:      text.String(),
		},
	}
	return &m, nil
}
