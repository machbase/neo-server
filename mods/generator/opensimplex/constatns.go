package opensimplex

const (
	stretchConstant2D = -0.211324865405187 // (1/Math.sqrt(2+1)-1)/2
	squishConstant2D  = 0.366025403784439  // (Math.sqrt(2+1)-1)/2
	stretchConstant3D = -1.0 / 6           // (1/Math.sqrt(3+1)-1)/3
	squishConstant3D  = 1.0 / 3            // (Math.sqrt(3+1)-1)/3
	stretchConstant4D = -0.138196601125011 // (1/Math.sqrt(4+1)-1)/4
	squishConstant4D  = 0.309016994374947  // (Math.sqrt(4+1)-1)/4

	normConstant2D = 47
	normConstant3D = 103
	normConstant4D = 30
)

func (s *noise) extrapolate2(xsb, ysb int32, dx, dy float64) float64 {
	index := s.perm[(int32(s.perm[xsb&0xFF])+ysb)&0xFF] & 0x0E
	return float64(gradients2D[index])*dx + float64(gradients2D[index+1])*dy
}

func (s *noise) extrapolate3(xsb, ysb, zsb int32, dx, dy, dz float64) float64 {
	index := s.permGradIndex3D[(int32(s.perm[(int32(s.perm[xsb&0xFF])+ysb)&0xFF])+zsb)&0xFF]
	return float64(gradients3D[index])*dx + float64(gradients3D[index+1])*dy + float64(gradients3D[index+2])*dz
}

func (s *noise) extrapolate4(xsb, ysb, zsb, wsb int32, dx, dy, dz, dw float64) float64 {
	index := s.perm[(int32(s.perm[(int32(s.perm[(int32(s.perm[xsb&0xFF])+ysb)&0xFF])+zsb)&0xFF])+wsb)&0xFF] & 0xFC
	return float64(gradients4D[index])*dx + float64(gradients4D[index+1])*dy + float64(gradients4D[index+2])*dz + float64(gradients4D[index+3])*dw
}

// Gradients for 2D. They approximate the directions to the
// vertices of an octagon from the center.
var gradients2D = []int8{
	5, 2, 2, 5,
	-5, 2, -2, 5,
	5, -2, 2, -5,
	-5, -2, -2, -5,
}

// Gradients for 3D. They approximate the directions to the
// vertices of a rhombicuboctahedron from the center, skewed so
// that the triangular and square facets can be inscribed inside
// circles of the same radius.
var gradients3D = []int8{
	-11, 4, 4, -4, 11, 4, -4, 4, 11,
	11, 4, 4, 4, 11, 4, 4, 4, 11,
	-11, -4, 4, -4, -11, 4, -4, -4, 11,
	11, -4, 4, 4, -11, 4, 4, -4, 11,
	-11, 4, -4, -4, 11, -4, -4, 4, -11,
	11, 4, -4, 4, 11, -4, 4, 4, -11,
	-11, -4, -4, -4, -11, -4, -4, -4, -11,
	11, -4, -4, 4, -11, -4, 4, -4, -11,
}

// Gradients for 4D. They approximate the directions to the
// vertices of a disprismatotesseractihexadecachoron from the center,
// skewed so that the tetrahedral and cubic facets can be inscribed inside
// spheres of the same radius.
var gradients4D = []int8{
	3, 1, 1, 1, 1, 3, 1, 1, 1, 1, 3, 1, 1, 1, 1, 3,
	-3, 1, 1, 1, -1, 3, 1, 1, -1, 1, 3, 1, -1, 1, 1, 3,
	3, -1, 1, 1, 1, -3, 1, 1, 1, -1, 3, 1, 1, -1, 1, 3,
	-3, -1, 1, 1, -1, -3, 1, 1, -1, -1, 3, 1, -1, -1, 1, 3,
	3, 1, -1, 1, 1, 3, -1, 1, 1, 1, -3, 1, 1, 1, -1, 3,
	-3, 1, -1, 1, -1, 3, -1, 1, -1, 1, -3, 1, -1, 1, -1, 3,
	3, -1, -1, 1, 1, -3, -1, 1, 1, -1, -3, 1, 1, -1, -1, 3,
	-3, -1, -1, 1, -1, -3, -1, 1, -1, -1, -3, 1, -1, -1, -1, 3,
	3, 1, 1, -1, 1, 3, 1, -1, 1, 1, 3, -1, 1, 1, 1, -3,
	-3, 1, 1, -1, -1, 3, 1, -1, -1, 1, 3, -1, -1, 1, 1, -3,
	3, -1, 1, -1, 1, -3, 1, -1, 1, -1, 3, -1, 1, -1, 1, -3,
	-3, -1, 1, -1, -1, -3, 1, -1, -1, -1, 3, -1, -1, -1, 1, -3,
	3, 1, -1, -1, 1, 3, -1, -1, 1, 1, -3, -1, 1, 1, -1, -3,
	-3, 1, -1, -1, -1, 3, -1, -1, -1, 1, -3, -1, -1, 1, -1, -3,
	3, -1, -1, -1, 1, -3, -1, -1, 1, -1, -3, -1, 1, -1, -1, -3,
	-3, -1, -1, -1, -1, -3, -1, -1, -1, -1, -3, -1, -1, -1, -1, -3,
}
