package tagql

import (
	"bufio"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strings"

	"github.com/machbase/neo-server/mods/codec"
	"github.com/machbase/neo-server/mods/do"
	"github.com/machbase/neo-server/mods/expression"
	"github.com/machbase/neo-server/mods/stream"
	"github.com/machbase/neo-server/mods/stream/spec"
	spi "github.com/machbase/neo-spi"
	"github.com/pkg/errors"
)

type TagQL interface {
	ToSQL() string
	Execute(ctx context.Context, db spi.Database, output spec.OutputStream) error
	ExecuteEncoder(ctxCtx context.Context, db spi.Database, encoder codec.RowsEncoder) error
	ExecuteHandler(ctx context.Context, db spi.Database, w http.ResponseWriter) error
}

type tagQL struct {
	table string
	tag   string

	baseTimeColumn string

	srcInput *SrcInput
	mapExprs []string
	sinkExpr string
}

var regexpTagQL = regexp.MustCompile(`([a-zA-Z0-9_-]+)\/(.+)`)
var regexpSpaceprefix = regexp.MustCompile(`^\s+(.*)`)

func Parse(table, tag string, in io.Reader) (TagQL, error) {
	reader := bufio.NewReader(in)

	parts := []byte{}
	stmt := []string{}
	expressions := []Line{}
	lineNo := 0
	for {
		b, isPrefix, err := reader.ReadLine()
		if err != nil {
			if err == io.EOF {
				if len(stmt) > 0 {
					line := Line{
						text: strings.Join(stmt, ""),
						line: lineNo,
					}
					expressions = append(expressions, line)
				}
				break
			}
			return nil, err
		}
		parts = append(parts, b...)
		if isPrefix {
			continue
		}
		lineNo++

		lineText := string(parts)
		parts = parts[:0]

		if lineText == "" || strings.HasPrefix(lineText, "#") || strings.HasPrefix(lineText, "--") {
			continue
		}
		if len(stmt) == 0 {
			stmt = append(stmt, lineText)
			continue
		}

		if regexpSpaceprefix.MatchString(lineText) {
			stmt = append(stmt, lineText)
			continue
		} else {
			line := Line{
				text: strings.Join(stmt, ""),
				line: lineNo,
			}
			expressions = append(expressions, line)
			stmt = stmt[:0]
			stmt = append(stmt, lineText)
		}

	}

	return ParseExpressions(table, tag, expressions)
}

type Line struct {
	text string
	line int
}

func ParseExpressions(table, tag string, exprs []Line) (TagQL, error) {
	if len(exprs) == 0 {
		return nil, errors.New("empty expressions")
	}

	tq := &tagQL{}
	tq.baseTimeColumn = "time"
	tq.table = table
	tq.tag = tag

	// src
	if len(exprs) >= 1 {
		srcLine := exprs[0]
		expr, err := expression.NewWithFunctions(srcLine.text, srcFunctions)
		if err != nil {
			return nil, errors.Wrapf(err, "at line %d", srcLine.line)
		}
		ret, err := expr.Eval(nil)
		if err != nil {
			return nil, err
		}
		input, ok := ret.(*SrcInput)
		if !ok {
			return nil, fmt.Errorf("invalid compile result of src at line %d, %v", srcLine.line, input)
		}
		if len(input.Columns) == 0 {
			input.Columns = []string{"value"}
		}
		tq.srcInput = input
	}

	// sink
	if len(exprs) >= 2 {
		sinkLine := exprs[len(exprs)-1]
		// validates the syntax
		_, err := expression.NewWithFunctions(sinkLine.text, sinkFunctions)
		if err != nil {
			return nil, errors.Wrapf(err, "at line %d", sinkLine.line)
		}
		tq.sinkExpr = sinkLine.text
	}

	// map
	if len(exprs) >= 3 {
		exprs = exprs[1 : len(exprs)-1]
		for _, mapLine := range exprs {
			// validates the syntax
			_, err := expression.NewWithFunctions(mapLine.text, mapFunctions)
			if err != nil {
				return nil, errors.Wrapf(err, "at line %d", mapLine.line)
			}
			tq.mapExprs = append(tq.mapExprs, mapLine.text)
		}
	}
	return tq, nil
}

func ParseURI(query string) (TagQL, error) {
	return ParseURIContext(context.Background(), query)
}

