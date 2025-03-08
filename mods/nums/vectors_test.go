package nums_test

import (
	"encoding/hex"
	"fmt"

	"github.com/machbase/neo-server/v8/mods/nums"
)

func ExampleVector() {
	v := nums.Vector{1.0, 2.0, 3.0}
	fmt.Println(v)
	fmt.Println(v.Dimension())
	// Output:
	// [1,2,3]
	// 3
}

func ExampleVector_MarshalJSON() {
	v := nums.Vector{1.0, 2.0, 3.0}
	b, err := v.MarshalJSON()
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(string(b))
	// Output: {"d":3,"v":[1,2,3]}
}

func ExampleVector_UnmarshalJSON() {
	v := nums.Vector{}
	b := []byte(`{"d":3,"v":[1,2,3]}`)
	err := v.UnmarshalJSON(b)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(v)
	// Output: [1,2,3]
}

func ExampleVector_Marshal() {
	v64 := nums.Vector{1.0, 2.0, 3.0}
	b, err := v64.Marshal(64)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(hex.EncodeToString(b))
	// Output:
	// 0800033ff000000000000040000000000000004008000000000000
}

func ExampleVector_Unmarshal() {
	b, err := hex.DecodeString("0800033ff000000000000040000000000000004008000000000000")
	if err != nil {
		fmt.Println(err)
	}
	v := nums.Vector{}
	err = v.Unmarshal(b)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(v)

	// Output:
	// [1,2,3]
}

func ExampleVector_Marshal_bits() {
	v32 := nums.Vector{1.0, 2.0, 3.0}
	b, err := v32.Marshal(32)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(hex.EncodeToString(b))
	// Output:
	// 0400033f8000004000000040400000
}

func ExampleVector_Unmarshal_bits() {
	b, err := hex.DecodeString("0400033f8000004000000040400000")
	if err != nil {
		fmt.Println(err)
	}
	v := nums.Vector{}
	err = v.Unmarshal(b)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(v)
	// Output:
	// [1,2,3]
}
