package nums_test

import (
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/machbase/neo-server/v8/mods/nums"
)

func ExampleVector() {
	v := nums.Vector[float64]{1.2, 3.4, 5.6}
	fmt.Println(v)

	// Output:
	// [1.2,3.4,5.6]
}

func ExampleVector_Dimension() {
	v := nums.Vector[float64]{1.2, 3.4, 5.6}
	fmt.Println(v.Dimension())

	// Output:
	// 3
}

func ExampleVector_MarshalJSON() {
	vFloat64 := nums.Vector[float64]{1.2, 3.4, 5.6}
	b, _ := vFloat64.MarshalJSON()
	fmt.Println("float64", string(b))

	vFloat32 := nums.Vector[float32]{1.2, 3.4, 5.6}
	b, _ = vFloat32.MarshalJSON()
	fmt.Println("float32", string(b))

	vInt8 := nums.Vector[int8]{1, 2, 3}
	b, _ = vInt8.MarshalJSON()
	fmt.Println("int8", string(b))

	vInt16 := nums.Vector[int16]{1, 2, 3}
	b, _ = vInt16.MarshalJSON()
	fmt.Println("int16", string(b))

	vInt32 := nums.Vector[int32]{1, 2, 3}
	b, _ = vInt32.MarshalJSON()
	fmt.Println("int32", string(b))

	vInt64 := nums.Vector[int64]{1, 2, 3}
	b, _ = vInt64.MarshalJSON()
	fmt.Println("int64", string(b))

	// Output:
	// float64 [1.2,3.4,5.6]
	// float32 [1.2,3.4,5.6]
	// int8 [1,2,3]
	// int16 [1,2,3]
	// int32 [1,2,3]
	// int64 [1,2,3]
}

func ExampleVector_UnmarshalJSON() {
	b := []byte(`[1.2,3.4,5.6]`)

	vFloat64 := nums.Vector[float64]{}
	json.Unmarshal(b, &vFloat64)
	fmt.Println("float64", vFloat64)

	vFloat32 := nums.Vector[float32]{}
	json.Unmarshal(b, &vFloat32)
	fmt.Println("float32", vFloat32)

	b = []byte(`[1,2,3]`)
	vInt8 := nums.Vector[int8]{}
	json.Unmarshal(b, &vInt8)
	fmt.Println("int8", vInt8)

	vInt16 := nums.Vector[int16]{}
	json.Unmarshal(b, &vInt16)
	fmt.Println("int16", vInt16)

	vInt32 := nums.Vector[int32]{}
	json.Unmarshal(b, &vInt32)
	fmt.Println("int32", vInt32)

	vInt64 := nums.Vector[int64]{}
	json.Unmarshal(b, &vInt64)
	fmt.Println("int64", vInt64)

	// Output:
	// float64 [1.2,3.4,5.6]
	// float32 [1.2,3.4,5.6]
	// int8 [1,2,3]
	// int16 [1,2,3]
	// int32 [1,2,3]
	// int64 [1,2,3]
}

func ExampleVector_Marshal() {
	vFloat64 := nums.Vector[float64]{1.2, 3.4, 5.6}
	b, _ := vFloat64.Marshal()
	fmt.Println("float64", hex.EncodeToString(b))

	vFloat32 := nums.Vector[float32]{1.2, 3.4, 5.6}
	b, _ = vFloat32.Marshal()
	fmt.Println("float32", hex.EncodeToString(b))

	vInt8 := nums.Vector[int8]{1, 2, 3}
	b, _ = vInt8.Marshal()
	fmt.Println("int8", hex.EncodeToString(b))

	vInt16 := nums.Vector[int16]{1, 2, 3}
	b, _ = vInt16.Marshal()
	fmt.Println("int16", hex.EncodeToString(b))

	vInt32 := nums.Vector[int32]{1, 2, 3}
	b, _ = vInt32.Marshal()
	fmt.Println("int32", hex.EncodeToString(b))

	vInt64 := nums.Vector[int64]{1, 2, 3}
	b, _ = vInt64.Marshal()
	fmt.Println("int64", hex.EncodeToString(b))

	// Output:
	// float64 4600033ff3333333333333400b3333333333334016666666666666
	// float32 6600033f99999a4059999a40b33333
	// int8 6f0003010203
	// int16 680003000100020003
	// int32 690003000000010000000200000003
	// int64 490003000000000000000100000000000000020000000000000003
}

func ExampleVector_Unmarshal() {
	b, _ := hex.DecodeString("4600033ff3333333333333400b3333333333334016666666666666")
	vFloat64 := nums.Vector[float64]{}
	vFloat64.Unmarshal(b)
	fmt.Println("float64", vFloat64)

	b, _ = hex.DecodeString("6600033f99999a4059999a40b33333")
	vFloat32 := nums.Vector[float32]{}
	vFloat32.Unmarshal(b)
	fmt.Println("float32", vFloat32)

	b, _ = hex.DecodeString("6f0003010203")
	vInt8 := nums.Vector[int8]{}
	vInt8.Unmarshal(b)
	fmt.Println("int8", vInt8)

	b, _ = hex.DecodeString("680003000100020003")
	vInt16 := nums.Vector[int16]{}
	vInt16.Unmarshal(b)
	fmt.Println("int16", vInt16)

	b, _ = hex.DecodeString("690003000000010000000200000003")
	vInt32 := nums.Vector[int32]{}
	vInt32.Unmarshal(b)
	fmt.Println("int32", vInt32)

	b, _ = hex.DecodeString("490003000000000000000100000000000000020000000000000003")
	vInt64 := nums.Vector[int64]{}
	vInt64.Unmarshal(b)
	fmt.Println("int64", vInt64)

	// Output:
	// float64 [1.2,3.4,5.6]
	// float32 [1.2,3.4,5.6]
	// int8 [1,2,3]
	// int16 [1,2,3]
	// int32 [1,2,3]
	// int64 [1,2,3]
}

func ExampleVector_Marshal_bits() {
	v32 := nums.Vector[float32]{1.0, 2.0, 3.0}
	b, err := v32.Marshal()
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(hex.EncodeToString(b))
	// Output:
	// 6600033f8000004000000040400000
}

func ExampleVector_slice() {
	v := nums.Vector[int32]{1, 2, 3, 4, 5}
	s := v[1:3]
	fmt.Printf("%T %v\n", v, v)
	fmt.Printf("%T %v\n", s, s)

	// element vs. one-dimension vector
	a := v[4]
	b := v[4:5]
	fmt.Printf("%T %v\n", a, a)
	fmt.Printf("%T %v\n", b, b)

	// Output:
	// nums.Vector[int32] [1,2,3,4,5]
	// nums.Vector[int32] [2,3]
	// int32 5
	// nums.Vector[int32] [5]
}
