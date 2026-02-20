package pretty

import (
	"time"

	"github.com/jedib0t/go-pretty/v6/progress"
)

type ProgressWriter struct {
	progress.Writer
}

func Progress(opt map[string]any) *ProgressWriter {
	showPercentage := true
	if v, ok := opt["showPercentage"].(bool); ok {
		showPercentage = v
	}
	showETA := true
	if v, ok := opt["showETA"].(bool); ok {
		showETA = v
	}
	showSpeed := true
	if v, ok := opt["showSpeed"].(bool); ok {
		showSpeed = v
	}
	updateFrequency := 250 * time.Millisecond
	if v, ok := opt["updateFrequency"].(int64); ok {
		updateFrequency = time.Duration(v) * time.Millisecond
	}
	trackerLength := 20
	if v, ok := opt["trackerLength"].(int64); ok {
		trackerLength = int(v)
	}
	w := progress.NewWriter()
	w.SetAutoStop(true)
	w.SetTrackerLength(trackerLength)
	w.SetUpdateFrequency(updateFrequency)
	w.Style().Visibility.ETA = showETA
	w.Style().Visibility.Percentage = showPercentage
	w.Style().Visibility.Speed = showSpeed

	ret := &ProgressWriter{
		Writer: w,
	}
	go ret.Writer.Render()

	return ret
}

func (pw *ProgressWriter) Tracker(opt map[string]any) *progress.Tracker {
	message := ""
	if v, ok := opt["message"].(string); ok {
		message = v
	}
	var total int64 = 0
	if v, ok := opt["total"].(int64); ok {
		total = v
	}
	tracker := &progress.Tracker{
		Message:    message,
		Total:      total,
		DeferStart: true,
		Units:      progress.UnitsDefault,
	}

	pw.Writer.AppendTracker(tracker)
	return tracker
}
