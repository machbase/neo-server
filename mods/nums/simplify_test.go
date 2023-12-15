package nums

import "testing"

func TestSimplifyPath(t *testing.T) {
	points := []Point{
		{0, 0},
		{1, 2},
		{2, 7},
		{3, 1},
		{4, 8},
		{5, 2},
		{6, 8},
		{7, 3},
		{8, 3},
		{9, 0},
	}

	t.Run("Threshold=0", func(t *testing.T) {
		if len(SimplifyPath(points, 0)) != 10 {
			t.Error("simplified path should have all points")
		}
	})

	t.Run("Threshold=2", func(t *testing.T) {
		if len(SimplifyPath(points, 2)) != 7 {
			t.Error("simplified path should only have 7 points")
		}
	})

	t.Run("Threshold=5", func(t *testing.T) {
		if len(SimplifyPath(points, 100)) != 2 {
			t.Error("simplified path should only have two points")
		}
	})
}

func TestSeekMostDistantPoint(t *testing.T) {
	l := Line{Start: Point{0, 0}, End: Point{0, 10}}
	points := []Point{
		{13, 13},
		{1, 15},
		{1, 1},
		{3, 6},
	}

	idx, maxDist := l.SeekMostDistant(points)

	if idx != 0 {
		t.Errorf("failed to find most distant point away from a line, got %d", idx)
	}

	if maxDist != 13 {
		t.Errorf("maximum distance is incorrect, got %f", maxDist)
	}
}