func ParseURIContext(ctx context.Context, query string) (TagQL, error) {
	subs := regexpTagQL.FindAllStringSubmatch(query, -1)
	if len(subs) != 1 || len(subs[0]) < 3 {
		return nil, errors.New("invalid syntax")
	}

	tq := &tagQL{}
	tq.baseTimeColumn = "time"

	tq.table = strings.ToUpper(strings.TrimSpace(subs[0][1]))

	uri, err := url.Parse("tag:///" + query)
	if err != nil {
		return nil, err
	}

	tq.table = strings.ToUpper(strings.TrimPrefix(path.Dir(uri.Path), "/"))
	tq.tag = path.Base(uri.Path)
	queryPart := uri.RawQuery

	var params map[string][]string
	if queryPart != "" {
		urlParams, err := url.ParseQuery(queryPart)
		if err != nil {
			return nil, err
		}
		params = urlParams
	}

	srcParts := params["src"]
	if len(srcParts) == 0 {
		tq.srcInput = newSrcInput()
		tq.srcInput.Columns = []string{"value"}
	} else {
		// only take the last one
		part := srcParts[len(srcParts)-1]
		// validates the syntax
		expr, err := expression.NewWithFunctions(part, srcFunctions)
		if err != nil {
			return nil, err
		}
		ret, err := expr.Eval(nil)
		if err != nil {
			return nil, err
		}
		input, ok := ret.(*SrcInput)
		if !ok {
			return nil, fmt.Errorf("invalid compile result of src, %v", input)
		}

		tq.srcInput = input
	}
	mapParts := params["map"]
	for _, part := range mapParts {
		// validates the syntax
		_ /*expr */, err := expression.NewWithFunctions(part, mapFunctions)
		if err != nil {
			return nil, err
		}
		tq.mapExprs = append(tq.mapExprs, part)
	}

	sinkParts := params["sink"]
	if len(sinkParts) > 0 {
		// only take the last one
		part := sinkParts[len(sinkParts)-1]
		// validates the syntax
		_ /*expr*/, err := expression.NewWithFunctions(part, sinkFunctions)
		if err != nil {
			return nil, err
		}
		tq.sinkExpr = part
	}

	return tq, nil
}

func (tq *tagQL) ExecuteHandler(ctx context.Context, db spi.Database, w http.ResponseWriter) error {
	var compress = ""
	var output spec.OutputStream

	switch compress {
	case "gzip":
		output = &stream.WriterOutputStream{Writer: gzip.NewWriter(w)}
	default:
		compress = ""
		output = &stream.WriterOutputStream{Writer: w}
	}

	encoder, err := tq.buildEncoder(output)
	if err != nil {
		return err
	}

	w.Header().Set("Content-Type", encoder.ContentType())
	if len(compress) > 0 {
		w.Header().Set("Content-Encoding", compress)
	}
	return tq.ExecuteEncoder(ctx, db, encoder)
}

func (tq *tagQL) Execute(ctxCtx context.Context, db spi.Database, output spec.OutputStream) (err error) {
	if output == nil {
		output, err = stream.NewOutputStream("-")
		if err != nil {
			return err
		}
	}
	encoder, err := tq.buildEncoder(output)
	if err != nil {
		return err
	}
	return tq.ExecuteEncoder(ctxCtx, db, encoder)
}

func (tq *tagQL) ExecuteEncoder(ctxCtx context.Context, db spi.Database, encoder codec.RowsEncoder) (err error) {
	deferHooks := []func(){}
	defer func() {
		for _, f := range deferHooks {
			f()
		}
	}()

	chain, err := NewExecutionChain(ctxCtx, tq.mapExprs)
	if err != nil {
		return err
	}

	var cols spi.Columns
	queryCtx := &do.QueryContext{
		DB: db,
		OnFetchStart: func(c spi.Columns) {
			cols = c
		},
		OnFetch: func(nrow int64, values []any) bool {
			if chain.Error() != nil {
				return false
			}
			chain.Source(values)
			return true
		},
		OnFetchEnd: func() {
			chain.Source(nil)
		},
		OnExecuted: nil, // never happen in tagQL
	}

	go chain.Start()
	go func() {
		open := false
		for arr := range chain.Sink() {
			if !open {
				if cols == nil {
					for i, v := range arr {
						cols = append(cols, &spi.Column{
							Name: fmt.Sprintf("col#%d", i),
							Type: fmt.Sprintf("%T", v)})
					}
				}
				// TODO can not trust column types if arr comes through map()
				encoder.Open(cols)
				deferHooks = append(deferHooks, func() {
					// if close encoder right away without defer,
					// it will crash, because it could be earlier than all map() pipe to be closed
					encoder.Close()
				})
				open = true
			}
			encoder.AddRow(arr)
		}
	}()
	_, err = do.Query(queryCtx, tq.ToSQL())

	chain.Wait()
	chain.Stop()

	if chain.Error() != nil {
		return chain.Error()
	}
	return err
}

