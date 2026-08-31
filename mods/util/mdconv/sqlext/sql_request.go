package sqlext

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/machbase/neo-server/v8/mods/util"
	"github.com/machbase/neo-server/v8/spi"
)

// Rows is the minimal row set abstraction used by shared SQL request helpers.
type Rows interface {
	Columns() ([]string, error)
	ColumnTypes() ([]*sql.ColumnType, error)
	Close() error
	Next() bool
	Scan(dest ...any) error
}

// QueryRequest is the shared SQL request payload used by server handlers and mdconv extensions.
type QueryRequest struct {
	SqlText         string    `json:"q"`
	Params          []any     `json:"p,omitempty"`
	Preview         int       `json:"preview,omitempty"`
	TimeFormat      string    `json:"timeformat,omitempty"`
	Timezone        string    `json:"tz,omitempty"`
	BinaryFormat    string    `json:"binaryformat,omitempty"`
	RowsFlattern    bool      `json:"rowsFlattern,omitempty"`
	RowsArray       bool      `json:"rowsArray,omitempty"`
	Transpose       bool      `json:"transpose,omitempty"`
	Precision       int       `json:"precision,omitempty"`
	RowNum          bool      `json:"rownum,omitempty"`
	Heading         bool      `json:"heading,omitempty"`
	Header          bool      `json:"header,omitempty"`
	Delimiter       string    `json:"delimiter,omitempty"`
	BoxStyle        string    `json:"boxStyle,omitempty"`
	BoxSeparateCols bool      `json:"boxSeparateColumns,omitempty"`
	BoxDrawBorder   bool      `json:"boxDrawBorder,omitempty"`
	Output          string    `json:"output,omitempty"`
	Format          string    `json:"format,omitempty"`
	ReplyTo         string    `json:"replyTo,omitempty"`
	Database        string    `json:"database,omitempty"`
	User            string    `json:"user,omitempty"`
	Password        string    `json:"password,omitempty"`
	AuthKey         string    `json:"authKey,omitempty"`
	NoCache         bool      `json:"noCache,omitempty"`
	NoMeta          bool      `json:"noMeta,omitempty"`
	NoFormat        bool      `json:"noFormat,omitempty"`
	Cache           bool      `json:"cache,omitempty"`
	Truncate        int       `json:"truncate,omitempty"`
	Hook            QueryHook `json:"-"`
}

type QueryHook struct {
	SetContentType     func(string)
	SetContentEncoding func(string)
	SetStatusCode      func(int)
	SetUserMessage     func(string)
}

var tqlBaseURL = ""
var tqlDoRequest = func(req *http.Request) (*http.Response, error) {
	return http.DefaultClient.Do(req)
}

func NewQueryRequest() *QueryRequest {
	return &QueryRequest{Output: "box", Header: true, Heading: true, TimeFormat: "ns", Preview: 10}
}

func (q *QueryRequest) DecodeJSON(r io.Reader) error {
	var body struct {
		SqlText      string `json:"q"`
		Params       any    `json:"p"`
		Preview      int    `json:"preview"`
		TimeFormat   string `json:"timeformat"`
		Timezone     string `json:"tz"`
		BinaryFormat string `json:"binaryformat"`
		RowsFlattern bool   `json:"rowsFlattern"`
		RowsArray    bool   `json:"rowsArray"`
		Transpose    bool   `json:"transpose"`
		Precision    int    `json:"precision"`
		RowNum       bool   `json:"rownum"`
		Heading      bool   `json:"heading"`
		Header       bool   `json:"header"`
		Delimiter    string `json:"delimiter"`
		BoxStyle     string `json:"boxStyle"`
		BoxSeparate  bool   `json:"boxSeparateColumns"`
		BoxBorder    bool   `json:"boxDrawBorder"`
		Output       string `json:"output"`
		Format       string `json:"format"`
		ReplyTo      string `json:"replyTo"`
		Database     string `json:"database"`
		User         string `json:"user"`
		Password     string `json:"password"`
		AuthKey      string `json:"authKey"`
		NoCache      bool   `json:"noCache"`
		NoMeta       bool   `json:"noMeta"`
		NoFormat     bool   `json:"noFormat"`
		Cache        bool   `json:"cache"`
		Truncate     int    `json:"truncate"`
	}
	if err := json.NewDecoder(r).Decode(&body); err != nil {
		return err
	}
	q.SqlText = body.SqlText
	q.Preview = body.Preview
	q.TimeFormat = body.TimeFormat
	q.Timezone = body.Timezone
	q.BinaryFormat = body.BinaryFormat
	q.RowsFlattern = body.RowsFlattern
	q.RowsArray = body.RowsArray
	q.Transpose = body.Transpose
	q.Precision = body.Precision
	q.RowNum = body.RowNum
	q.Heading = body.Heading
	q.Header = body.Header
	q.Delimiter = body.Delimiter
	q.BoxStyle = body.BoxStyle
	q.BoxSeparateCols = body.BoxSeparate
	q.BoxDrawBorder = body.BoxBorder
	q.Output = body.Output
	q.Format = body.Format
	q.ReplyTo = body.ReplyTo
	q.Database = body.Database
	q.User = body.User
	q.Password = body.Password
	q.AuthKey = body.AuthKey
	q.NoCache = body.NoCache
	q.NoMeta = body.NoMeta
	q.NoFormat = body.NoFormat
	q.Cache = body.Cache
	q.Truncate = body.Truncate
	if body.Params == nil {
		q.Params = nil
		return nil
	}
	params, err := ParseQueryParams(body.Params)
	if err != nil {
		return err
	}
	q.Params = params
	return nil
}

