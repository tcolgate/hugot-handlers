package prometheus

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/golang/glog"
	"github.com/prometheus/alertmanager/notify"
	am "github.com/prometheus/client_golang/api/alertmanager"
	"github.com/tcolgate/hugot"
)

func (p *promH) alertsCmd(ctx context.Context, w hugot.ResponseWriter, m *hugot.Message) error {
	if err := m.Parse(); err != nil {
		return err
	}
	as, err := am.NewAlertAPI(p.amclient).ListGroups(ctx)
	if err != nil {
		return err
	}

	if len(as) == 0 {
		fmt.Fprint(w, "There are no outstanding alerts")
		return nil
	}

	fmt.Fprintf(w, "%#v", as)
	return nil
}

func (p *promH) silencesCmd(ctx context.Context, w hugot.ResponseWriter, m *hugot.Message) error {
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
