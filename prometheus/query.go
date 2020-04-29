package prometheus

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/golang/glog"
	"github.com/tcolgate/hugot"
	"github.com/tcolgate/hugot/handlers/command"
	"github.com/vdobler/chart"
	"github.com/vdobler/chart/imgg"

	prom "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

func (p *promH) graphCmd(root *command.Command, defGraph bool) {
	cmd := &command.Command{
		Use:   "graph",
		Short: "render simple graphs from prometheus queries",
	}

	text := cmd.Flags().BoolP("text", "t", false, "Render the graphs as text sparkline.")
	dur := cmd.Flags().DurationP("duration", "d", 15*time.Minute, "how far back to render")
	cmd.Run = func(ctx context.Context, w hugot.ResponseWriter, m *hugot.Message, args []string) error {
		if len(args) == 0 {
			return fmt.Errorf("you need to give a query")
		}
		q := strings.Join(args, " ")
		s := time.Now().Add(-1 * *dur)
		e := time.Now()

		if !*text {
			nu := *p.wh.URL()
			nu.Path = nu.Path + "graph/thing.png"
			qs := nu.Query()
			qs.Set("q", q)
			qs.Set("s", fmt.Sprintf("%d", s.Unix()))
			qs.Set("e", fmt.Sprintf("%d", e.Unix()))
			nu.RawQuery = qs.Encode()

			m := hugot.Message{
				Channel: m.Channel,
				Attachments: []hugot.Attachment{
					{
						Fallback: "fallback",
						ImageURL: nu.String(),
					},
				},
			}
			w.Send(ctx, &m)
			return nil
		}

		qapi := prom.NewAPI(p.client)
		d, _, err := qapi.QueryRange(ctx, q, prom.Range{
			Start: s,
			End:   e,
			Step:  1 * time.Second,
		})
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

	root.AddCommand(cmd)
}

func maxMin(ss []model.SamplePair) (float64, float64) {
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
	max, min := maxMin(ss)
	if max == 0 {
		max = 1
	}
	out := make([]float64, len(ss))
	for i, s := range ss {
		out[i] = (float64(s.Value) - min) / (max - min)
	}
	return out
}

const lineLen = 40

func line(ss []model.SamplePair) string {
	sls := []rune("▁▂▃▄▅▆▇█")
	if len(ss) == 0 {
		return ""
	}
	down := lttb(ss, 40)
	norm := normalize(down)
	out := bytes.Buffer{}
	for _, n := range norm {
		i := float64(n) * float64(len(sls)-1)
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

	qapi := prom.NewAPI(p.client)
	d, _, err := qapi.QueryRange(ctx, q[0], prom.Range{
		Start: time.Unix(int64(st), 0),
		End:   time.Unix(int64(et), 0),
		Step:  1 * time.Second,
	})
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

	img := plot(q[0], mx)

	w.Header().Set("Content-Type", "image/png")
	png.Encode(w, img)
}

func modelToPlot(sps []model.SamplePair) []chart.EPoint {
	pts := []chart.EPoint{}
	for i := range sps {
		ep := chart.EPoint{X: float64(sps[i].Timestamp / 1000), Y: float64(sps[i].Value)}
		pts = append(pts, ep)
	}
	return pts
}

const (
	width  = 800
	height = 300
)

func plot(title string, mx model.Matrix) *image.RGBA {
	tdc := chart.ScatterChart{Title: title}

	tdc.XRange.Time, tdc.YRange.Time = true, false
	tdc.XRange.MinMode.Expand = chart.ExpandTight
	tdc.XRange.MaxMode.Expand = chart.ExpandTight

	for i, sps := range mx {
		dt := modelToPlot(lttb(sps.Values, width))
		tdc.AddData(sps.Metric.String(), dt, chart.PlotStyleLines, chart.AutoStyle(i, false))
	}

	tdc.Key.Pos = "ibr"

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	igr := imgg.AddTo(img, 0, 0, width, height, color.RGBA{0xff, 0xff, 0xff, 0xff}, nil, nil)

	tdc.Plot(igr)

	return img
}
