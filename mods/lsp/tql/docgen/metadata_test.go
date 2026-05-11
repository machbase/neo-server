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

func TestParseDocFrontmatterDraft(t *testing.T) {
	doc, err := parseDoc(`---
draft: true
---
# HIDDEN

## Kind

helper

## Category

internal

## Signatures

`+"```text"+`
HIDDEN()
`+"```"+`

## Slots

| Slot | Required | Repeat | Accepts | Suggestions |
| --- | --- | --- | --- | --- |

## Description

Hidden draft.

## Examples

`+"```js"+`
HIDDEN()
`+"```"+`

## Related
`, docTarget{Name: "HIDDEN"})
	if err != nil {
		t.Fatalf("parseDoc returned error: %v", err)
	}
	if !doc.Draft {
		t.Fatal("expected draft flag")
	}
}

func TestParseDocRoleVariants(t *testing.T) {
	doc, err := parseDoc(`# CSV

## Kind

statement source_or_sink

## Category

csv source or encoder

## Signatures

`+"```text"+`
CSV(input, options...)
CSV(options...)
`+"```"+`

## Slots

| Slot | Required | Repeat | Accepts | Suggestions |
| --- | --- | --- | --- | --- |
| input | no | no | stream|string|helper:file|helper:payload | file, payload |
| options | no | yes | helper | field, charset, nullValue |

## Description

CSV draft.

## Examples

`+"```js"+`
CSV()
`+"```"+`

## Related

file, payload

## Source

### Slots

| Slot | Required | Repeat | Accepts | Suggestions |
| --- | --- | --- | --- | --- |
| input | yes | no | helper:file|helper:payload | file, payload |
| options | no | yes | helper | field, charset |

### Description

CSV source.

## Sink

### Slots

| Slot | Required | Repeat | Accepts | Suggestions |
| --- | --- | --- | --- | --- |
| options | no | yes | helper | nullValue |

### Description

CSV sink.
`, docTarget{Name: "CSV"})
	if err != nil {
		t.Fatalf("parseDoc returned error: %v", err)
	}
	if len(doc.Roles) != 2 {
		t.Fatalf("expected 2 role variants, got %d", len(doc.Roles))
	}
	if got := doc.Roles["source"].Slots[0].Name; got != "input" {
		t.Fatalf("expected source input slot, got %q", got)
	}
	if got := doc.Roles["sink"].Slots[0].Suggestions[0]; got != "nullValue" {
		t.Fatalf("expected sink nullValue suggestion, got %q", got)
	}
}
