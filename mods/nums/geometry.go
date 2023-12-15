package nums

import (
	"fmt"
	"math"

	"github.com/paulmach/orb"
)

// A Point is a Lng/Lat 2d point.
type Point orb.Point

func (p Point) String() string {
	return fmt.Sprintf("[%v,%v]", p[0], p[1])
}

type Line struct {
	Start Point
	End   Point
}

// Cartesian distance
func (l Line) DistanceTo(p Point) float64 {
	a, b, c := l.Coefficients()
	return math.Abs(a*p[0]+b*p[1]+c) / math.Sqrt(a*a+b*b)
}

// returns the three coefficients that define a line
// A line can be defined by following equation.
//
// ax + by + c = 0
func (l Line) Coefficients() (a, b, c float64) {
	a = l.Start[1] - l.End[1]
	b = l.End[0] - l.Start[0]
	c = l.Start[0]*l.End[1] - l.End[0]*l.Start[1]
	return a, b, c
}

func (l Line) SeekMostDistant(points []Point) (idx int, maxDist float64) {
	for i, p := range points {
		d := l.DistanceTo(p)
		if d > maxDist {
			maxDist = d
			idx = i
		}
	}
	return
}
