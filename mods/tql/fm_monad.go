package tql

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	gocsv "encoding/csv"

	"github.com/machbase/neo-server/mods/util/glob"
	spi "github.com/machbase/neo-spi"
)

type lazyOption struct {
	flag bool
}

func (node *Node) fmLazy(flag bool) *lazyOption {
	return &lazyOption{flag: flag}
}

func (node *Node) fmTake(args ...int) (*Record, error) {
	limit := 0
	if n, ok := node.GetValue("limit"); !ok {
		if len(args) == 1 {
			limit = args[0]
		} else if len(args) == 2 {
			limit = args[1]
		}
		node.SetValue("limit", limit)
	} else {
		limit = n.(int)
	}
	if limit < 0 {
		return nil, ErrArgs("TAKE", 1, "limit should be larger than 0")
	}
	offset := 0
	if n, ok := node.GetValue("offset"); !ok {
		if len(args) == 2 {
			offset = args[0]
		}
		node.SetValue("offset", offset)
	} else {
		offset = n.(int)
	}
	count := 0
	if n, ok := node.GetValue("count"); ok {
		count = n.(int)
	}
	count++
	node.SetValue("count", count)

	if count > offset+limit {
		return BreakRecord, nil
	}
	if count <= offset {
		return nil, nil
	}
	return node.Inflight(), nil
}

func (node *Node) fmDrop(args ...int) (*Record, error) {
	limit := 0
	if n, ok := node.GetValue("limit"); !ok {
		if len(args) == 1 {
			limit = args[0]
		} else if len(args) == 2 {
			limit = args[1]
		}
		node.SetValue("limit", limit)
	} else {
		limit = n.(int)
	}
	if limit < 0 {
		return nil, ErrArgs("DROP", 1, "limit should be larger than 0")
	}
	offset := 0
	if n, ok := node.GetValue("offset"); !ok {
		if len(args) == 2 {
			offset = args[0]
		}
		node.SetValue("offset", offset)
	} else {
		offset = n.(int)
	}
	count := 0
	if n, ok := node.GetValue("count"); ok {
		count = n.(int)
	}
	count++
	node.SetValue("count", count)

	if count > offset && count <= offset+limit {
		return nil, nil
	}
	return node.Inflight(), nil
}

func (node *Node) fmFilter(flag bool) *Record {
	if !flag {
		return nil // drop this vector
	}
	return node.Inflight()
}

func (node *Node) fmFlatten() any {
	rec := node.Inflight()
	if rec.IsArray() {
		ret := []*Record{}
		for _, r := range rec.Array() {
			k := r.Key()
			switch value := r.Value().(type) {
			case []any:
				for _, v := range value {
					if v == nil {
						continue
					}
					ret = append(ret, NewRecord(k, v))
				}
			case any:
				ret = append(ret, r)
			default:
				ret = append(ret, ErrorRecord(fmt.Errorf("fmtFlatten() unknown type '%T' in array record", value)))
			}
		}
		return ret
	} else if rec.IsTuple() {
		switch value := rec.Value().(type) {
		case [][]any:
			k := rec.Key()
			ret := []*Record{}
			for _, v := range value {
				if len(v) == 0 {
					continue
				}
				ret = append(ret, NewRecord(k, v))
			}
			return ret
		case []any:
			k := rec.Key()
			ret := []*Record{}
			for _, v := range value {
				if v == nil {
					continue
				}
				ret = append(ret, NewRecord(k, v))
			}
			return ret
		case any:
			return rec
		default:
			return ErrorRecord(fmt.Errorf("fmtFlatten() unknown type '%T' in array record", value))
		}
	} else {
		return rec
	}
}

func (node *Node) fmGroupByKey(args ...any) any {
	key := node.Inflight().key
	value := node.Inflight().value
	lazy := false
	if len(args) > 0 {
		for _, arg := range args {
			switch v := arg.(type) {
			case *lazyOption:
				lazy = v.flag
			}
		}
	}
	if lazy {
		node.Buffer(key, value)
		return nil
	}

	var curKey any
	curKey, _ = node.GetValue("curKey")
	defer func() {
		node.SetValue("curKey", curKey)
	}()
	if curKey == nil {
		curKey = key
	}
	node.Buffer(key, value)

	if curKey != key {
		node.YieldBuffer(curKey)
		curKey = key
	}
	return nil
}

