package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type parsedDoc struct {
	Label       string
	Draft       bool
	Kind        string
	Category    string
	Signatures  []parsedSignature
	Slots       []parsedSlot
	Description string
	Markdown    string
	Related     []string
	Roles       map[string]parsedDocVariant
}

type parsedDocVariant struct {
	Role        string
	Kind        string
	Category    string
	Signatures  []parsedSignature
	Slots       []parsedSlot
	Description string
	Markdown    string
	Related     []string
}

type parsedSignature struct {
	Label      string
	Parameters []string
}

type parsedSlot struct {
	Name        string
	Required    bool
	Repeat      bool
	Accepts     string
	Suggestions []string
}

func generateDocs(root string, targets map[string]docTarget) (int, error) {
	docs, err := parseDocs(filepath.Join(root, docRootName), targets)
	if err != nil {
		return 0, err
	}
	if err := writeGeneratedDocs(filepath.Join(root, "docs_gen.go"), docs); err != nil {
		return 0, err
	}
	return len(docs), nil
}

func parseDocs(docRoot string, targets map[string]docTarget) ([]parsedDoc, error) {
	docs := make([]parsedDoc, 0, len(targets))
	names := make([]string, 0, len(targets))
	for name := range targets {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		target := targets[name]
		path := filepath.Join(docRoot, target.Directory, target.Name+".md")
		content, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		doc, err := parseDoc(string(content), target)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", filepath.ToSlash(filepath.Join(docRootName, target.Directory, target.Name+".md")), err)
		}
		docs = append(docs, doc)
	}
	return docs, nil
}

func parseDoc(markdown string, target docTarget) (parsedDoc, error) {
	frontmatter, body := parseFrontmatter(markdown)
	markdown = body
	sections := splitSections(markdown)
	label := firstHeading(markdown)
	if label == "" {
		return parsedDoc{}, fmt.Errorf("missing top-level heading")
	}
	if label != target.Name {
		return parsedDoc{}, fmt.Errorf("heading %q does not match %q", label, target.Name)
	}
	for _, required := range []string{"Kind", "Category", "Signatures", "Slots", "Description", "Examples", "Related"} {
		if _, ok := sections[required]; !ok {
			return parsedDoc{}, fmt.Errorf("missing %q section", required)
		}
	}
	doc := parsedDoc{
		Label:       label,
		Draft:       frontmatter["draft"] == "true",
		Kind:        strings.TrimSpace(sections["Kind"]),
		Category:    strings.TrimSpace(sections["Category"]),
		Signatures:  parseSignatures(sections["Signatures"]),
		Slots:       parseSlots(sections["Slots"]),
		Description: strings.TrimSpace(sections["Description"]),
		Markdown:    strings.TrimSpace(markdown),
		Related:     parseCSVList(sections["Related"]),
		Roles:       parseRoleVariants(label, sections),
	}
	if len(doc.Signatures) == 0 {
		return parsedDoc{}, fmt.Errorf("missing signature")
	}
	return doc, nil
}

func parseFrontmatter(markdown string) (map[string]string, string) {
	metadata := make(map[string]string)
	markdown = strings.TrimPrefix(markdown, "\ufeff")
	if !strings.HasPrefix(markdown, "---\n") && !strings.HasPrefix(markdown, "---\r\n") {
		return metadata, markdown
	}
	lines := strings.Split(markdown, "\n")
	end := -1
	for idx := 1; idx < len(lines); idx++ {
		if strings.TrimSpace(lines[idx]) == "---" {
			end = idx
			break
		}
	}
	if end < 0 {
		return metadata, markdown
	}
	for _, line := range lines[1:end] {
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		metadata[strings.ToLower(strings.TrimSpace(key))] = strings.ToLower(strings.Trim(strings.TrimSpace(value), `"'`))
	}
	return metadata, strings.TrimLeft(strings.Join(lines[end+1:], "\n"), "\r\n")
}

