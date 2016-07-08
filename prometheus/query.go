package prometheus

import (
	"bytes"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/golang/glog"
	"github.com/tcolgate/hugot"
	"golang.org/x/net/context"

	prom "github.com/prometheus/client_golang/api/prometheus"
	"github.com/prometheus/common/model"
)

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
	_ = m.Bool("g", defGraph, "Render the graphs as a png.")
	dur := m.Duration("d", 15*time.Minute, "how far back to render")
	if err := m.Parse(); err != nil {
		return err
	}

	if len(m.Args()) == 0 {
		return fmt.Errorf("you need to give a query")
	}

	qapi := prom.NewQueryAPI(*p.client)
	d, err := qapi.QueryRange(ctx, strings.Join(m.Args(), " "), prom.Range{time.Now().Add(-1 * *dur), time.Now(), 15 * time.Second})
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
		out.WriteRune(sls[int(i)])
	}
	return out.String()
}