func (q *QueryRequest) DecodeQuery(v url.Values) error {
	if v == nil {
		return nil
	}
	q.SqlText = v.Get("q")
	q.Preview = parseInt(v.Get("preview"))
	q.TimeFormat = v.Get("timeformat")
	q.Timezone = v.Get("tz")
	q.BinaryFormat = v.Get("binaryformat")
	q.RowsFlattern = parseBool(v.Get("rowsFlattern"))
	q.RowsArray = parseBool(v.Get("rowsArray"))
	q.Transpose = parseBool(v.Get("transpose"))
	q.Precision = parseInt(v.Get("precision"))
	q.RowNum = parseBool(v.Get("rownum"))
	q.Heading = parseBool(v.Get("heading"))
	q.Header = parseBool(v.Get("header"))
	q.Delimiter = v.Get("delimiter")
	q.BoxStyle = v.Get("boxStyle")
	q.BoxSeparateCols = parseBool(v.Get("boxSeparateColumns"))
	q.BoxDrawBorder = parseBool(v.Get("boxDrawBorder"))
	q.Output = v.Get("output")
	q.Format = v.Get("format")
	q.ReplyTo = v.Get("replyTo")
	q.Database = v.Get("database")
	q.User = v.Get("user")
	q.Password = v.Get("password")
	q.AuthKey = v.Get("authKey")
	q.NoCache = parseBool(v.Get("noCache"))
	q.NoMeta = parseBool(v.Get("noMeta"))
	q.NoFormat = parseBool(v.Get("noFormat"))
	q.Cache = parseBool(v.Get("cache"))
	q.Truncate = parseInt(v.Get("truncate"))
	if raw := v.Get("p"); raw != "" {
		params, err := ParseQueryParams(raw)
		if err != nil {
			return err
		}
		q.Params = params
	}
	return nil
}

func (q *QueryRequest) DecodePostForm(v url.Values) error {
	return q.DecodeQuery(v)
}

func ParseQueryParams(raw any) ([]any, error) {
	switch value := raw.(type) {
	case nil:
		return nil, nil
	case string:
		text := strings.TrimSpace(value)
		if text == "" {
			return nil, nil
		}
		var parsed []any
		if err := json.Unmarshal([]byte(text), &parsed); err != nil {
			return nil, fmt.Errorf("invalid p: %w", err)
		}
		return normalizeParams(parsed)
	case []any:
		return normalizeParams(value)
	case []string:
		res := make([]any, 0, len(value))
		for _, item := range value {
			res = append(res, item)
		}
		return res, nil
	default:
		return nil, fmt.Errorf("invalid p: unsupported type %T", raw)
	}
}

func normalizeParams(params []any) ([]any, error) {
	res := make([]any, 0, len(params))
	for _, item := range params {
		norm, err := NormalizeQueryParamValue(item)
		if err != nil {
			return nil, fmt.Errorf("invalid p: %w", err)
		}
		res = append(res, norm)
	}
	return res, nil
}