func parseRoleVariants(label string, sections map[string]string) map[string]parsedDocVariant {
	roles := make(map[string]parsedDocVariant)
	for _, role := range []string{"source", "map", "sink"} {
		section, ok := sections[roleTitle(role)]
		if !ok {
			continue
		}
		subsections := splitSubsections(section)
		variant := parsedDocVariant{
			Role:        role,
			Kind:        sectionOrDefault(subsections, "Kind", "statement "+role),
			Category:    sectionOrDefault(subsections, "Category", strings.TrimSpace(sections["Category"])),
			Signatures:  parseSignatures(sectionOrDefault(subsections, "Signatures", sections["Signatures"])),
			Slots:       parseSlots(sectionOrDefault(subsections, "Slots", sections["Slots"])),
			Description: sectionOrDefault(subsections, "Description", sections["Description"]),
			Related:     parseCSVList(sectionOrDefault(subsections, "Related", sections["Related"])),
		}
		variant.Markdown = renderVariantMarkdown(label, variant)
		roles[role] = variant
	}
	if len(roles) == 0 {
		return nil
	}
	return roles
}

func roleTitle(role string) string {
	return strings.ToUpper(role[:1]) + role[1:]
}

func splitSubsections(markdown string) map[string]string {
	sections := make(map[string]string)
	current := ""
	var builder strings.Builder
	flush := func() {
		if current != "" {
			sections[current] = strings.TrimSpace(builder.String())
			builder.Reset()
		}
	}
	for _, line := range strings.Split(markdown, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "### ") {
			flush()
			current = strings.TrimSpace(strings.TrimPrefix(trimmed, "### "))
			continue
		}
		if current != "" {
			builder.WriteString(line)
			builder.WriteByte('\n')
		}
	}
	flush()
	return sections
}

func sectionOrDefault(sections map[string]string, name string, fallback string) string {
	if section := strings.TrimSpace(sections[name]); section != "" {
		return section
	}
	return strings.TrimSpace(fallback)
}

func renderVariantMarkdown(label string, variant parsedDocVariant) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n## Kind\n\n%s\n\n## Category\n\n%s\n\n## Signatures\n\n```text\n", label, variant.Kind, variant.Category)
	for _, signature := range variant.Signatures {
		fmt.Fprintln(&b, signature.Label)
	}
	b.WriteString("```\n\n## Slots\n\n| Slot | Required | Repeat | Accepts | Suggestions |\n| --- | --- | --- | --- | --- |\n")
	if len(variant.Slots) == 0 {
		b.WriteString("| none | no | no | none | none |\n")
	} else {
		for _, slot := range variant.Slots {
			required := "no"
			if slot.Required {
				required = "yes"
			}
			repeat := "no"
			if slot.Repeat {
				repeat = "yes"
			}
			fmt.Fprintf(&b, "| %s | %s | %s | %s | %s |\n", slot.Name, required, repeat, slot.Accepts, strings.Join(slot.Suggestions, ", "))
		}
	}
	fmt.Fprintf(&b, "\n## Description\n\n%s\n", variant.Description)
	return strings.TrimSpace(b.String())
}

func firstHeading(markdown string) string {
	for _, line := range strings.Split(markdown, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "# "))
		}
	}
	return ""
}

func splitSections(markdown string) map[string]string {
	sections := make(map[string]string)
	current := ""
	var builder strings.Builder
	flush := func() {
		if current != "" {
			sections[current] = strings.TrimSpace(builder.String())
			builder.Reset()
		}
	}
	for _, line := range strings.Split(markdown, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "## ") && !strings.HasPrefix(trimmed, "### ") {
			flush()
			current = strings.TrimSpace(strings.TrimPrefix(trimmed, "## "))
			continue
		}
		if current != "" {
			builder.WriteString(line)
			builder.WriteByte('\n')
		}
	}
	flush()
	return sections
}

func parseSignatures(section string) []parsedSignature {
	body := fencedBody(section)
	if body == "" {
		body = section
	}
	sigs := make([]parsedSignature, 0)
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		sigs = append(sigs, parsedSignature{Label: line, Parameters: signatureParameters(line)})
	}
	return sigs
}

func fencedBody(section string) string {
	lines := strings.Split(section, "\n")
	inFence := false
	body := make([]string, 0)
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			if inFence {
				break
			}
			inFence = true
			continue
		}
		if inFence {
			body = append(body, line)
		}
	}
	return strings.TrimSpace(strings.Join(body, "\n"))
}

