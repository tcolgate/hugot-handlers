package prometheus

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"text/template"

	"github.com/Masterminds/sprig"
	"github.com/golang/glog"
	"github.com/prometheus/alertmanager/notify"
	"github.com/prometheus/common/model"
	am "github.com/tcolgate/client_golang/api/alertmanager"
	"github.com/tcolgate/hugot"
)

// These templates are stolen from Qubit's internal alertmanager configs and are rather
// opinionated, and by no means best practice. These should probably be configurable.
var (
	channelTmpl = template.Must(template.New("channel").Funcs(sprig.TxtFuncMap()).Parse(`{{ if .GroupLabels.slack_channel }}{{ .GroupLabels.slack_channel }}{{ else }}DEFAULTCHAN{{ end }}`))
	colorTmpl   = template.Must(template.New("color").Funcs(sprig.TxtFuncMap()).Parse(`{{ if eq .Status "firing" }}{{ if eq .GroupLabels.severity "page" }}danger{{else}}#ffa500{{end}}{{ else }}good{{ end }}`))
	textTmpl    = template.Must(template.New("text").Funcs(sprig.TxtFuncMap()).Parse(`{{$caRB := .CommonAnnotations.runbook_url}}{{$caDash := .CommonAnnotations.dashboard_url}}{{ range .Alerts.Firing }}{{ printf "%s" .Annotations.description }}{{if not $caDash}}{{ if .Annotations.dashboard_url }}{{printf " <%s|:thermometer:>"  .Annotations.dashboard_url }}{{end}}{{end}}{{if not $caRB}}{{ if .Annotations.runbook_url }}{{ printf "<%s|:clipboard:>" .Annotations.runbook_url }}{{end}}{{end}}{{ printf "\n"}}{{end}}{{ if eq .Status "firing" }} {{if $caRB }}<{{ $caRB }}#{{ lower .GroupLabels.alertname }}|:clipboard:>{{ end }}{{ if $caDash }} <{{ $caDash }}|:thermometer:>{{end}}{{end}}`))
	titleTmpl   = template.Must(template.New("title").Funcs(sprig.TxtFuncMap()).Parse(`[{{ .Status | upper }}{{ if eq .Status "firing" }}:{{ .Alerts.Firing | len }}{{ end }}] {{ .GroupLabels.alertname }} ({{ .GroupLabels.job }} in {{ .GroupLabels.cluster }})`))
)

func alertToMessage(a *model.Alert) *hugot.Message {
	var err error

	glog.Infof("ALERT DUMP %#v", *a)

	color := bytes.Buffer{}
	err = colorTmpl.Execute(&color, *a)
	if err != nil {
		glog.Infof("color template errors: ", err)
		return nil
	}

	text := bytes.Buffer{}
	err = textTmpl.Execute(&text, *a)
	if err != nil {
		glog.Infof("text template errors: ", err)
		return nil
	}

	title := bytes.Buffer{}
	err = titleTmpl.Execute(&title, *a)
	if err != nil {
		glog.Infof("title template errors: ", err)
		return nil
	}

	channel := bytes.Buffer{}
	err = channelTmpl.Execute(&channel, *a)
	if err != nil {
		glog.Infof("channel template errors: ", err)
		return nil
	}

	m := hugot.Message{
		Channel: channel.String(),
		Attachments: []hugot.Attachment{
			{
				Title: title.String(),
				Color: color.String(),
				Text:  text.String(),
			},
		},
	}

	return &m
}

func (p *promH) alertsCmd(ctx context.Context, w hugot.ResponseWriter, m *hugot.Message) error {
	if err := m.Parse(); err != nil {
		return err
	}
	as, err := am.NewAlertAPI(p.amclient).List(ctx)
	if err != nil {
		return err
	}

	if len(as) == 0 {
		fmt.Fprint(w, "There are no outstanding alerts")
		return nil
	}

	for _, a := range as {
		fmt.Fprintf(w, "%s: Started at %s, %#v, %#v", a.Labels["alertname"], a.StartsAt, a.Labels, a.Annotations)

		m := alertToMessage(a)
		if m != nil {
			w.Send(ctx, m)
		}
	}

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
	_, ok := hugot.ResponseWriterFromContext(r.Context())
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

	glog.Infof("Webhook dump: %#v", *hm.Data)
	//rw.SetChannel(p.alertChan)
	//status := strings.ToUpper(hm.Data.Status)
	//fmt.Fprintf(rw, "%s: %#v", status, hm.Data)
}
