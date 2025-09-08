package metric

import (
	"fmt"
	"time"
)

var nowFunc func() time.Time = time.Now

var timeZone *time.Location = time.Local

// T is the input type for the time series.
// P is the type of the value stored in the time series.
type Producer interface {
	// Add adds a value to the producer.
	Add(float64)
	// Produce returns the last value in the producer.
	// It resets the producer after producing.
	// reset indicates whether to reset the producer after producing.
	Produce(bool) Value
	// String returns a string representation of the producer.
	// It should be `{"ts":"2023-10-01T12:04:05Z","value":1}` for a single value.
	String() string
	// MarshalJSON marshals the producer to JSON.
	MarshalJSON() ([]byte, error)
	// UnmarshalJSON unmarshal the producer from JSON.
	UnmarshalJSON(data []byte) error
}

// Value is the output type for the time series.
type Value interface {
	String() string
}

// Type is the type of the Field.
type Type struct {
	p func() Producer
	s string
	u Unit
}

func (ft Type) Empty() bool {
	return ft.p == nil
}

func (ft Type) Producer() Producer {
	return ft.p()
}

func (ft Type) String() string {
	return ft.s
}

func (ft Type) Unit() Unit {
	return ft.u
}

type Unit string

const (
	UnitPercent  Unit = "Percent"
	UnitBytes    Unit = "Bytes"
	UnitShort    Unit = "Short"
	UnitDuration Unit = "Duration"
)

const (
	bytesInKB = 1_024
	bytesInMB = 1_048_576
	bytesInGB = 1_073_741_824
	bytesInTB = 1_099_511_627_776
	bytesInPB = 1_125_899_906_842_624
	bytesInEB = 1_152_921_504_606_846_976
)

func (u Unit) Format(value float64, decimal int) string {
	switch u {
	case UnitPercent:
		return fmt.Sprintf("%.*f%%", decimal, value)
	case UnitBytes:
		if value < bytesInMB {
			return fmt.Sprintf("%.1f%s", value/bytesInKB, "KB")
		} else if value < bytesInGB {
			return fmt.Sprintf("%.1f%s", value/bytesInMB, "MB")
		} else if value < bytesInTB {
			return fmt.Sprintf("%.1f%s", value/bytesInGB, "GB")
		} else if value < bytesInPB {
			return fmt.Sprintf("%.1f%s", value/bytesInTB, "TB")
		} else if value < bytesInEB {
			return fmt.Sprintf("%.1f%s", value/bytesInPB, "PB")
		}
		return fmt.Sprintf("%.f%s", value, "B")
	case UnitShort:
		if value < 1_000 {
			return fmt.Sprintf("%.1f", value)
		} else if value < 1_000_000 {
			return fmt.Sprintf("%.1fK", value/1_000)
		} else if value < 1_000_000_000 {
			return fmt.Sprintf("%.1fM", value/1_000_000)
		} else if value < 1_000_000_000_000 {
			return fmt.Sprintf("%.1fG", value/1_000_000_000)
		} else if value < 1_000_000_000_000_000 {
			return fmt.Sprintf("%.1fT", value/1_000_000_000_000)
		}
		return fmt.Sprintf("%.1fP", value/1_000_000_000_000_000)
	case UnitDuration:
		switch {
		case value < 1e3:
			return fmt.Sprintf("%.0fns", value)
		case value < 1e6:
			return fmt.Sprintf("%.0fus", value/1e3)
		case value < 1e9:
			return fmt.Sprintf("%.0fms", value/1e6)
		case value < 60e6:
			return fmt.Sprintf("%.0fs", value/1e9)
		case value < 3600e6:
			return fmt.Sprintf("%.0fm", value/60e6)
		case value < 86400e6:
			return fmt.Sprintf("%.0fh", value/3600e6)
		case value < 604800e6:
			return fmt.Sprintf("%.0fd", value/86400e6)
		case value < 2592000e6:
			return fmt.Sprintf("%.0fw", value/604800e6)
		case value < 31536000e6:
			return fmt.Sprintf("%.0fmo", value/2592000e6)
		case value < 31536000000e6:
			return fmt.Sprintf("%.0fy", value/31536000e6)
		}
		return time.Duration(value).String()
	default:
		return fmt.Sprintf("%g", value)
	}
}
