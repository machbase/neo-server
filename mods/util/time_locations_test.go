package util

import (
	"fmt"
	"time"
)

func ExampleParseTimeLocation() {
	tz, _ := ParseTimeLocation("Asia/Seoul", nil)
	if tz == nil {
		panic("invalid timezone")
	}
	fmt.Println("Asia/Seoul =", tz.String())

	tz, _ = ParseTimeLocation("KST", nil)
	if tz == nil {
		panic("invalid timezone")
	}
	fmt.Println("KST =", tz.String())

	tz, _ = ParseTimeLocation("UTC", nil)
	if tz == nil {
		panic("invalid timezone")
	}
	fmt.Println("UTC =", tz.String())

	fallback, err := ParseTimeLocation("America/Invalid", time.UTC)
	fmt.Println("Error =", err)
	fmt.Println("Fallback =", fallback.String())

	fallback, err = ParseTimeLocation("Invalid", time.UTC)
	fmt.Println("Error =", err)
	fmt.Println("Fallback =", fallback.String())

	// Output:
	// Asia/Seoul = Asia/Seoul
	// KST = Asia/Seoul
	// UTC = UTC
	// Error = unknown time zone America/Invalid
	// Fallback = UTC
	// Error = unknown time zone Invalid
	// Fallback = UTC
}
