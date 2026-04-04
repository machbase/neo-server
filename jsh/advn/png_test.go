package advn

import (
	"testing"
	"time"
)

func TestChooseMRTGTickSpecUsesPlotWidth(t *testing.T) {
	span := 24 * time.Hour

	narrow := chooseMRTGTickSpec(span, 180, rendererMeasureTickLabelWidth)
	wide := chooseMRTGTickSpec(span, 720, rendererMeasureTickLabelWidth)

	if !(narrow.Step > wide.Step) {
		t.Fatalf("expected narrower plot to choose coarser step, got narrow=%v wide=%v", narrow.Step, wide.Step)
	}
}

func TestSelectMRTGVisibleXTicksKeepsTicksButSkipsLabels(t *testing.T) {
	ticks := []pngTick{
		{Pos: 0, Label: "00:00"},
		{Pos: 20, Label: "00:05"},
		{Pos: 40, Label: "00:10"},
		{Pos: 60, Label: "00:15"},
	}

	visible := selectMRTGVisibleXTicks(ticks, func(string) int { return 30 })
	if len(visible) >= len(ticks) {
		t.Fatalf("expected overlapping labels to be reduced, got %d from %d", len(visible), len(ticks))
	}
	if visible[0].Label != "00:00" {
		t.Fatalf("expected first tick label to stay visible, got %q", visible[0].Label)
	}
	if visible[len(visible)-1].Label != "00:15" {
		t.Fatalf("expected last tick label to stay visible, got %q", visible[len(visible)-1].Label)
	}
}