func NormalizeQueryParamValue(value any) (any, error) {
	switch v := value.(type) {
	case nil:
		return nil, nil
	case string, bool, int, int8, int16, int32, int64:
		return v, nil
	case float32:
		if float64(v) == math.Trunc(float64(v)) {
			return int(v), nil
		}
		return float64(v), nil
	case float64:
		if v == math.Trunc(v) {
			return int(v), nil
		}
		return v, nil
	case json.Number:
		if strings.ContainsAny(v.String(), ".eE") {
			f, err := strconv.ParseFloat(v.String(), 64)
			if err != nil {
				return nil, err
			}
			return f, nil
		}
		if i, err := strconv.ParseInt(v.String(), 10, 64); err == nil {
			return i, nil
		}
		if i, err := strconv.Atoi(v.String()); err == nil {
			return i, nil
		}
		return nil, fmt.Errorf("invalid syntax")
	case []any, []string:
		return nil, fmt.Errorf("invalid p: scalar required, got slice")
	case map[string]any:
		return nil, fmt.Errorf("invalid p: scalar required, got map")
	default:
		return nil, fmt.Errorf("unsupported bind parameter type %T", value)
	}
}

func (q *QueryRequest) Execute(ctx context.Context, w io.Writer, hook *QueryHook) error {
	if q.SqlText == "" {
		return fmt.Errorf("query sql is empty")
	}
	output := normalizeRequestedOutputFormat(q.Output, q.Format)
	if output == "none" {
		return nil
	}
	if hook == nil {
		hook = &QueryHook{}
	}

	stmtType := spi.DetectSQLStatementType(q.SqlText)
	return q.executeWithTQL(ctx, w, hook, stmtType, output)
}

