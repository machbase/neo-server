package main

import "testing"

func TestParseDocRequiresFixedSections(t *testing.T) {
	_, err := parseDoc(`# SQL

## Kind

statement source
`, docTarget{Name: "SQL"})
	if err == nil {
		t.Fatal("expected missing section error")
	}
}

func TestParseDocSlotAcceptsPipeSeparatedAccepts(t *testing.T) {
	doc, err := parseDoc(`# CSV

## Kind

statement source_or_sink

## Category

csv source or encoder

## Signatures

`+"```text"+`
CSV(input, options...)
`+"```"+`

## Slots

| Slot | Required | Repeat | Accepts | Suggestions |
| --- | --- | --- | --- | --- |
| input | no | no | stream|string|helper:file|helper:payload | file, payload |
| options | no | yes | helper | field, charset |

## Description

CSV draft.

## Examples

### Basic

`+"```js"+`
CSV()
`+"```"+`

## Related

file, payload
`, docTarget{Name: "CSV"})
	if err != nil {
		t.Fatalf("parseDoc returned error: %v", err)
	}
	if len(doc.Slots) != 2 {
		t.Fatalf("expected 2 slots, got %d", len(doc.Slots))
	}
	if doc.Slots[0].Accepts != "stream|string|helper:file|helper:payload" {
		t.Fatalf("unexpected accepts value: %q", doc.Slots[0].Accepts)
	}
	if len(doc.Slots[0].Suggestions) != 2 || doc.Slots[0].Suggestions[0] != "file" || doc.Slots[0].Suggestions[1] != "payload" {
		t.Fatalf("unexpected suggestions: %+v", doc.Slots[0].Suggestions)
	}
}
