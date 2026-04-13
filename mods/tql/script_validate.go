package tql

import "fmt"

func ValidateScriptStructure(script *TQLScript) error {
	if script == nil {
		return newScriptError("nil_script", nil, "script is nil", nil)
	}

	var codes []*Statement
	for _, stmt := range script.Statements {
		if stmt.IsCode() {
			codes = append(codes, stmt)
		}
	}

	if len(codes) == 0 {
		return newScriptError("no_source", nil, "no source exists", nil)
	}
	if len(codes) == 1 {
		return newScriptError("no_sink", codes[0], "no sink exists", nil)
	}

	head := codes[0]
	tail := codes[len(codes)-1]

	if !isApplicableForSource(head.Kind) {
		return newScriptError("invalid_source", head, fmt.Sprintf("%q is not applicable for SRC", head.Name), nil)
	}
	if !isApplicableForSink(tail.Kind) {
		return newScriptError("invalid_sink", tail, fmt.Sprintf("%q is not applicable for SINK", tail.Name), nil)
	}

	for _, stmt := range codes[1 : len(codes)-1] {
		if !isApplicableForMap(stmt.Kind) {
			return newScriptError("invalid_map", stmt, fmt.Sprintf("%q is not applicable for MAP", stmt.Name), nil)
		}
	}

	return nil
}

func isApplicableForSource(kind StatementKind) bool {
	switch kind {
	case StatementSource, StatementSourceOrMap, StatementSourceOrSink:
		return true
	default:
		return false
	}
}

func isApplicableForMap(kind StatementKind) bool {
	switch kind {
	case StatementMap, StatementSourceOrMap:
		return true
	default:
		return false
	}
}

func isApplicableForSink(kind StatementKind) bool {
	switch kind {
	case StatementSink, StatementSourceOrSink:
		return true
	default:
		return false
	}
}
