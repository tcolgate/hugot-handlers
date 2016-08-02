package prometheus

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"context"

	"github.com/golang/freetype"
	"github.com/golang/glog"
	"github.com/tcolgate/hugot"

	"github.com/prometheus/common/model"
	prom "github.com/tcolgate/client_golang/api/prometheus"

	"github.com/gonum/plot"
	"github.com/gonum/plot/plotter"
	"github.com/gonum/plot/plotutil"
	"github.com/gonum/plot/vg"
	"github.com/gonum/plot/vg/fonts"
)

func Init() {
	fontbytes, err := fonts.Asset("LiberationSans-Regular.ttf")
	if err != nil {
		panic(err)
	}
	f, err := freetype.ParseFont(fontbytes)
	if err != nil {
		panic(err)
	}
	vg.AddFont("Helvetica", f)
	vg.AddFont("LiberationSans-Regular", f)
	vg.AddFont("LiberationSans-Regular.ttf", f)

	plot.DefaultFont = "Helvetica"
	plotter.DefaultFont = "Helvetica"
}

func (p *promH) graphCmd(ctx context.Context, w hugot.ResponseWriter, m *hugot.Message) error {
	defText, defGraph := false, false
	if !hugot.IsTextOnly(w) {
		defText = true
		defGraph = false
	} else {
		defText = false
		defGraph = true
	}

	_ = m.Bool("t", defText, "Render the graphs as text sparkline.")
	webGraph := m.Bool("g", defGraph, "Render the graphs as a png.")
	dur := m.Duration("d", 15*time.Minute, "how far back to render")

	if err := m.Parse(); err != nil {
		return err
	}

	if len(m.Args()) == 0 {
		return fmt.Errorf("you need to give a query")
	}
	q := strings.Join(m.Args(), " ")
	s := time.Now().Add(-1 * *dur)
	e := time.Now()

	if *webGraph {
		nu := *p.URL()
		nu.Path = nu.Path + "graph/thing.png"
		qs := nu.Query()
		qs.Set("q", q)
		qs.Set("s", fmt.Sprintf("%d", s.UnixNano()))
		qs.Set("e", fmt.Sprintf("%d", e.UnixNano()))
		nu.RawQuery = qs.Encode()

		m := hugot.Message{
			Channel: m.Channel,
			Attachments: []hugot.Attachment{
				{
					Fallback: "fallback",
					Pretext:  "image",
					ImageURL: nu.String(),
				},
			},
		}
		w.Send(ctx, &m)
		return nil
	}

	qapi := prom.NewQueryAPI(*p.client)
	d, err := qapi.QueryRange(ctx, q, prom.Range{s, e, 15 * time.Second})
	if err != nil {
		return err
	}

	switch d.Type() {
	case model.ValScalar:
		m := d.(*model.Scalar)
		glog.Infof("scalar %v", m)
	case model.ValVector:
		m := d.(model.Vector)
		glog.Infof("vector %v", m)
	case model.ValMatrix:
		mx := d.(model.Matrix)
		sort.Sort(mx)
		for _, ss := range mx {
			l := line(ss.Values)
			fmt.Fprintf(w, "%v\n%s", ss.Metric, l)
		}
	case model.ValString:
		m := d.(*model.String)
		glog.Infof("matrix %v", m)
	case model.ValNone:
	}

	return nil
}

func max_min(ss []model.SamplePair) (float64, float64) {
	max := math.Inf(-1)
	min := math.Inf(1)
	for _, s := range ss {
		if float64(s.Value) > max {
			max = float64(s.Value)
		}
		if float64(s.Value) < min {
			min = float64(s.Value)
		}
	}
	return max, min
}

func normalize(ss []model.SamplePair) []float64 {
	max, min := max_min(ss)
	if max == 0 {
		max = 1
	}
	out := make([]float64, len(ss))
	for i, s := range ss {
		out[i] = (float64(s.Value) - min) / (max - min)
	}
	return out
}

func line(ss []model.SamplePair) string {
	sls := []rune("▁▂▃▄▅▆▇█")
	if len(ss) == 0 {
		return ""
	}
	norm := normalize(ss)
	out := bytes.Buffer{}
	for _, n := range norm {
		i := n * float64(len(sls)-1)
		if i < 0 {
			i = 0
		}
		if i >= float64(len(sls)) {
			i = float64(len(sls)) - 1
		}
		out.WriteRune(sls[int(i)])
	}
	return out.String()
}

func (p *promH) graphHook(w http.ResponseWriter, r *http.Request) {
	q, ok := r.URL.Query()["q"]
	if !ok || len(q) != 1 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	s, ok := r.URL.Query()["s"]
	if !ok || len(s) != 1 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	e, ok := r.URL.Query()["e"]
	if !ok || len(e) != 1 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	st, _ := strconv.Atoi(s[0])
	et, _ := strconv.Atoi(e[0])

	ctx := r.Context()

	qapi := prom.NewQueryAPI(*p.client)
	d, err := qapi.QueryRange(ctx, q[0], prom.Range{time.Unix(0, int64(st)), time.Unix(0, int64(et)), 15 * time.Second})
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	var mx model.Matrix
	switch d.Type() {
	case model.ValMatrix:
		mx = d.(model.Matrix)
		sort.Sort(mx)
	default:
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	plt, err := plot.New()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	plt.Title.Text = q[0]
	plt.X.Tick.Marker = plot.UnixTimeTicks{Format: "Mon 15:04:05"}

	for _, ss := range mx {
		for _, sps := range mx {
			err = plotutil.AddLinePoints(plt, ss.Metric.String(), modelToPlot(sps.Values))
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		}
	}

	// Save the plot to a PNG file.
	var wt io.WriterTo
	if wt, err = plt.WriterTo(4*vg.Inch, 2*vg.Inch, "png"); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "image/png")
	if _, err := wt.WriteTo(w); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func modelToPlot(sps []model.SamplePair) plotter.XYs {
	pts := make(plotter.XYs, len(sps))
	for i := range pts {
		pts[i].X = float64(float64(sps[i].Timestamp) / 1000.0)
		pts[i].Y = float64(sps[i].Value)
	}
	return pts
}
