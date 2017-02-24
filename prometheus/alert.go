package prometheus

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"

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
			d := data(b.Receiver, ls, model.Alerts(b.Alerts))
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

	glog.Infof("channel: %s\n", channel.String())

	m := hugot.Message{}
	m.Channel = channel.String()
	m.Attachments = []hugot.Attachment{
		{
			Title:     title.String(),
			TitleLink: titleLink.String(),
			Color:     color.String(),
		},
	}
	rw.Send(context.TODO(), &m)
}

type tmplData struct {
	Receiver          string
	Status            string
	Alerts            model.Alerts
	GroupLabels       map[string]string
	CommonLabels      map[string]string
	CommonAnnotations map[string]string
	ExternalURL       string
}

func data(recv string, groupLabels model.LabelSet, alerts model.Alerts) *tmplData {
	data := &tmplData{
		Receiver:          strings.SplitN(recv, "/", 2)[0],
		Status:            string(alerts.Status()),
		Alerts:            alerts,
		GroupLabels:       map[string]string{},
		CommonLabels:      map[string]string{},
		CommonAnnotations: map[string]string{},
		//		ExternalURL:       t.ExternalURL.String(),
	}

	for k, v := range groupLabels {
		data.GroupLabels[string(k)] = string(v)
	}

	if len(alerts) >= 1 {
		var (
			commonLabels      = alerts[0].Labels.Clone()
			commonAnnotations = alerts[0].Annotations.Clone()
		)
		for _, a := range alerts[1:] {
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

	m := hugot.Message{}
	m.Channel = channel.String()
	m.Attachments = []hugot.Attachment{
		{
			Title:     title.String(),
			TitleLink: titleLink.String(),
			Color:     color.String(),
		},
	}
	return &m, nil
}
