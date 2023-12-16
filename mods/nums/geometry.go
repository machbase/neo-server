package nums

import (
	"fmt"
	"math"
)

// DegreesToRadians converts from degrees to radians.
func DegreesToRadians(d float64) float64 {
	return d * math.Pi / 180
}

// A Point is a Lng/Lat 2d point.
type Point [2]float64

func (p Point) X() float64      { return p[0] }
func (p Point) Y() float64      { return p[1] }
func (p Point) Lat() float64    { return p[1] }
func (p Point) Lon() float64    { return p[0] }
func (p Point) Dimensions() int { return 0 }

// Equal returns if two points are equal.
func (p Point) Equal(o Point) bool {
	return p[0] == o[0] && p[1] == o[1]
}

func (p Point) String() string {
	return fmt.Sprintf("[%v,%v]", p[0], p[1])
}

type Line struct {
	Start Point
	End   Point
}

func (l Line) Dimensions() int { return 1 }

// Cartesian distance
func (l Line) DistanceTo(p Point) float64 {
	a, b, c := l.Coefficients()
	return math.Abs(a*p.X()+b*p.Y()+c) / math.Sqrt(a*a+b*b)
}

// returns the three coefficients that define a line
// A line can be defined by following equation.
//
// ax + by + c = 0
func (l Line) Coefficients() (a, b, c float64) {
	a = l.Start.Y() - l.End.Y()
	b = l.End.X() - l.Start.X()
	c = l.Start.X()*l.End.Y() - l.End.X()*l.Start.Y()
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