func signatureParameters(signature string) []string {
	open := strings.Index(signature, "(")
	close := strings.LastIndex(signature, ")")
	if open < 0 || close < open {
		return nil
	}
	inside := strings.TrimSpace(signature[open+1 : close])
	if inside == "" {
		return nil
	}
	parts := strings.Split(inside, ",")
	params := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		part = strings.Trim(part, "[]")
		part = strings.TrimSuffix(part, "...")
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		fields := strings.Fields(part)
		params = append(params, fields[len(fields)-1])
	}
	return params
}

func parseSlots(section string) []parsedSlot {
	slots := make([]parsedSlot, 0)
	for _, line := range strings.Split(section, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "|") || strings.Contains(line, "---") || strings.Contains(line, "Slot | Required") {
			continue
		}
		cols := normalizeSlotColumns(splitTableRow(line))
		if len(cols) != 5 || strings.EqualFold(cols[0], "none") {
			continue
		}
		slots = append(slots, parsedSlot{
			Name:        cols[0],
			Required:    parseBoolWord(cols[1]),
			Repeat:      parseBoolWord(cols[2]),
			Accepts:     cols[3],
			Suggestions: parseCSVList(cols[4]),
		})
	}
	return slots
}

func splitTableRow(line string) []string {
	line = strings.Trim(line, "|")
	parts := strings.Split(line, "|")
	cols := make([]string, 0, len(parts))
	for _, part := range parts {
		cols = append(cols, strings.TrimSpace(part))
	}
	return cols
}

func normalizeSlotColumns(cols []string) []string {
	if len(cols) <= 5 {
		return cols
	}
	normalized := []string{cols[0], cols[1], cols[2], strings.Join(cols[3:len(cols)-1], "|"), cols[len(cols)-1]}
	return normalized
}

func parseBoolWord(value string) bool {
	return strings.EqualFold(strings.TrimSpace(value), "yes") || strings.EqualFold(strings.TrimSpace(value), "true")
}

func parseCSVList(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" || strings.EqualFold(value, "TODO") || strings.EqualFold(value, "none") {
		return nil
	}
	parts := strings.Split(value, ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.TrimSpace(part)
		if item == "" || strings.EqualFold(item, "TODO") || strings.EqualFold(item, "none") {
			continue
		}
		items = append(items, item)
	}
	return items
}

func writeGeneratedDocs(path string, docs []parsedDoc) error {
	var b bytes.Buffer
	b.WriteString("// Code generated by go generate ./mods/lsp/tql; DO NOT EDIT.\n\n")
	b.WriteString("package tql\n\n")
	b.WriteString("var generatedTqlDocs = map[string]tqlDocInfo{\n")
	for _, doc := range docs {
		fmt.Fprintf(&b, "\t%s: {\n", quote(doc.Label))
		fmt.Fprintf(&b, "\t\tLabel: %s,\n", quote(doc.Label))
		if doc.Draft {
			b.WriteString("\t\tDraft: true,\n")
		}
		fmt.Fprintf(&b, "\t\tKind: %s,\n", quote(doc.Kind))
		fmt.Fprintf(&b, "\t\tCategory: %s,\n", quote(doc.Category))
		writeGeneratedSignatures(&b, doc.Signatures)
		writeGeneratedSlots(&b, doc.Slots)
		fmt.Fprintf(&b, "\t\tDescription: %s,\n", quote(doc.Description))
		fmt.Fprintf(&b, "\t\tMarkdown: %s,\n", quote(doc.Markdown))
		writeGeneratedStringSlice(&b, "Related", doc.Related, 2)
		writeGeneratedRoles(&b, doc.Roles)
		b.WriteString("\t},\n")
	}
	b.WriteString("}\n")
	return os.WriteFile(path, b.Bytes(), 0o644)
}

