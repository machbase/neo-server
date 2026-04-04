package advn

import (
	"fmt"
	"strconv"
	"time"
)

type OutputTimeOptions struct {
	Timeformat string `json:"timeformat,omitempty"`
	TZ         string `json:"tz,omitempty"`
}

type resolvedOutputTimeOptions struct {
	Timeformat string
	TZ         string
	Location   *time.Location
}

func resolveOutputTimeOptions(domain Domain, options OutputTimeOptions) (resolvedOutputTimeOptions, error) {
	resolved := resolvedOutputTimeOptions{}
	if options.Timeformat == "default" {
		options.Timeformat = ""
	}
	if options.Timeformat == "" {
		options.Timeformat = TimeformatRFC3339
	}
	if options.TZ == "" {
		options.TZ = "Local"
	}
	if options.TZ == "local" {
		options.TZ = "Local"
	}
	timeformat := canonicalTimeformat(options.Timeformat, "")
	if timeformat == "" {
		timeformat = TimeformatRFC3339
	}
	if !contains(timeformat, TimeformatRFC3339, TimeformatSecond, TimeformatMilli, TimeformatMicro, TimeformatNano) {
		return resolvedOutputTimeOptions{}, fmt.Errorf("advn: invalid output timeformat %q", timeformat)
	}
	tz := options.TZ
	location := time.UTC
	if tz != "" {
		loaded, err := time.LoadLocation(tz)
		if err != nil {
			return resolvedOutputTimeOptions{}, fmt.Errorf("advn: invalid output tz %q", tz)
		}
		location = loaded
	}
	resolved.Timeformat = timeformat
	resolved.TZ = tz
	resolved.Location = location
	return resolved, nil
}

func formatTimeValueWithOptions(value any, domain Domain, options resolvedOutputTimeOptions) string {
	timeValue, ok := parseTimeValueWithDomain(value, domain)
	if !ok {
		return formatAny(value)
	}
	return formatResolvedTime(timeValue, options)
}

func normalizeTimeValueForEChartsWithOptions(value any, domain Domain, options resolvedOutputTimeOptions) any {
	timeValue, ok := parseTimeValueWithDomain(value, domain)
	if !ok {
		return value
	}
	return encodeResolvedTime(timeValue, options)
}

func formatResolvedTime(timeValue time.Time, options resolvedOutputTimeOptions) string {
	switch options.Timeformat {
	case TimeformatSecond:
		return strconv.FormatInt(timeValue.Unix(), 10)
	case TimeformatMilli:
		return strconv.FormatInt(timeValue.UnixMilli(), 10)
	case TimeformatMicro:
		return strconv.FormatInt(timeValue.UnixMicro(), 10)
	case TimeformatNano:
		return strconv.FormatInt(timeValue.UnixNano(), 10)
	default:
		return timeValue.In(options.Location).Format(time.RFC3339Nano)
	}
}

func encodeResolvedTime(timeValue time.Time, options resolvedOutputTimeOptions) any {
	switch options.Timeformat {
	case TimeformatSecond:
		return timeValue.Unix()
	case TimeformatMilli:
		return timeValue.UnixMilli()
	case TimeformatMicro:
		return timeValue.UnixMicro()
	case TimeformatNano:
		return strconv.FormatInt(timeValue.UnixNano(), 10)
	default:
		return timeValue.In(options.Location).Format(time.RFC3339Nano)
	}
}

func formatUnixNanoWithOptions(unixNano int64, span int64, options resolvedOutputTimeOptions) string {
	timeValue := time.Unix(0, unixNano)
	if options.Timeformat != TimeformatRFC3339 {
		return formatResolvedTime(timeValue, options)
	}
	timeValue = timeValue.In(options.Location)
	switch {
	case span <= int64(time.Minute):
		return timeValue.Format("15:04:05")
	case span <= int64(6*time.Hour):
		return timeValue.Format("15:04")
	case span <= int64(48*time.Hour):
		return timeValue.Format("01-02 15:04")
	case span <= int64(180*24*time.Hour):
		return timeValue.Format("2006-01-02")
	case span <= int64(2*365*24*time.Hour):
		return timeValue.Format("2006-01")
	default:
		return timeValue.Format("2006")
	}
}

func formatTimeTickWithOptions(value time.Time, span time.Duration, options resolvedOutputTimeOptions) string {
	if options.Timeformat != TimeformatRFC3339 {
		return formatResolvedTime(value, options)
	}
	value = value.In(options.Location)
	switch {
	case span <= time.Minute:
		return value.Format("15:04:05")
	case span <= 6*time.Hour:
		return value.Format("15:04")
	case span <= 48*time.Hour:
		return value.Format("01-02 15:04")
	case span <= 180*24*time.Hour:
		return value.Format("2006-01-02")
	case span <= 2*365*24*time.Hour:
		return value.Format("2006-01")
	default:
		return value.Format("2006")
	}
}

func echartsTimeAxisType(output resolvedOutputTimeOptions) string {
	if output.Timeformat == TimeformatRFC3339 || output.Timeformat == TimeformatMilli {
		return "time"
	}
	return "value"
}