// Drop Key, then make the first element of value to promote as a key,
// decrease dimension of vector as result if the input is not multiple dimension vector.
// `map=POPKEY(V, 0)` produces
// 1 dimension : `K: [V1, V2, V3...]` ==> `V1 : [V2, V3, .... ]`
// 2 dimension : `K: [[V11, V12, V13...],[V21, V22, V23...], ...] ==> `V11: [V12, V13...]` and `V21: [V22, V23...]` ...
func (node *Node) fmPopKey(args ...int) (any, error) {
	var nth = 0
	if len(args) > 0 {
		nth = args[0]
	}

	// V : value
	inflight := node.Inflight()
	if inflight == nil || inflight.value == nil {
		return nil, nil
	}
	switch val := inflight.value.(type) {
	default:
		return nil, fmt.Errorf("f(POPKEY) V should be []any or [][]any, but %T", val)
	case []any:
		if nth < 0 || nth >= len(val) {
			return nil, fmt.Errorf("f(POPKEY) 1st arg should be between 0 and %d, but %d", len(val)-1, nth)
		}
		if _, ok := node.GetValue("isFirst"); !ok {
			node.SetValue("isFirst", true)
			columns := node.task.ResultColumns()
			cols := columns
			if len(columns) > nth+1 {
				cols = []*spi.Column{columns[nth+1]}
				cols = append(cols, columns[1:nth+1]...)
			}
			if len(cols) > nth+2 {
				cols = append(cols, columns[nth+2:]...)
			}
			node.task.SetResultColumns(cols)
		}
		newKey := val[nth]
		newVal := append(val[0:nth], val[nth+1:]...)
		ret := NewRecord(newKey, newVal)
		return ret, nil
	case [][]any:
		ret := make([]*Record, len(val))
		if _, ok := node.GetValue("isFirst"); !ok {
			node.SetValue("isFirst", true)
			columns := node.task.ResultColumns()
			if len(columns) > 1 {
				node.task.SetResultColumns(columns[1:])
			}
		}
		for i, v := range val {
			if len(v) < 2 {
				return nil, fmt.Errorf("f(POPKEY) arg elements should be larger than 2, but %d", len(v))
			}
			if len(v) == 2 {
				ret[i] = NewRecord(v[0], v[1])
			} else {
				ret[i] = NewRecord(v[0], v[1:])
			}
		}
		return ret, nil
	}
}

// Merge all incoming values into a single key,
// incresing dimension of vector as result.
// `map=PUSHKEY(NewKEY)` produces `NewKEY: [K, V...]`
func (node *Node) fmPushKey(newKey any) (any, error) {
	if _, ok := node.GetValue("isFirst"); !ok {
		node.SetValue("isFirst", true)
		node.task.SetResultColumns(append([]*spi.Column{node.AsColumnTypeOf(newKey)}, node.task.ResultColumns()...))
	}
	rec := node.Inflight()
	if rec == nil {
		return nil, nil
	}
	key, value := rec.key, rec.value
	var newVal []any
	switch val := value.(type) {
	case []any:
		newVal = append([]any{key}, val...)
	case any:
		newVal = []any{key, val}
	default:
		return nil, ErrArgs("PUSHKEY", 0, fmt.Sprintf("Value should be array, but %T", value))
	}
	return NewRecord(newKey, newVal), nil
}

func (node *Node) fmMapKey(newKey any) (any, error) {
	if _, ok := node.GetValue("isFirst"); !ok {
		node.SetValue("isFirst", true)
		node.task.SetResultColumns(append([]*spi.Column{node.AsColumnTypeOf(newKey)}, node.task.ResultColumns()[1:]...))
	}
	rec := node.Inflight()
	if rec == nil {
		return nil, nil
	}
	return NewRecord(newKey, rec.value), nil
}

