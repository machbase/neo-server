package advn

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func TestToTUIBlocksBand(t *testing.T) {
	spec := (&Spec{
		Version: Version1,
		Domain:  Domain{Kind: DomainKindTime, From: "2026-04-03T00:00:00Z", To: "2026-04-03T01:00:00Z"},
		Series: []Series{{
			ID:   "sensor-1",
			Name: "sensor-1",
			Representation: Representation{
				Kind:   RepresentationTimeBucketBand,
				Fields: []string{"time", "min", "max", "avg"},
			},
			Data: []any{
				[]any{"2026-04-03T00:00:00Z", 1, 5, 3},
				[]any{"2026-04-03T00:10:00Z", 2, 6, 4},
				[]any{"2026-04-03T00:20:00Z", 1, 4, 2},
			},
		}},
		Annotations: []Annotation{{Kind: AnnotationKindLine, Axis: "x", Value: "2026-04-03T00:30:00Z", Label: "checkpoint"}},
	}).Normalize()

	blocks, err := ToTUIBlocks(spec)
	if err != nil {
		t.Fatalf("ToTUIBlocks() returned unexpected error: %v", err)
	}
	if len(blocks) < 4 {
		t.Fatalf("expected at least 4 blocks, got %d", len(blocks))
	}
	if blocks[0].Type != "summary" {
		t.Fatalf("expected first block type %q, got %q", "summary", blocks[0].Type)
	}
	if blocks[2].Type != "bandline" {
		t.Fatalf("expected visualization block type %q, got %q", "bandline", blocks[2].Type)
	}
	if len(blocks[2].Lines) != 3 {
		t.Fatalf("expected 3 band lines, got %d", len(blocks[2].Lines))
	}
	if blocks[3].Type != "table" {
		t.Fatalf("expected table block type %q, got %q", "table", blocks[3].Type)
	}
	if blocks[4].Type != "annotations" {
		t.Fatalf("expected annotations block type %q, got %q", "annotations", blocks[4].Type)
	}
}

func TestToTUIBlocksHistogramAndEvents(t *testing.T) {
	spec := (&Spec{
		Version: Version1,
		Domain:  Domain{Kind: DomainKindTime, From: "2026-04-03T00:00:00Z", To: "2026-04-03T12:00:00Z"},
		Series: []Series{
			{
				ID:             "hist-1",
				Name:           "hist-1",
				Representation: Representation{Kind: RepresentationDistributionHistogram, Fields: []string{"binStart", "binEnd", "count"}},
				Data:           []any{[]any{0, 10, 3}, []any{10, 20, 8}},
			},
			{
				ID:             "event-range-1",
				Name:           "maintenance",
				Representation: Representation{Kind: RepresentationEventRange, Fields: []string{"from", "to", "label"}},
				Data:           []any{[]any{"2026-04-03T10:00:00Z", "2026-04-03T11:00:00Z", "maintenance"}},
			},
		},
	}).Normalize()

	blocks, err := ToTUIBlocks(spec)
	if err != nil {
		t.Fatalf("ToTUIBlocks() returned unexpected error: %v", err)
	}
	var sawBars bool
	var sawTimeline bool
	for _, block := range blocks {
		if block.Type == "bars" && len(block.Lines) > 0 {
			sawBars = true
		}
		if block.Type == "timeline" && len(block.Lines) > 0 {
			sawTimeline = true
		}
	}
	if !sawBars {
		t.Fatal("expected histogram bars block")
	}
	if !sawTimeline {
		t.Fatal("expected event timeline block")
	}
}

func TestToTUIBlocksWithOptions(t *testing.T) {
	data := make([]any, 0, 12)
	for index := 0; index < 12; index++ {
		data = append(data, []any{index, index})
	}
	spec := (&Spec{
		Version: Version1,
		Series: []Series{{
			ID:             "raw-1",
			Name:           "raw-1",
			Representation: Representation{Kind: RepresentationRawPoint, Fields: []string{"x", "y"}},
			Data:           data,
		}},
	}).Normalize()

	blocks, err := ToTUIBlocksWithOptions(spec, &TUIOptions{Width: 5, Rows: 3})
	if err != nil {
		t.Fatalf("ToTUIBlocksWithOptions() returned unexpected error: %v", err)
	}
	if len(blocks) != 4 {
		t.Fatalf("expected 4 blocks, got %d", len(blocks))
	}
	if blocks[2].Type != "sparkline" {
		t.Fatalf("expected visualization block type %q, got %q", "sparkline", blocks[2].Type)
	}
	if len(blocks[2].Lines) != 4 {
		t.Fatalf("expected sparkline with axis and 3 chart rows, got %#v", blocks[2].Lines)
	}
	if !strings.Contains(blocks[2].Lines[1], "┤") {
		t.Fatalf("expected sparkline y-axis marker, got %#v", blocks[2].Lines)
	}
	if utf8.RuneCountInString(blocks[2].Lines[0]) < 5 {
		t.Fatalf("expected sparkline width 5, got %#v", blocks[2].Lines)
	}
	if len(blocks[3].Rows) != 3 {
		t.Fatalf("expected 3 table rows, got %d", len(blocks[3].Rows))
	}

	compactBlocks, err := ToTUIBlocksWithOptions(spec, &TUIOptions{Width: 5, Rows: 3, Compact: true})
	if err != nil {
		t.Fatalf("ToTUIBlocksWithOptions(compact) returned unexpected error: %v", err)
	}
	if len(compactBlocks) != 2 {
		t.Fatalf("expected 2 compact blocks, got %d", len(compactBlocks))
	}
	if compactBlocks[1].Type != "sparkline" {
		t.Fatalf("expected compact visualization block type %q, got %q", "sparkline", compactBlocks[1].Type)
	}
}

