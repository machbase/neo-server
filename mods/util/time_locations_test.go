package util

import "fmt"

func ExampleParseTimeLocation() {
	tz := ParseTimeLocation("Asia/Seoul", nil)
	if tz == nil {
		panic("invalid timezone")
	}
	fmt.Println("Asia/Seoul =", tz.String())

	tz = ParseTimeLocation("KST", nil)
	if tz == nil {
		panic("invalid timezone")
	}
	fmt.Println("KST =", tz.String())

	tz = ParseTimeLocation("UTC", nil)
	if tz == nil {
		panic("invalid timezone")
	}
	fmt.Println("UTC =", tz.String())

	// Output:
	// Asia/Seoul = Asia/Seoul
	// KST = Asia/Seoul
	// UTC = UTC
}
