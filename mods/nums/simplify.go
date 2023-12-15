package nums

// Ram-Douglas-Peucker simplify
func SimplifyPath(points []Point, ep float64) []Point {
	if len(points) <= 2 {
		return points
	}

	l := Line{Start: points[0], End: points[len(points)-1]}

	idx, maxDist := l.SeekMostDistant(points)
	if maxDist >= ep {
		left := SimplifyPath(points[:idx+1], ep)
		right := SimplifyPath(points[idx:], ep)
		return append(left[:len(left)-1], right...)
	}

	return []Point{points[0], points[len(points)-1]}
}