func (tq *tagQL) buildEncoder(output spec.OutputStream) (codec.RowsEncoder, error) {
	sinkExpr, err := expression.NewWithFunctions(normalizeSinkFuncExpr(tq.sinkExpr), sinkFunctions)
	if err != nil {
		return nil, err
	}
	sinkRet, err := sinkExpr.Evaluate(map[string]any{"outstream": output})
	if err != nil {
		return nil, err
	}
	encoder, ok := sinkRet.(codec.RowsEncoder)
	if !ok {
		return nil, fmt.Errorf("invalid sink type: %T", sinkRet)
	}
	return encoder, nil
}

func (tq *tagQL) ToSQL() string {
	if tq.srcInput.Range == nil || tq.srcInput.Range.groupBy == 0 {
		return tq.toSql()
	} else {
		return tq.toSqlGroup()
	}
}

func (tq *tagQL) toSqlGroup() string {
	ret := ""
	rng := tq.srcInput.Range
	columns := strings.Join(tq.srcInput.Columns, ", ")
	if rng.ts == "last" {
		ret = fmt.Sprintf(`SELECT from_timestamp(round(to_timestamp(%s)/%d)*%d) %s, %s FROM %s
			WHERE
				name = '%s'
			AND %s
				BETWEEN
				    (SELECT MAX_TIME - %d FROM V$%s_STAT WHERE name = '%s') 
				AND (SELECT MAX_TIME FROM V$%s_STAT WHERE name = '%s')
			GROUP BY %s
			ORDER BY %s
			LIMIT %d
			`,
			tq.baseTimeColumn, rng.groupBy, rng.groupBy, tq.baseTimeColumn, columns, tq.table,
			tq.tag,
			tq.baseTimeColumn,
			rng.duration, tq.table, tq.tag,
			tq.table, tq.tag,
			tq.baseTimeColumn,
			tq.baseTimeColumn,
			tq.srcInput.Limit.limit,
		)
	} else if rng.ts == "now" {
		ret = fmt.Sprintf(`SELECT from_timestamp(round(to_timestamp(%s)/%d)*%d) %s, %s FROM %s
			WHERE
				name = '%s'
			AND %s BETWEEN now - %d AND now 
			GROUP BY %s
			ORDER BY %s
			LIMIT %d
			`,
			tq.baseTimeColumn, rng.groupBy, rng.groupBy, tq.baseTimeColumn, columns, tq.table,
			tq.tag,
			tq.baseTimeColumn, rng.duration,
			tq.baseTimeColumn,
			tq.baseTimeColumn,
			tq.srcInput.Limit.limit,
		)
	} else {
		ret = fmt.Sprintf(`SELECT from_timestamp(round(to_timestamp(%s)/%d)*%d) %s, %s FROM %s
			WHERE
				name = '%s'
			AND %s
				BETWEEN %d - %d AND %d
			GROUP BY %s
			ORDER BY %s
			LIMIT %d
			`,
			tq.baseTimeColumn, rng.groupBy, rng.groupBy, tq.baseTimeColumn, columns, tq.table,
			tq.tag,
			tq.baseTimeColumn,
			rng.tsTime.UnixNano(), rng.duration, rng.tsTime.UnixNano(),
			tq.baseTimeColumn,
			tq.baseTimeColumn,
			tq.srcInput.Limit.limit,
		)
	}
	return ret
}

func (tq *tagQL) toSql() string {
	ret := ""
	src := tq.srcInput
	columns := strings.Join(tq.srcInput.Columns, ", ")
	if src.Range.ts == "last" {
		ret = fmt.Sprintf(`SELECT %s, %s FROM %s
			WHERE
				name = '%s'
			AND %s
				BETWEEN 
					(SELECT MAX_TIME - %d FROM V$%s_STAT WHERE name = '%s') 
				AND (SELECT MAX_TIME FROM V$%s_STAT WHERE name = '%s')
			LIMIT %d
			`,
			tq.baseTimeColumn, columns, tq.table,
			tq.tag,
			tq.baseTimeColumn,
			src.Range.duration, tq.table, tq.tag,
			tq.table, tq.tag,
			src.Limit.limit,
		)
	} else if src.Range.ts == "now" {
		ret = fmt.Sprintf(`SELECT %s, %s FROM %s
			WHERE
				name = '%s'
			AND %s BETWEEN now - %d AND now 
			LIMIT %d
			`,
			tq.baseTimeColumn, columns, tq.table,
			tq.tag,
			tq.baseTimeColumn, src.Range.duration,
			src.Limit.limit,
		)
	} else {
		ret = fmt.Sprintf(`SELECT %s, %s FROM %s 
			WHERE
				name = '%s'
			AND %s
				BETWEEN %d - %d AND %d
			LIMIT %d`,
			tq.baseTimeColumn, columns, tq.table,
			tq.tag,
			tq.baseTimeColumn,
			src.Range.tsTime.UnixNano(), src.Range.duration, src.Range.tsTime.UnixNano(),
			src.Limit.limit)
	}
	return ret
}
