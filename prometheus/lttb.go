package prometheus

import (
	"math"

	"github.com/prometheus/common/model"
)

// lttb is an implementation of Largest-Triangle-Three-Buckets downsampling
// which atempts to preserve the visual repsentation of a time series
func lttb(ss []model.SamplePair, t int) []model.SamplePair {
	sampled := []model.SamplePair{}
	if t >= len(ss) || t == 0 {
		return ss
	}

	// Bucket size. Leave room for start and end data points
	bsize := float64((len(ss) - 2)) / float64(t-2)
	a, nexta := 0, 0

	sampled = append(sampled, ss[0])

	for i := 0; i < t-2; i++ {
		avgRangeStart := (int)(math.Floor((float64(i+1) * bsize)) + 1)
		avgRangeEnd := (int)(math.Floor((float64(i+2))*bsize) + 1)

		if avgRangeEnd >= len(ss) {
			avgRangeEnd = len(ss)
		}

		avgRangeLength := (avgRangeEnd - avgRangeStart)

		avgX, avgY := 0.0, 0.0

		for {
			if avgRangeStart >= avgRangeEnd {
				break
			}
			avgX += float64(ss[avgRangeStart].Timestamp)
			avgY += float64(ss[avgRangeStart].Value)
			avgRangeStart++
		}

		avgX /= float64(avgRangeLength)
		avgY /= float64(avgRangeLength)

		rangeOffs := (int)(math.Floor((float64(i)+0)*bsize) + 1)
		rangeTo := (int)(math.Floor((float64(i)+1)*bsize) + 1)

		pointAx := float64(ss[a].Timestamp)
		pointAy := float64(ss[a].Value)

		maxArea := -1.0

		var maxAreaPoint model.SamplePair

		for {
			if rangeOffs >= rangeTo {
				break
			}

			area := math.Abs((pointAx-avgX)*(float64(ss[rangeOffs].Value)-pointAy)-(pointAx-float64(ss[rangeOffs].Timestamp))*(avgY-pointAy)) * 0.5

			if area > maxArea {
				maxArea = area
				maxAreaPoint = ss[rangeOffs]
				nexta = rangeOffs
			}
			rangeOffs++
		}

		sampled = append(sampled, maxAreaPoint)
		a = nexta
	}

	sampled = append(sampled, ss[len(ss)-1])

	return sampled
}