func (q *QueryRequest) executeWithTQL(ctx context.Context, w io.Writer, hook *QueryHook, stmtType spi.SQLStatementType, output string) error {
	var body strings.Builder
	body.WriteString("SQL(")
	body.WriteString(quoteTQLString(q.SqlText))
	if len(q.Params) > 0 {
		for _, param := range q.Params {
			body.WriteString(", ")
			body.WriteString(quoteTQLValue(param))
		}
	}
	body.WriteString(")")
	if q.Preview > 0 {
		body.WriteString("\nTAKE(")
		body.WriteString(strconv.Itoa(q.Preview))
		body.WriteString(")")
	}

	switch strings.ToLower(output) {
	case "json":
		body.WriteString("\nJSON(")
	case "ndjson":
		body.WriteString("\nNDJSON(")
	case "csv":
		body.WriteString("\nCSV(")
	default:
		body.WriteString("\nBOX(")
	}

	options := make([]string, 0, 8)
	if q.TimeFormat != "" {
		options = append(options, fmt.Sprintf("timeformat('%s')", escapeTQLString(q.TimeFormat)))
	}
	if q.Timezone != "" {
		options = append(options, fmt.Sprintf("tz('%s')", escapeTQLString(q.Timezone)))
	}
	if q.BinaryFormat != "" {
		options = append(options, fmt.Sprintf("binaryformat('%s')", escapeTQLString(q.BinaryFormat)))
	}
	if q.Precision > 0 {
		options = append(options, fmt.Sprintf("precision(%d)", q.Precision))
	}
	if q.RowNum {
		options = append(options, "rownum(true)")
	}
	if q.RowsFlattern {
		options = append(options, "rowsFlatten(true)")
	}
	if q.RowsArray {
		options = append(options, "rowsArray(true)")
	}
	if q.Heading {
		options = append(options, "heading(true)")
	}
	if q.Header {
		options = append(options, "header(true)")
	}
	if q.BoxStyle != "" {
		options = append(options, fmt.Sprintf("boxStyle('%s')", escapeTQLString(q.BoxStyle)))
	}
	if len(options) > 0 {
		body.WriteString(strings.Join(options, ", "))
	}
	body.WriteString(")")

	if tqlBaseURL == "" {
		tqlBaseURL = spi.DefaultHttpEndpoint()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tqlBaseURL+"/db/tql", strings.NewReader(body.String()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "text/plain")

	resp, err := tqlDoRequest(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("tql request failed: %s", resp.Status)
	}
	_, err = io.Copy(w, resp.Body)
	if err != nil {
		return err
	}
	if hook.SetUserMessage != nil {
		hook.SetUserMessage(userMessageForStatement(stmtType, 0))
	}
	return nil
}

func quoteTQLString(text string) string {
	return "{<<END_OF_SQL\n" + text + "\nEND_OF_SQL}"
}

func quoteTQLValue(value any) string {
	switch v := value.(type) {
	case string:
		return fmt.Sprintf("'%s'", escapeTQLString(v))
	case bool:
		if v {
			return "true"
		}
		return "false"
	case nil:
		return "null"
	case int:
		return strconv.Itoa(v)
	case int8:
		return strconv.FormatInt(int64(v), 10)
	case int16:
		return strconv.FormatInt(int64(v), 10)
	case int32:
		return strconv.FormatInt(int64(v), 10)
	case int64:
		return strconv.FormatInt(v, 10)
	case float32:
		return strconv.FormatFloat(float64(v), 'f', -1, 32)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	default:
		return fmt.Sprintf("'%s'", escapeTQLString(fmt.Sprint(v)))
	}
}

func escapeTQLString(text string) string {
	return strings.ReplaceAll(strings.ReplaceAll(text, `\`, `\\`), `'`, `\\'`)
}

func normalizeRequestedOutputFormat(output, format string) string {
	output = strings.TrimSpace(output)
	if output == "" {
		output = strings.TrimSpace(format)
	}
	if output == "" {
		output = "box"
	}
	return strings.ToLower(output)
}

func contentTypeForFormat(output string) string {
	switch output {
	case "json":
		return "application/json"
	case "ndjson":
		return "application/x-ndjson"
	case "csv":
		return "text/csv; charset=utf-8"
	case "markdown":
		return "text/markdown"
	case "html":
		return "text/html"
	case "text":
		return "text/plain"
	default:
		return "text/plain"
	}
}

func renderRowsByFormat(w io.Writer, output string, heading bool, rows Rows, columnNames []string, columnTypes []*sql.ColumnType, timeFormat, tzName string, timeFormatter *util.TimeFormatter, binaryFormatter *util.BinaryFormatter, stmtType spi.SQLStatementType, limit int) (int64, error) {
	if output == "none" {
		return 0, nil
	}

	switch strings.ToLower(output) {
	case "json":
		return renderJSONOutput(w, rows, columnNames, columnTypes, timeFormatter, binaryFormatter, stmtType, limit)
	case "ndjson":
		return renderNDJSONOutput(w, rows, columnNames, timeFormatter, binaryFormatter, limit)
	case "csv":
		return renderCSVOutput(w, heading, rows, columnNames, timeFormatter, binaryFormatter, limit)
	default:
		return renderTableOutput(w, heading, rows, columnNames, columnTypes, timeFormat, tzName, timeFormatter, binaryFormatter, stmtType, limit)
	}
}

func shouldAppendTimezoneToHeader(timeFormat string) bool {
	switch strings.ToLower(strings.TrimSpace(timeFormat)) {
	case "", "ns", "us", "ms", "s", "ns.str", "us.str", "ms.str", "s.str":
		return false
	default:
		return true
	}
}

func formatTableHeaderLabel(name, columnTypeName, timeFormat, tzName string) string {
	if !strings.EqualFold(strings.TrimSpace(columnTypeName), "datetime") {
		return name
	}
	if !shouldAppendTimezoneToHeader(timeFormat) {
		return name
	}
	if strings.TrimSpace(tzName) == "" {
		return name
	}
	return fmt.Sprintf("%s(%s)", name, tzName)
}

func renderTableOutput(w io.Writer, heading bool, rows Rows, columnNames []string, columnTypes []*sql.ColumnType, timeFormat, tzName string, timeFormatter *util.TimeFormatter, binaryFormatter *util.BinaryFormatter, stmtType spi.SQLStatementType, limit int) (int64, error) {
	tw := table.NewWriter()
	tw.SetOutputMirror(w)
	style := table.StyleDefault
	style.Options.SeparateColumns = true
	style.Options.DrawBorder = true
	tw.SetStyle(style)

	if heading && len(columnNames) > 0 {
		header := make([]any, len(columnNames))
		for i, name := range columnNames {
			columnTypeName := ""
			if i < len(columnTypes) && columnTypes[i] != nil {
				columnTypeName = columnTypes[i].DatabaseTypeName()
			}
			header[i] = formatTableHeaderLabel(name, columnTypeName, timeFormat, tzName)
		}
		tw.AppendHeader(table.Row(header))
	}

	nrows := int64(0)
	rawRows := make([][]any, 0)
	for rows.Next() {
		nrows++
		dests := make([]any, len(columnNames))
		for i := range dests {
			dests[i] = new(any)
		}
		if err := rows.Scan(dests...); err != nil {
			return 0, err
		}
		values := make([]any, 0, len(dests))
		for _, dest := range dests {
			value := any(nil)
			if p, ok := dest.(*any); ok {
				value = *p
			}
			values = append(values, value)
		}
		rawRows = append(rawRows, values)
	}

	displayRows := previewRows(rawRows, limit)
	for i, values := range displayRows {
		if shouldRenderEllipsisRow(rawRows, limit, i) {
			tw.AppendRow(table.Row(makeEllipsisRow(len(columnNames))))
			continue
		}
		formatted := make([]any, 0, len(values))
		for _, value := range values {
			formatted = append(formatted, formatValueForTextOutput(value, timeFormatter, binaryFormatter))
		}
		tw.AppendRow(table.Row(formatted))
	}

	tw.Render()
	userMessage := userMessageForStatement(stmtType, nrows)
	if userMessage != "" {
		_, _ = io.WriteString(w, "\n")
		_, _ = io.WriteString(w, userMessage)
	}
	return nrows, nil
}

func renderJSONOutput(w io.Writer, rows Rows, columnNames []string, columnTypes []*sql.ColumnType, timeFormatter *util.TimeFormatter, binaryFormatter *util.BinaryFormatter, stmtType spi.SQLStatementType, limit int) (int64, error) {
	nrows := int64(0)
	rawRows := make([][]any, 0)
	_, _ = io.WriteString(w, `{"data":{"columns":`)
	if err := json.NewEncoder(w).Encode(columnNames); err != nil {
		return 0, err
	}
	_, _ = io.WriteString(w, `,"types":`)
	if err := json.NewEncoder(w).Encode(columnTypeNamesForColumns(columnNames, columnTypes)); err != nil {
		return 0, err
	}
	_, _ = io.WriteString(w, `,"rows":[`)
	for rows.Next() {
		nrows++
		values := make([]any, len(columnNames))
		for i := range values {
			values[i] = new(any)
		}
		if err := rows.Scan(values...); err != nil {
			return 0, err
		}
		raw := make([]any, 0, len(values))
		for _, value := range values {
			if p, ok := value.(*any); ok {
				raw = append(raw, *p)
			} else {
				raw = append(raw, nil)
			}
		}
		rawRows = append(rawRows, raw)
	}
	displayRows := previewRows(rawRows, limit)
	firstRow := true
	for i, values := range displayRows {
		if shouldRenderEllipsisRow(rawRows, limit, i) {
			continue
		}
		if !firstRow {
			_, _ = io.WriteString(w, ",")
		}
		firstRow = false
		rowValues := make([]any, 0, len(values))
		for _, value := range values {
			rowValues = append(rowValues, formatValueForJSONOutput(value, timeFormatter, binaryFormatter))
		}
		if err := json.NewEncoder(w).Encode(rowValues); err != nil {
			return 0, err
		}
	}
	reason := "success"
	if stmtType != spi.SQLStatementTypeOther {
		reason = userMessageForStatement(stmtType, nrows)
	}
	_, _ = io.WriteString(w, `],"success":true,"reason":"`)
	_, _ = io.WriteString(w, strings.ReplaceAll(reason, `"`, `\\"`))
	_, _ = io.WriteString(w, `","elapse":"0s"}`)
	return nrows, nil
}

func renderNDJSONOutput(w io.Writer, rows Rows, columnNames []string, timeFormatter *util.TimeFormatter, binaryFormatter *util.BinaryFormatter, limit int) (int64, error) {
	nrows := int64(0)
	rawRows := make([][]any, 0)
	for rows.Next() {
		nrows++
		values := make([]any, len(columnNames))
		for i := range values {
			values[i] = new(any)
		}
		if err := rows.Scan(values...); err != nil {
			return 0, err
		}
		raw := make([]any, 0, len(values))
		for _, value := range values {
			if p, ok := value.(*any); ok {
				raw = append(raw, *p)
			} else {
				raw = append(raw, nil)
			}
		}
		rawRows = append(rawRows, raw)
	}
	displayRows := previewRows(rawRows, limit)
	for i, values := range displayRows {
		if shouldRenderEllipsisRow(rawRows, limit, i) {
			continue
		}
		rowMap := make(map[string]any, len(columnNames))
		for i, name := range columnNames {
			if i < len(values) {
				rowMap[name] = formatValueForJSONOutput(values[i], timeFormatter, binaryFormatter)
			}
		}
		if err := json.NewEncoder(w).Encode(rowMap); err != nil {
			return 0, err
		}
	}
	return nrows, nil
}

func renderCSVOutput(w io.Writer, heading bool, rows Rows, columnNames []string, timeFormatter *util.TimeFormatter, binaryFormatter *util.BinaryFormatter, limit int) (int64, error) {
	nrows := int64(0)
	rawRows := make([][]any, 0)
	if heading && len(columnNames) > 0 {
		_, _ = fmt.Fprintf(w, "%s\n", strings.Join(columnNames, ","))
	}
	for rows.Next() {
		nrows++
		values := make([]any, len(columnNames))
		for i := range values {
			values[i] = new(any)
		}
		if err := rows.Scan(values...); err != nil {
			return 0, err
		}
		raw := make([]any, 0, len(values))
		for _, value := range values {
			if p, ok := value.(*any); ok {
				raw = append(raw, *p)
			} else {
				raw = append(raw, nil)
			}
		}
		rawRows = append(rawRows, raw)
	}
	displayRows := previewRows(rawRows, limit)
	for i, values := range displayRows {
		if shouldRenderEllipsisRow(rawRows, limit, i) {
			ellipsis := make([]string, len(columnNames))
			for j := range ellipsis {
				ellipsis[j] = "..."
			}
			_, _ = fmt.Fprintf(w, "%s\n", strings.Join(ellipsis, ","))
			continue
		}
		parts := make([]string, 0, len(values))
		for _, value := range values {
			parts = append(parts, formatValueForTextOutput(value, timeFormatter, binaryFormatter))
		}
		_, _ = fmt.Fprintf(w, "%s\n", strings.Join(parts, ","))
	}
	return nrows, nil
}

func columnTypeNamesForColumns(columnNames []string, columnTypes []*sql.ColumnType) []string {
	res := make([]string, len(columnNames))
	for i := range res {
		res[i] = "string"
		if len(columnTypes) > i && columnTypes[i] != nil {
			res[i] = columnTypes[i].DatabaseTypeName()
		}
	}
	return res
}

func previewRows(rows [][]any, limit int) [][]any {
	if limit <= 0 || len(rows) <= limit {
		return rows
	}
	headCount := limit / 2
	if headCount <= 0 {
		headCount = 1
	}
	tailCount := limit - headCount
	if tailCount <= 0 {
		tailCount = 1
	}
	if headCount+tailCount >= len(rows) {
		return rows
	}
	preview := make([][]any, 0, headCount+tailCount+1)
	preview = append(preview, rows[:headCount]...)
	preview = append(preview, makeEllipsisRow(len(rows[0])))
	preview = append(preview, rows[len(rows)-tailCount:]...)
	return preview
}

func shouldRenderEllipsisRow(rows [][]any, limit int, index int) bool {
	if limit <= 0 || len(rows) <= limit {
		return false
	}
	headCount := limit / 2
	if headCount <= 0 {
		headCount = 1
	}
	tailCount := limit - headCount
	if tailCount <= 0 {
		tailCount = 1
	}
	if headCount+tailCount >= len(rows) {
		return false
	}
	return index == headCount
}

func makeEllipsisRow(columnCount int) []any {
	row := make([]any, columnCount)
	for i := range row {
		row[i] = "..."
	}
	return row
}

func userMessageForStatement(stmtType spi.SQLStatementType, rowCount int64) string {
	switch stmtType {
	case spi.SQLStatementTypeShow, spi.SQLStatementTypeExplain:
		return spi.MakeUserMessage(spi.SQLStatementTypeSelect, rowCount)
	default:
		return spi.MakeUserMessage(stmtType, rowCount)
	}
}

func formatValueForJSONOutput(value any, timeFormatter *util.TimeFormatter, binaryFormatter *util.BinaryFormatter) any {
	if value == nil {
		return nil
	}
	if t, ok := value.(time.Time); ok {
		if timeFormatter != nil {
			return timeFormatter.Format(t)
		}
		return t.Format(time.RFC3339Nano)
	}
	if b, ok := value.([]byte); ok {
		if binaryFormatter != nil {
			return binaryFormatter.Format(b)
		}
		return string(b)
	}
	if s, ok := value.(fmt.Stringer); ok {
		return s.String()
	}
	return value
}

func formatValueForTextOutput(value any, timeFormatter *util.TimeFormatter, binaryFormatter *util.BinaryFormatter) string {
	if value == nil {
		return "NULL"
	}
	if t, ok := value.(time.Time); ok {
		if timeFormatter != nil {
			return timeFormatter.Format(t)
		}
		return t.Format(time.RFC3339Nano)
	}
	if b, ok := value.([]byte); ok {
		if binaryFormatter != nil {
			return binaryFormatter.Format(b)
		}
		return string(b)
	}
	if s, ok := value.(fmt.Stringer); ok {
		return s.String()
	}
	return fmt.Sprintf("%v", value)
}

func (q *QueryRequest) HandleQuery(w io.Writer, hook *QueryHook) error {
	return q.Execute(context.Background(), w, hook)
}

func (q *QueryRequest) ToJSON() ([]byte, error) {
	return json.Marshal(q)
}

func (q *QueryRequest) FromJSON(data []byte) error {
	return q.DecodeJSON(bytes.NewReader(data))
}

func NewQueryRequestFromJSON(data []byte) (*QueryRequest, error) {
	q := NewQueryRequest()
	return q, q.FromJSON(data)
}

func (q *QueryRequest) AddHook(hook *QueryHook) {
	if hook != nil {
		q.Hook = *hook
	}
}

func (q *QueryRequest) GetQueryText() string          { return q.SqlText }
func (q *QueryRequest) GetParams() []any              { return q.Params }
func (q *QueryRequest) SetQueryText(text string)      { q.SqlText = text }
func (q *QueryRequest) SetParams(params []any)        { q.Params = params }
func (q *QueryRequest) SetOutput(output string)       { q.Output = output }
func (q *QueryRequest) SetFormat(format string)       { q.Format = format }
func (q *QueryRequest) SetPreview(preview int)        { q.Preview = preview }
func (q *QueryRequest) SetTimeFormat(format string)   { q.TimeFormat = format }
func (q *QueryRequest) SetTimezone(tz string)         { q.Timezone = tz }
func (q *QueryRequest) SetBinaryFormat(format string) { q.BinaryFormat = format }
func (q *QueryRequest) SetRowsFlattern(value bool)    { q.RowsFlattern = value }
func (q *QueryRequest) SetRowsArray(value bool)       { q.RowsArray = value }
func (q *QueryRequest) SetTranspose(value bool)       { q.Transpose = value }
func (q *QueryRequest) SetPrecision(precision int)    { q.Precision = precision }
func (q *QueryRequest) SetRowNum(value bool)          { q.RowNum = value }
func (q *QueryRequest) SetHeading(value bool)         { q.Heading = value }
func (q *QueryRequest) SetHeader(value bool)          { q.Header = value }
func (q *QueryRequest) SetDelimiter(delimiter string) { q.Delimiter = delimiter }
func (q *QueryRequest) SetBoxStyle(style string)      { q.BoxStyle = style }
func (q *QueryRequest) SetBoxSeparateCols(value bool) { q.BoxSeparateCols = value }
func (q *QueryRequest) SetBoxDrawBorder(value bool)   { q.BoxDrawBorder = value }
func (q *QueryRequest) SetReplyTo(replyTo string)     { q.ReplyTo = replyTo }
func (q *QueryRequest) SetDatabase(database string)   { q.Database = database }
func (q *QueryRequest) SetUser(user string)           { q.User = user }
func (q *QueryRequest) SetPassword(password string)   { q.Password = password }
func (q *QueryRequest) SetAuthKey(authKey string)     { q.AuthKey = authKey }
func (q *QueryRequest) SetNoCache(value bool)         { q.NoCache = value }
func (q *QueryRequest) SetNoMeta(value bool)          { q.NoMeta = value }

func parseInt(raw string) int {
	if raw == "" {
		return 0
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0
	}
	return value
}

func parseBool(raw string) bool {
	if raw == "" {
		return false
	}
	b, err := strconv.ParseBool(raw)
	if err != nil {
		return false
	}
	return b
}
