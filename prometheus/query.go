package prometheus

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"

	"context"

	"github.com/golang/freetype"
	"github.com/golang/glog"
	"github.com/tcolgate/hugot"

	prom "github.com/prometheus/client_golang/api/prometheus"
	"github.com/prometheus/common/model"

	"math/rand"

	"github.com/gonum/plot"
	"github.com/gonum/plot/plotter"
	"github.com/gonum/plot/plotutil"
	"github.com/gonum/plot/vg"
	"github.com/gonum/plot/vg/fonts"
)

func init() {
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

	if *webGraph {
		nu := *p.URL()
		nu.Path = nu.Path + "graph"
		qs := nu.Query()
		qs.Set("q", q)
		nu.RawQuery = qs.Encode()

		fmt.Fprint(w, nu.String())
		return nil
	}

	qapi := prom.NewQueryAPI(*p.client)
	d, err := qapi.QueryRange(ctx, q, prom.Range{time.Now().Add(-1 * *dur), time.Now(), 15 * time.Second})
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

func (*promH) graphHook(w http.ResponseWriter, r *http.Request) {
	//rw, ok := hugot.ResponseWriterFromContext(r.Context())
	//if !ok {
	//		http.NotFound(w, r)
	//	}

	rand.Seed(int64(0))

	p, err := plot.New()
	if err != nil {
		panic(err)
	}

	p.Title.Text = "Plotutil example"
	p.X.Label.Text = "X"
	p.Y.Label.Text = "Y"

	err = plotutil.AddLinePoints(p,
		"First", randomPoints(15),
		"Second", randomPoints(15),
		"Third", randomPoints(15))
	if err != nil {
		panic(err)
	}

	// Save the plot to a PNG file.
	var wt io.WriterTo
	if wt, err = p.WriterTo(4*vg.Inch, 2*vg.Inch, "png"); err != nil {
		panic(err)
	}

	w.Header().Set("Content-Type", "image/png")
	if _, err := wt.WriteTo(w); err != nil {
		log.Println("unable to write image.")
	}
}

// randomPoints returns some random x, y points.
func randomPoints(n int) plotter.XYs {
	pts := make(plotter.XYs, n)
	for i := range pts {
		if i == 0 {
			pts[i].X = rand.Float64()
		} else {
			pts[i].X = pts[i-1].X + rand.Float64()
		}
		pts[i].Y = pts[i].X + 10*rand.Float64()
	}
	return pts
}
