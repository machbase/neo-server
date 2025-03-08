package nums

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
)

type Vector []float64

func (v Vector) Dimension() int {
	return len(v)
}

func (v Vector) String() string {
	vs, err := json.Marshal([]float64(v))
	if err != nil {
		return fmt.Sprintf("%v", []float64(v))
	}
	return string(vs)
}

func (v Vector) MarshalJSON() ([]byte, error) {
	vs, err := json.Marshal([]float64(v))
	if err != nil {
		return nil, err
	}
	return []byte(fmt.Sprintf(`{"d":%d,"v":%s}`, len(v), string(vs))), nil
}

func (v *Vector) UnmarshalJSON(b []byte) error {
	val := struct {
		D int       `json:"d"`
		V []float64 `json:"v"`
	}{}
	if err := json.Unmarshal(b, &val); err != nil {
		return err
	}

	if val.D != len(val.V) {
		return fmt.Errorf("dimension mismatch: dim:%d != len:%d", val.D, len(val.V))
	}
	*v = Vector(val.V)
	return nil
}

// Marshal returns the binary representation of the vector.
// bits is the number of bits used to represent each float64 value.
// It returns an error if bits is not 32 or 64.
func (v Vector) Marshal(bits int) ([]byte, error) {
	if bits != 32 && bits != 64 {
		return nil, fmt.Errorf("invalid bits: %d", bits)
	}
	sz := bits / 8
	b := bytes.Buffer{}
	b.WriteByte(byte(sz))
	var buf [2]byte
	binary.BigEndian.PutUint16(buf[:], uint16(len(v)))
	b.Write(buf[:])
	for _, f := range v {
		var buf [8]byte
		if sz == 4 {
			binary.BigEndian.PutUint32(buf[:], math.Float32bits(float32(f)))
		} else {
			binary.BigEndian.PutUint64(buf[:], math.Float64bits(f))
		}
		b.Write(buf[0:sz])
	}
	return b.Bytes(), nil
}

func (v *Vector) Unmarshal(b []byte) error {
	if len(b) < 3 {
		return fmt.Errorf("invalid vector data")
	}
	sz := int(b[0])
	d := binary.BigEndian.Uint16(b[1:3])
	if len(b) < 3+int(d)*sz {
		return fmt.Errorf("invalid vector data")
	}
	*v = make(Vector, d)
	for i := 0; i < int(d); i++ {
		if sz == 4 {
			(*v)[i] = float64(math.Float32frombits(binary.BigEndian.Uint32(b[3+i*sz : 3+(i+1)*sz])))
		} else {
			(*v)[i] = math.Float64frombits(binary.BigEndian.Uint64(b[3+i*sz : 3+(i+1)*sz]))
		}
	}
	return nil
}