func (node *Node) fmPushValue(idx int, newValue any, opts ...any) (any, error) {
	var columnName = "column"
	if len(opts) > 0 {
		if str, ok := opts[0].(string); ok {
			columnName = str
		}
	}

	inflight := node.Inflight()
	if inflight == nil {
		return nil, nil
	}

	if idx < 0 {
		idx = 0
	}
	switch val := inflight.value.(type) {
	case []any:
		if idx > len(val) {
			idx = len(val)
		}
	default:
		if idx > 0 {
			idx = 1
		}
	}

	if _, ok := node.GetValue("isFirst"); !ok {
		node.SetValue("isFirst", true)
		cols := node.task.ResultColumns() // cols contains "ROWNUM"
		if len(cols) == idx {
			newCol := node.AsColumnTypeOf(newValue)
			newCol.Name = columnName
			head := cols[0 : idx+1]
			tail := cols[idx+1:]
			updateCols := []*spi.Column{}
			updateCols = append(updateCols, head...)
			updateCols = append(updateCols, newCol)
			updateCols = append(updateCols, tail...)
			node.task.SetResultColumns(updateCols)
		}
	}

	switch val := inflight.value.(type) {
	case []any:
		head := val[0:idx]
		tail := val[idx:]
		updateVal := []any{}
		updateVal = append(updateVal, head...)
		updateVal = append(updateVal, newValue)
		updateVal = append(updateVal, tail...)
		return NewRecord(inflight.key, updateVal), nil
	default:
		if idx <= 0 {
			return NewRecord(inflight.key, []any{newValue, val}), nil
		} else {
			return NewRecord(inflight.key, []any{val, newValue}), nil
		}
	}
}

func (node *Node) fmPopValue(idxes ...int) (any, error) {
	inflight := node.Inflight()
	if inflight == nil || len(idxes) == 0 {
		return inflight, nil
	}

	includes := []int{}
	switch val := inflight.value.(type) {
	case []any:
		count := len(val)
		for _, idx := range idxes {
			if idx < 0 || idx >= count {
				return nil, ErrArgs("PUSHKEY", 0, fmt.Sprintf("Index is out of range, value[%d]", idx))
			}
		}
		offset := 0
		for i := 0; i < count; i++ {
			if offset < len(idxes) && i == idxes[offset] {
				offset++
			} else {
				includes = append(includes, i)
			}
		}
	default:
		return nil, ErrArgs("POPHKEY", 0, fmt.Sprintf("Value should be array, but %T", val))
	}

	if _, ok := node.GetValue("isFirst"); !ok {
		node.SetValue("isFirst", true)
		cols := node.task.ResultColumns() // cols contains "ROWNUM"
		updateCols := []*spi.Column{cols[0]}
		for _, idx := range includes {
			updateCols = append(updateCols, cols[idx+1])
		}
		node.task.SetResultColumns(updateCols)
	}

	val := inflight.value.([]any)
	updateVal := []any{}
	for _, idx := range includes {
		updateVal = append(updateVal, val[idx])
	}
	return NewRecord(inflight.key, updateVal), nil
}

func (node *Node) fmMapValue(idx int, newValue any, opts ...any) (any, error) {
	inflight := node.Inflight()
	if inflight == nil {
		return nil, nil
	}
	switch val := inflight.value.(type) {
	case []any:
		if idx < 0 || idx >= len(val) {
			return node.fmPushValue(idx, newValue, opts...)
		}
		if _, ok := node.GetValue("isFirst"); !ok {
			node.SetValue("isFirst", true)
			if len(opts) > 0 {
				if newName, ok := opts[0].(string); ok {
					cols := node.task.ResultColumns() // cols contains "ROWNUM"
					cols[idx+1].Name = newName
				}
			}
		}
		val[idx] = newValue
		ret := NewRecord(inflight.key, val)
		return ret, nil
	default:
		if idx != 0 {
			return node.fmPushValue(idx, newValue, opts...)
		}

		if _, ok := node.GetValue("isFirst"); !ok {
			node.SetValue("isFirst", true)
			if len(opts) > 0 {
				if newName, ok := opts[0].(string); ok {
					cols := node.task.ResultColumns() // cols contains "ROWNUM"
					cols[idx+1].Name = newName
				}
			}
		}
		ret := NewRecord(inflight.key, newValue)
		return ret, nil
	}
}

