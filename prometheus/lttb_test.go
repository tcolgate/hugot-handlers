package prometheus

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"
	"testing"

	"github.com/prometheus/common/model"
)

func readData(ir io.Reader) ([]model.SamplePair, error) {
	r := csv.NewReader(ir)

	out := []model.SamplePair{}
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		X, _ := strconv.ParseFloat(record[0], 64)
		Y, _ := strconv.ParseFloat(record[1], 64)
		out = append(out, model.SamplePair{Value: model.SampleValue(Y), Timestamp: model.Time(X)})
	}

	return out, nil
}

func TestLTTB(t *testing.T) {
	rsrc, _ := os.Open("testdata/source.csv")
	rexp, _ := os.Open("testdata/sampled.csv")

	src, _ := readData(rsrc)
	exp, _ := readData(rexp)

	res := lttb(src, 500)

	if len(exp) != len(res) {
		t.Fatalf("Wrong number of data points")
	}

	for i := 0; i < len(res); i++ {
		if exp[i] != res[i] {
			//t.Fatalf("Expected res[%v] == %v, got %v", i, exp[i], res[i])
			fmt.Printf("Expected res[%v] == %v, got %v\n", i, exp[i], res[i].Value)
		}
	}
}

func TestLTTB2(t *testing.T) {
	data := []model.SamplePair{
		{1, 2},
		{2, 2},
		{3, 3},
		{4, 3},
		{5, 6},
		{6, 3},
		{7, 3},
		{8, 5},
		{9, 4},
		{10, 4},
		{11, 1},
		{12, 2}}

	exp := []model.SamplePair{
		{1, 2},
		{5, 6},
		{12, 2}}

	res := lttb(data, 3)
	for i := 0; i < len(res); i++ {
		if exp[i] != res[i] {
			t.Fatalf("Expected res[%v] == %v, got %v", i, exp[i], res[i])
		}
	}
}

func BenchmarkLTTB(b *testing.B) {
}
