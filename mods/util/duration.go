package util

import (
	"fmt"
	"math"
	"time"
)

type HumanizeDurationFormat int

const (
	HumanizeDurationFormatLong HumanizeDurationFormat = iota + 1
	HumanizeDurationFormatShort
	HumanizeDurationFormatShortPadding
	HumanizeDurationFormatSimple
	HumanizeDurationFormatSimplePadding
)

func HumanizeDuration(duration time.Duration) string {
	return HumanizeDurationWithFormat(duration, HumanizeDurationFormatLong)
}

func HumanizeDurationWithFormat(duration time.Duration, format HumanizeDurationFormat) string {
	if duration.Seconds() < 60.0 {
		switch format {
		default:
			return fmt.Sprintf("%d seconds", int(duration.Seconds()))
		case HumanizeDurationFormatShort:
			return fmt.Sprintf("0:%d", int(duration.Seconds()))
		case HumanizeDurationFormatShortPadding:
			return fmt.Sprintf("00:%02d", int(duration.Seconds()))
		case HumanizeDurationFormatSimple:
			return fmt.Sprintf("%ds", int(duration.Seconds()))
		case HumanizeDurationFormatSimplePadding:
			return fmt.Sprintf("%02ds", int(duration.Seconds()))
		}
	}
	if duration.Minutes() < 60.0 {
		remainingSeconds := int(math.Mod(duration.Seconds(), 60))
		switch format {
		default:
			return fmt.Sprintf("%d minutes %d seconds", int(duration.Minutes()), remainingSeconds)
		case HumanizeDurationFormatShort:
			return fmt.Sprintf("%d:%d", int(duration.Minutes()), remainingSeconds)
		case HumanizeDurationFormatShortPadding:
			return fmt.Sprintf("%02d:%02d", int(duration.Minutes()), remainingSeconds)
		case HumanizeDurationFormatSimple:
			return fmt.Sprintf("%dm %ds", int(duration.Minutes()), remainingSeconds)
		case HumanizeDurationFormatSimplePadding:
			return fmt.Sprintf("%02dm %02ds", int(duration.Minutes()), remainingSeconds)
		}
	}
	if duration.Hours() < 24.0 {
		remainingMinutes := int(math.Mod(duration.Minutes(), 60))
		remainingSeconds := int(math.Mod(duration.Seconds(), 60))
		switch format {
		default:
			return fmt.Sprintf("%d hours %d minutes %d seconds",
				int(duration.Hours()), remainingMinutes, remainingSeconds)
		case HumanizeDurationFormatShort:
			return fmt.Sprintf("%d:%d:%d",
				int(duration.Hours()), remainingMinutes, remainingSeconds)
		case HumanizeDurationFormatShortPadding:
			return fmt.Sprintf("%02d:%02d:%02d",
				int(duration.Hours()), remainingMinutes, remainingSeconds)
		case HumanizeDurationFormatSimple:
			return fmt.Sprintf("%dh %dm %ds",
				int(duration.Hours()), remainingMinutes, remainingSeconds)
		case HumanizeDurationFormatSimplePadding:
			return fmt.Sprintf("%02dh %02dm %02ds",
				int(duration.Hours()), remainingMinutes, remainingSeconds)
		}
	}
	remainingHours := int(math.Mod(duration.Hours(), 24))
	remainingMinutes := int(math.Mod(duration.Minutes(), 60))
	remainingSeconds := int(math.Mod(duration.Seconds(), 60))
	days := int(duration.Hours() / 24)
	if days > 0 {
		dd := "days"
		if days == 1 {
			dd = "day"
		}
		switch format {
		default:
			return fmt.Sprintf("%d %s %d hours %d minutes %d seconds",
				days, dd, remainingHours, remainingMinutes, remainingSeconds)
		case HumanizeDurationFormatShort:
			return fmt.Sprintf("%d %s %d:%d:%d",
				days, dd, remainingHours, remainingMinutes, remainingSeconds)
		case HumanizeDurationFormatShortPadding:
			return fmt.Sprintf("%d %s %d:%02d:%02d",
				days, dd, remainingHours, remainingMinutes, remainingSeconds)
		case HumanizeDurationFormatSimple:
			return fmt.Sprintf("%d %s %dh %dm %ds",
				days, dd, remainingHours, remainingMinutes, remainingSeconds)
		case HumanizeDurationFormatSimplePadding:
			return fmt.Sprintf("%d %s %02dh %02dm %02ds",
				days, dd, remainingHours, remainingMinutes, remainingSeconds)
		}
	} else {
		switch format {
		default:
			return fmt.Sprintf("%d hours %d minutes %d seconds",
				remainingHours, remainingMinutes, remainingSeconds)
		case HumanizeDurationFormatShort:
			return fmt.Sprintf("%d:%d:%d",
				remainingHours, remainingMinutes, remainingSeconds)
		case HumanizeDurationFormatShortPadding:
			return fmt.Sprintf("%d:%02d:%02d",
				remainingHours, remainingMinutes, remainingSeconds)
		case HumanizeDurationFormatSimple:
			return fmt.Sprintf("%dh %dm %ds",
				remainingHours, remainingMinutes, remainingSeconds)
		case HumanizeDurationFormatSimplePadding:
			return fmt.Sprintf("%02dh %02dm %02ds",
				remainingHours, remainingMinutes, remainingSeconds)
		}
	}
}