func TestToSparkline(t *testing.T) {
	spec := (&Spec{
		Version: Version1,
		Domain:  Domain{Kind: DomainKindTime, From: "2026-04-03T00:00:00Z", To: "2026-04-03T00:04:00Z"},
		Series: []Series{{
			ID:             "value-1",
			Representation: Representation{Kind: RepresentationTimeBucketValue, Fields: []string{"time", "value"}},
			Data: []any{
				[]any{"2026-04-03T00:00:00Z", 1},
				[]any{"2026-04-03T00:02:00Z", 4},
				[]any{"2026-04-03T00:04:00Z", 2},
			},
		}},
	}).Normalize()

	lines, err := ToSparklineWithOptions(spec, &TUIOptions{Width: 12})
	if err != nil {
		t.Fatalf("ToSparklineWithOptions() returned unexpected error: %v", err)
	}
	if len(lines) != 4 {
		t.Fatalf("expected sparkline with axis and 3 chart rows, got %#v", lines)
	}
	if !strings.Contains(lines[1], "┤") {
		t.Fatalf("expected sparkline y-axis marker, got %#v", lines)
	}
	if !strings.Contains(lines[0], ":") {
		t.Fatalf("expected sparkline x-axis label, got %#v", lines)
	}
}

func TestToSparklineRejectsUnsupportedSeries(t *testing.T) {
	spec := (&Spec{
		Version: Version1,
		Series: []Series{{
			ID:             "hist-1",
			Representation: Representation{Kind: RepresentationDistributionHistogram, Fields: []string{"binStart", "binEnd", "count"}},
			Data:           []any{[]any{0, 10, 3}},
		}},
	}).Normalize()

	if _, err := ToSparkline(spec); err == nil {
		t.Fatal("expected unsupported sparkline series error")
	}
}

func TestToTUIBlocksEpochNanoseconds(t *testing.T) {
	oldLocal := time.Local
	time.Local = time.FixedZone("KST", 9*60*60)
	t.Cleanup(func() {
		time.Local = oldLocal
	})

	spec := (&Spec{
		Version: Version1,
		Domain: Domain{
			Kind:       DomainKindTime,
			Timeformat: TimeformatNano,
			From:       json.Number("1775174400000000000"),
			To:         json.Number("1775217600000000000"),
		},
		Series: []Series{{
			ID:             "event-range-1",
			Name:           "maintenance",
			Representation: Representation{Kind: RepresentationEventRange, Fields: []string{"from", "to", "label"}},
			Data: []any{
				[]any{json.Number("1775210400000000000"), json.Number("1775214000000000000"), "maintenance"},
			},
		}},
	}).Normalize()

	blocks, err := ToTUIBlocks(spec)
	if err != nil {
		t.Fatalf("ToTUIBlocks() returned unexpected error: %v", err)
	}
	if !strings.Contains(blocks[0].Lines[0], "2026-04-03T09:00:00+09:00") {
		t.Fatalf("expected summary line to contain local RFC3339 time, got %q", blocks[0].Lines[0])
	}
	if strings.Contains(blocks[0].Lines[0], "1775174400000000000") {
		t.Fatalf("expected summary line to avoid raw epoch ns time, got %q", blocks[0].Lines[0])
	}
	if !strings.Contains(blocks[2].Lines[1], "2026-04-03T19:00:00+09:00") {
		t.Fatalf("expected timeline detail to contain local RFC3339 from time, got %q", blocks[2].Lines[1])
	}
	if !strings.Contains(blocks[2].Lines[1], "2026-04-03T20:00:00+09:00") {
		t.Fatalf("expected timeline detail to contain local RFC3339 to time, got %q", blocks[2].Lines[1])
	}

	overrideBlocks, err := ToTUIBlocksWithOptions(spec, &TUIOptions{Timeformat: TimeformatRFC3339, TZ: "Asia/Seoul"})
	if err != nil {
		t.Fatalf("ToTUIBlocksWithOptions() returned unexpected error: %v", err)
	}
	if !strings.Contains(overrideBlocks[0].Lines[0], "+09:00") {
		t.Fatalf("expected summary line to contain timezone-adjusted RFC3339, got %q", overrideBlocks[0].Lines[0])
	}
}

func TestToTUIBlocksTableTimeOverrides(t *testing.T) {
	spec := (&Spec{
		Version: Version1,
		Domain:  Domain{Kind: DomainKindTime, Timeformat: TimeformatNano},
		Series: []Series{{
			ID:             "value-1",
			Name:           "value-1",
			Representation: Representation{Kind: RepresentationTimeBucketValue, Fields: []string{"time", "value"}},
			Data: []any{
				[]any{"1775174400000000000", 1},
			},
		}},
	}).Normalize()

	blocks, err := ToTUIBlocksWithOptions(spec, &TUIOptions{Timeformat: TimeformatRFC3339, TZ: "Asia/Seoul"})
	if err != nil {
		t.Fatalf("ToTUIBlocksWithOptions() returned unexpected error: %v", err)
	}
	row := blocks[3].Rows[0].([]any)
	if row[0] != "2026-04-03T09:00:00+09:00" {
		t.Fatalf("expected table row time override, got %#v", row[0])
	}
}
