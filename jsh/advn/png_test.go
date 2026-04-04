package advn

import (
	"strings"
	"testing"
)

func TestBuildMRTGLegendPlanUsesFullStatsWhenItFits(t *testing.T) {
	statsColor := mustParseColor("#4a4a4a")
	seriesData := []pngSeriesData{
		{Stats: pngSeriesStats{Label: "MEM", Color: mustParseColor("#00d000"), Max: 63.7, Average: 62.9, Current: 63.4, HasData: true}},
		{Stats: pngSeriesStats{Label: "CPU", Color: mustParseColor("#0000ff"), Max: 47.7, Average: 14.2, Current: 6.8, HasData: true}},
	}

	plan := buildMRTGLegendPlan(seriesData, svgDefaultWidth-12, statsColor)
	if plan.LabelsOnly {
		t.Fatal("expected full stats legend plan for default width")
	}
	if len(plan.Rows) == 0 || len(plan.Rows) > 2 {
		t.Fatalf("expected 1 or 2 legend rows, got %d", len(plan.Rows))
	}

	foundStatsText := false
	foundStatsColor := false
	for _, row := range plan.Rows {
		for _, item := range row {
			for _, part := range item.Parts {
				if strings.Contains(part.Text, "Max:") && strings.Contains(part.Text, "Avg:") && strings.Contains(part.Text, "Cur:") {
					foundStatsText = true
					if part.Color == statsColor {
						foundStatsColor = true
					}
				}
			}
		}
	}
	if !foundStatsText {
		t.Fatal("expected legend items to contain full stats text")
	}
	if !foundStatsColor {
		t.Fatal("expected stats text to use the shared stats color")
	}
}

func TestBuildMRTGLegendPlanFallsBackToLabelsOnlyWhenWidthIsSmall(t *testing.T) {
	statsColor := mustParseColor("#4a4a4a")
	seriesData := []pngSeriesData{
		{Stats: pngSeriesStats{Label: "memory-used", Color: mustParseColor("#00d000"), Max: 63.7, Average: 62.9, Current: 63.4, HasData: true}},
		{Stats: pngSeriesStats{Label: "cpu-user", Color: mustParseColor("#0000ff"), Max: 47.7, Average: 14.2, Current: 6.8, HasData: true}},
		{Stats: pngSeriesStats{Label: "disk-read", Color: mustParseColor("#ff8000"), Max: 17.7, Average: 12.2, Current: 8.1, HasData: true}},
		{Stats: pngSeriesStats{Label: "net-rx", Color: mustParseColor("#cc00cc"), Max: 22.5, Average: 10.5, Current: 2.0, HasData: true}},
	}

	plan := buildMRTGLegendPlan(seriesData, 220, statsColor)
	if !plan.LabelsOnly {
		t.Fatal("expected label-only fallback for narrow width")
	}
	if len(plan.Rows) != 2 {
		t.Fatalf("expected 2 legend rows in label-only fallback, got %d", len(plan.Rows))
	}

	for _, row := range plan.Rows {
		for _, item := range row {
			for _, part := range item.Parts {
				if strings.Contains(part.Text, "Max:") || strings.Contains(part.Text, "Avg:") || strings.Contains(part.Text, "Cur:") {
					t.Fatalf("expected label-only legend item, got %q", part.Text)
				}
			}
		}
	}
}

func TestBuildMRTGPNGLayoutUsesLegendRows(t *testing.T) {
	options := svgResolvedOptions{
		Width:    svgDefaultWidth,
		Height:   svgDefaultHeight,
		FontSize: svgDefaultFontSize,
	}

	oneRowPlan := pngLegendPlan{
		Rows: [][]pngLegendItem{{
			{Width: 120},
		}},
	}
	twoRowPlan := pngLegendPlan{
		Rows: [][]pngLegendItem{
			{{Width: 120}},
			{{Width: 120}},
		},
	}
	labelOnlyPlan := pngLegendPlan{
		Rows: [][]pngLegendItem{
			{{Width: 80}, {Width: 80}},
			{{Width: 80}, {Width: 80}},
		},
		LabelsOnly: true,
	}

	layout1 := buildMRTGPNGLayout(options, oneRowPlan)
	layout2 := buildMRTGPNGLayout(options, twoRowPlan)
	layout3 := buildMRTGPNGLayout(options, labelOnlyPlan)

	if layout1.Plot.Height <= layout2.Plot.Height {
		t.Fatalf("expected 1-row legend plot height %d to be larger than 2-row legend plot height %d", layout1.Plot.Height, layout2.Plot.Height)
	}
	if layout1.XTickBaseY-(layout1.Plot.Y+layout1.Plot.Height) != svgDefaultFontSize+4 {
		t.Fatalf("expected x tick offset %d, got %d", svgDefaultFontSize+4, layout1.XTickBaseY-(layout1.Plot.Y+layout1.Plot.Height))
	}
	if layout3.Stats.Y-layout3.XTickBaseY <= layout2.Stats.Y-layout2.XTickBaseY {
		t.Fatalf("expected label-only bottom gap %d to be larger than full legend gap %d", layout3.Stats.Y-layout3.XTickBaseY, layout2.Stats.Y-layout2.XTickBaseY)
	}
}
