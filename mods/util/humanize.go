package util

import (
	"fmt"
	"math"
	"time"

	"golang.org/x/exp/constraints"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

func HumanizeNumber[T constraints.Integer](b T) string {
	p := message.NewPrinter(language.English)
	return p.Sprint(b)
}

func HumanizeByteCount[T constraints.Integer](b T) string {
	return HumanizeByteCountUint64(uint64(b))
}

func HumanizeByteCountUint64(b uint64) string {
	const unit = 1000
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB",
		float64(b)/float64(div), "kMGTPE"[exp])
}

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
			return _formatSeconds(int(duration.Seconds()))
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
			return fmt.Sprintf("%s %s", _formatMinutes(int(duration.Minutes())), _formatSeconds(remainingSeconds))
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
			return fmt.Sprintf("%s %s %s",
				_formatHours(int(duration.Hours())),
				_formatMinutes(remainingMinutes),
				_formatSeconds(remainingSeconds))
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
	} else {
		remainingHours := int(math.Mod(duration.Hours(), 24))
		remainingMinutes := int(math.Mod(duration.Minutes(), 60))
		remainingSeconds := int(math.Mod(duration.Seconds(), 60))
		days := int(duration.Hours() / 24)
		switch format {
		default:
			return fmt.Sprintf("%s %s %s %s",
				_formatDays(days), _formatHours(remainingHours),
				_formatMinutes(remainingMinutes), _formatSeconds(remainingSeconds))
		case HumanizeDurationFormatShort:
			return fmt.Sprintf("%s %d:%d:%d",
				_formatDays(days), remainingHours, remainingMinutes, remainingSeconds)
		case HumanizeDurationFormatShortPadding:
			return fmt.Sprintf("%s %02d:%02d:%02d",
				_formatDays(days), remainingHours, remainingMinutes, remainingSeconds)
		case HumanizeDurationFormatSimple:
			return fmt.Sprintf("%s %dh %dm %ds",
				_formatDays(days), remainingHours, remainingMinutes, remainingSeconds)
		case HumanizeDurationFormatSimplePadding:
			return fmt.Sprintf("%s %02dh %02dm %02ds",
				_formatDays(days), remainingHours, remainingMinutes, remainingSeconds)
		}
	}
}

func _formatSeconds(s int) string {
	if s < 2 {
		return fmt.Sprintf("%d second", s)
	}
	return fmt.Sprintf("%d seconds", s)
}

func _formatMinutes(m int) string {
	if m < 2 {
		return fmt.Sprintf("%d minute", m)
	}
	return fmt.Sprintf("%d minutes", m)
}

func _formatHours(h int) string {
	if h < 2 {
		return fmt.Sprintf("%d hour", h)
	}
	return fmt.Sprintf("%d hours", h)
}

func _formatDays(d int) string {
	if d < 2 {
		return fmt.Sprintf("%d day", d)
	}
	return fmt.Sprintf("%d days", d)
}