func (node *Node) fmRegexp(pattern string, text string) (bool, error) {
	var expr *regexp.Regexp
	if v, exists := node.GetValue("$regexp.pattern"); exists {
		if v.(string) == pattern {
			if v, exists := node.GetValue("$regexp"); exists {
				expr = v.(*regexp.Regexp)
			}
		}
	}
	if expr == nil {
		compiled, err := regexp.Compile(pattern)
		if err != nil {
			return false, err
		}
		expr = compiled
		node.SetValue("$regexp", expr)
		node.SetValue("$regexp.pattern", pattern)
	}
	return expr.MatchString(text), nil
}

func (node *Node) fmGlob(pattern string, text string) (bool, error) {
	return glob.Match(pattern, text)
}

type LogDoer []any

func (ld LogDoer) Do(node *Node) error {
	node.task.LogInfo(ld...)
	return nil
}

func (node *Node) fmDoLog(args ...any) LogDoer {
	return LogDoer(args)
}

type HttpDoer struct {
	method  string
	url     string
	args    []string
	content any

	client *http.Client
}

func (doer *HttpDoer) Do(node *Node) error {
	var body io.Reader
	if doer.content != nil {
		buff := &bytes.Buffer{}
		csvEnc := gocsv.NewWriter(buff)
		switch v := doer.content.(type) {
		case []float64:
			arr := make([]string, len(v))
			for i, a := range v {
				arr[i] = fmt.Sprintf("%v", a)
			}
			csvEnc.Write(arr)
		case float64:
			csvEnc.Write([]string{fmt.Sprintf("%v", v)})
		case []string:
			csvEnc.Write(v)
		case string:
			csvEnc.Write([]string{v})
		case []any:
			arr := make([]string, len(v))
			for i, a := range v {
				arr[i] = fmt.Sprintf("%v", a)
			}
			csvEnc.Write(arr)
		case any:
			csvEnc.Write([]string{fmt.Sprintf("%v", v)})
		default:
			return fmt.Errorf("unhandled content value type %T", v)
		}
		csvEnc.Flush()
		body = buff
	}
	req, err := http.NewRequestWithContext(node.task.ctx, doer.method, doer.url, body)
	if err != nil {
		return err
	}

	for _, str := range doer.args {
		k, v, ok := strings.Cut(str, ":")
		if ok {
			k, v = strings.TrimSpace(k), strings.TrimSpace(v)
			req.Header.Add(k, v)
		}
	}

	if req.Header.Get("Content-Type") == "" {
		req.Header.Add("Content-Type", "text/csv")
	}
	if req.Header.Get("User-Agent") == "" {
		req.Header.Add("User-Agent", "machbase-neo tql http doer")
	}
	if doer.client == nil {
		doer.client = &http.Client{}
	}
	resp, err := doer.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		node.task.LogWarn("http-doer", doer.method, doer.url, resp.Status)
	} else if resp.StatusCode >= 300 {
		node.task.LogInfo("http-doer", doer.method, doer.url, resp.Status)
	} else {
		node.task.LogDebug("http-doer", doer.method, doer.url, resp.Status)
	}
	return nil
}

func (node *Node) fmDoHttp(method string, url string, body any, args ...string) *HttpDoer {
	var ret *HttpDoer
	if v, ok := node.GetValue("$httpDoer"); !ok {
		ret = &HttpDoer{}
		node.SetValue("$httpDoer", ret)
	} else {
		ret = v.(*HttpDoer)
	}
	ret.method = method
	ret.url = url
	ret.args = args
	ret.content = body
	return ret
}

type WhenDoer interface {
	Do(*Node) error
}

var (
	_ WhenDoer = LogDoer{}
	_ WhenDoer = &HttpDoer{}
)

func (node *Node) fmWhen(cond bool, action any) any {
	if !cond {
		return node.Inflight()
	}
	doer, ok := action.(WhenDoer)
	if !ok {
		node.task.LogErrorf("f(WHEN) 2nd arg is not a Doer type, but %T", action)
	} else {
		if err := doer.Do(node); err != nil {
			node.task.LogErrorf("f(WHEN) doer occurs, %s", err.Error())
		}
	}
	return node.Inflight()
}