func writeGeneratedRoles(b *bytes.Buffer, roles map[string]parsedDocVariant) {
	if len(roles) == 0 {
		return
	}
	names := make([]string, 0, len(roles))
	for name := range roles {
		names = append(names, name)
	}
	sort.Strings(names)
	b.WriteString("\t\tRoles: map[string]tqlDocVariant{\n")
	for _, name := range names {
		role := roles[name]
		fmt.Fprintf(b, "\t\t\t%s: {\n", quote(name))
		fmt.Fprintf(b, "\t\t\t\tRole: %s,\n", quote(role.Role))
		fmt.Fprintf(b, "\t\t\t\tKind: %s,\n", quote(role.Kind))
		fmt.Fprintf(b, "\t\t\t\tCategory: %s,\n", quote(role.Category))
		writeGeneratedVariantSignatures(b, role.Signatures)
		writeGeneratedVariantSlots(b, role.Slots)
		fmt.Fprintf(b, "\t\t\t\tDescription: %s,\n", quote(role.Description))
		fmt.Fprintf(b, "\t\t\t\tMarkdown: %s,\n", quote(role.Markdown))
		writeGeneratedStringSlice(b, "Related", role.Related, 4)
		b.WriteString("\t\t\t},\n")
	}
	b.WriteString("\t\t},\n")
}

func writeGeneratedVariantSignatures(b *bytes.Buffer, signatures []parsedSignature) {
	b.WriteString("\t\t\t\tSignatures: []tqlDocSignature{\n")
	for _, signature := range signatures {
		fmt.Fprintf(b, "\t\t\t\t\t{Label: %s", quote(signature.Label))
		if len(signature.Parameters) > 0 {
			b.WriteString(", Parameters: ")
			writeInlineStringSlice(b, signature.Parameters)
		}
		b.WriteString("},\n")
	}
	b.WriteString("\t\t\t\t},\n")
}

func writeGeneratedVariantSlots(b *bytes.Buffer, slots []parsedSlot) {
	b.WriteString("\t\t\t\tSlots: []tqlDocSlot{\n")
	for _, slot := range slots {
		fmt.Fprintf(b, "\t\t\t\t\t{Name: %s, Required: %t, Repeat: %t, Accepts: %s", quote(slot.Name), slot.Required, slot.Repeat, quote(slot.Accepts))
		if len(slot.Suggestions) > 0 {
			b.WriteString(", Suggestions: ")
			writeInlineStringSlice(b, slot.Suggestions)
		}
		b.WriteString("},\n")
	}
	b.WriteString("\t\t\t\t},\n")
}

func writeGeneratedSignatures(b *bytes.Buffer, signatures []parsedSignature) {
	b.WriteString("\t\tSignatures: []tqlDocSignature{\n")
	for _, signature := range signatures {
		fmt.Fprintf(b, "\t\t\t{Label: %s", quote(signature.Label))
		if len(signature.Parameters) > 0 {
			b.WriteString(", Parameters: ")
			writeInlineStringSlice(b, signature.Parameters)
		}
		b.WriteString("},\n")
	}
	b.WriteString("\t\t},\n")
}

func writeGeneratedSlots(b *bytes.Buffer, slots []parsedSlot) {
	b.WriteString("\t\tSlots: []tqlDocSlot{\n")
	for _, slot := range slots {
		fmt.Fprintf(b, "\t\t\t{Name: %s, Required: %t, Repeat: %t, Accepts: %s", quote(slot.Name), slot.Required, slot.Repeat, quote(slot.Accepts))
		if len(slot.Suggestions) > 0 {
			b.WriteString(", Suggestions: ")
			writeInlineStringSlice(b, slot.Suggestions)
		}
		b.WriteString("},\n")
	}
	b.WriteString("\t\t},\n")
}

func writeGeneratedStringSlice(b *bytes.Buffer, name string, values []string, tabs int) {
	indent := strings.Repeat("\t", tabs)
	if len(values) == 0 {
		return
	}
	fmt.Fprintf(b, "%s%s: ", indent, name)
	writeInlineStringSlice(b, values)
	b.WriteString(",\n")
}

func writeInlineStringSlice(b *bytes.Buffer, values []string) {
	b.WriteString("[]string{")
	for idx, value := range values {
		if idx > 0 {
			b.WriteString(", ")
		}
		b.WriteString(quote(value))
	}
	b.WriteString("}")
}

func quote(value string) string {
	return strconv.Quote(value)
}
