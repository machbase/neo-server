package opensimplex

import (
	"compress/gzip"
	"encoding/json"
	"io"
	"math"
	"os"
	"path"
	"testing"
	"time"
)

func loadSamples() <-chan []float64 {
	c := make(chan []float64)
	go func() {
		f, err := os.Open(path.Join("test", "samples.json.gz"))
		if err != nil {
			panic(err.Error())
		}
		defer f.Close()

		gz, err := gzip.NewReader(f)
		if err != nil {
			panic(err.Error())
		}

		dec := json.NewDecoder(gz)
		for {
			var sample []float64
			if err := dec.Decode(&sample); err == io.EOF {
				break
			} else if err != nil {
				panic(err.Error())
			} else {
				c <- sample
			}
		}
		close(c)
	}()

	return c
}

func TestSamples(t *testing.T) {
	samples := loadSamples()
	n := New(0)

	for s := range samples {
		var expected, actual float64
		switch len(s) {
		case 3:
			expected = s[2]
			actual = n.Eval(s[0], s[1])
		case 4:
			expected = s[3]
			actual = n.Eval(s[0], s[1], s[2])
		case 5:
			expected = s[4]
			actual = n.Eval(s[0], s[1], s[2], s[3])
		default:
			t.Fatalf("Unexpected size sample: %d", len(s))
		}

		tolerant := math.Pow(0.1, 12)
		if expected-actual > tolerant {
			t.Fatalf("Expected %v, got %v for %dD sample at %v",
				expected, actual, len(s)-1, s[:len(s)-1])
		}
	}
}

func TestTimeDomain(t *testing.T) {
	n := New(0)

	for i := 0; i < 10; i++ {
		v := n.Eval(float64(time.Now().Nanosecond()))
		t.Logf("==>%v", v)
	}
}
