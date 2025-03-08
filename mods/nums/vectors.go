package nums

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
)

type Scalar interface {
	~int8 | ~int16 | ~int32 | ~int64 | ~float32 | ~float64
}

type Vector[S Scalar] []S

func (v Vector[S]) Dimension() int {
	return len(v)
}

func (v Vector[S]) Symbol() (ty byte, sz int) {
	var s any = v[0]
	switch x := s.(type) {
	case int8:
		ty, sz = 'o', 1
	case int16:
		ty, sz = 'h', 2
	case int32:
		ty, sz = 'i', 4
	case int64:
		ty, sz = 'I', 8
	case float32:
		ty, sz = 'f', 4
	case float64:
		ty, sz = 'F', 8
	default:
		panic(fmt.Sprintf("invalid type: %T", x))
	}
	return
}

func (v Vector[S]) String() string {
	b, err := json.Marshal(v)
	if err != nil {
		return err.Error()
	}
	return string(b)
}

func (v Vector[S]) MarshalJSON() ([]byte, error) {
	buf := &bytes.Buffer{}
	buf.WriteByte('[')
	for i, s := range v {
		if i > 0 {
			buf.WriteByte(',')
		}
		buf.WriteString(fmt.Sprintf("%v", s))
	}
	buf.WriteByte(']')
	return buf.Bytes(), nil
}

func (v *Vector[S]) UnmarshalJSON(b []byte) error {
	val := []S{}
	if err := json.Unmarshal(b, &val); err != nil {
		return err
	}
	*v = Vector[S](val)
	return nil
}

// Marshal returns the binary representation of the vector.
// bits is the number of bits used to represent each float64 value.
// It returns an error if bits is not 32 or 64.
func (v Vector[S]) Marshal() ([]byte, error) {
	var sz int
	var ty byte
	ty, sz = v.Symbol()
	b := bytes.Buffer{}
	b.WriteByte(ty) // ty
	var buf [2]byte
	binary.BigEndian.PutUint16(buf[:], uint16(len(v))) // len
	b.Write(buf[:])
	for _, f := range v {
		var buf [8]byte
		switch ty {
		case 'o': // int8
			buf[0] = byte(int8(f))
		case 'h': // int16
			binary.BigEndian.PutUint16(buf[:], uint16(int16(f)))
		case 'i': // int32
			binary.BigEndian.PutUint32(buf[:], uint32(int32(f)))
		case 'I': // int64
			binary.BigEndian.PutUint64(buf[:], uint64(int64(f)))
		case 'f': // float32
			binary.BigEndian.PutUint32(buf[:], math.Float32bits(float32(f)))
		case 'F': // float64
			binary.BigEndian.PutUint64(buf[:], math.Float64bits(float64(f)))
		}
		b.Write(buf[0:sz])
	}
	return b.Bytes(), nil
}

func (v *Vector[S]) Unmarshal(b []byte) error {
	if len(b) < 3 {
		return fmt.Errorf("invalid vector data")
	}
	ty := byte(b[0])
	sz := 0
	switch ty {
	case 'o':
		sz = 1
	case 'h':
		sz = 2
	case 'i':
		sz = 4
	case 'I':
		sz = 8
	case 'f':
		sz = 4
	case 'F':
		sz = 8
	default:
		return fmt.Errorf("invalid vector type: %c", ty)
	}
	d := binary.BigEndian.Uint16(b[1:3])
	if len(b) < 3+int(d)*sz {
		return fmt.Errorf("invalid vector data")
	}
	*v = make(Vector[S], d)
	for i := 0; i < int(d); i++ {
		switch ty {
		case 'o': // int8
			(*v)[i] = S(b[3+i*sz])
		case 'h': // int16
			(*v)[i] = S(binary.BigEndian.Uint16(b[3+i*sz : 3+(i+1)*sz]))
		case 'i': // int32
			(*v)[i] = S(binary.BigEndian.Uint32(b[3+i*sz : 3+(i+1)*sz]))
		case 'I': // int64
			(*v)[i] = S(binary.BigEndian.Uint64(b[3+i*sz : 3+(i+1)*sz]))
		case 'f': // float32
			(*v)[i] = S(math.Float32frombits(binary.BigEndian.Uint32(b[3+i*sz : 3+(i+1)*sz])))
		case 'F': // float64
			(*v)[i] = S(math.Float64frombits(binary.BigEndian.Uint64(b[3+i*sz : 3+(i+1)*sz])))
		}
	}
	return nil
}
