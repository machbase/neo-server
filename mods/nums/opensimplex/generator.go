package opensimplex

type Generator struct {
	base *noise
}

// OpenSimplex noise generator
func New(seed int64) *Generator {
	s := &noise{
		perm:            make([]int16, 256),
		permGradIndex3D: make([]int16, 256),
	}

	source := make([]int16, 256)
	for i := range source {
		source[i] = int16(i)
	}

	seed = seed*6364136223846793005 + 1442695040888963407
	seed = seed*6364136223846793005 + 1442695040888963407
	seed = seed*6364136223846793005 + 1442695040888963407
	for i := int32(255); i >= 0; i-- {
		seed = seed*6364136223846793005 + 1442695040888963407
		r := int32((seed + 31) % int64(i+1))
		if r < 0 {
			r += i + 1
		}

		s.perm[i] = source[r]
		s.permGradIndex3D[i] = (s.perm[i] % (int16(len(gradients3D)) / 3)) * 3
		source[r] = source[i]
	}

	return &Generator{
		base: s,
	}
}

func (g *Generator) Eval(dim ...float64) float64 {
	switch len(dim) {
	case 1:
		return g.base.Eval2(dim[0], dim[0])
	case 2:
		return g.base.Eval2(dim[0], dim[1])
	case 3:
		return g.base.Eval3(dim[0], dim[1], dim[2])
	case 4:
		return g.base.Eval4(dim[0], dim[1], dim[2], dim[3])
	}
	return 0
}

func (g *Generator) Eval32(dim ...float32) float32 {
	switch len(dim) {
	case 1:
		return float32(g.base.Eval2(float64(dim[0]), float64(dim[0])))
	case 2:
		return float32(g.base.Eval2(float64(dim[0]), float64(dim[1])))
	case 3:
		return float32(g.base.Eval3(float64(dim[0]), float64(dim[1]), float64(dim[2])))
	case 4:
		return float32(g.base.Eval4(float64(dim[0]), float64(dim[1]), float64(dim[2]), float64(dim[3])))
	}
	return 0
}

const (
	// The normMin and normScale constants are used
	// in the formula for normalizing the raw output
	// of the OpenSimplex algorithm. They were
	// derived from empirical observations of the
	// range of raw values. Different constants are
	// required for each of Eval2, Eval3, and Eval4.
	normMin2   = 0.8659203878240322
	normScale2 = 0.577420288914181

	normMin3   = 0.9871048542519545
	normScale3 = 0.506595297177236

	normMin4   = 1.0040848236330158
	normScale4 = 0.5007450643319374
)

// EvalNormalize returns a random noise value in the range [0, 1)
func (g *Generator) EvalNormalize(dim ...float64) float64 {
	r := g.Eval(dim...)
	switch len(dim) {
	case 1:
		r = (r + normMin2) * normScale2
	case 2:
		r = (r + normMin2) * normScale2
	case 3:
		r = (r + normMin3) * normScale3
	case 4:
		r = (r + normMin4) * normScale4
	}
	return r
}

// Eval32Normalize returns a random noise value in the range [0, 1)
func (g *Generator) Eval32Normalize(dim ...float32) float32 {
	r := g.Eval32(dim...)
	switch len(dim) {
	case 1:
		r = (r + normMin2) * normScale2
	case 2:
		r = (r + normMin2) * normScale2
	case 3:
		r = (r + normMin3) * normScale3
	case 4:
		r = (r + normMin4) * normScale4
	}

	if r >= 1.0 {
		// depends on the platform casting float32 from float64,
		// may produce a value of 1.0
		return float32(0.9999999999)
	}
	return r
}
