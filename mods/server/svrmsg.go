package server

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/machbase/neo-client/api"
)

type QueryRequest struct {
	SqlText      string `json:"q"`
	Params       []any  `json:"p,omitempty"`
	ReplyTo      string `json:"reply,omitempty"`       // for mqtt query only
	RowsFlatten  bool   `json:"rowsFlatten,omitempty"` // json output only for http, mqtt
	RowsArray    bool   `json:"rowsArray,omitempty"`   // json output only for http, mqtt
	Transpose    bool   `json:"transpose,omitempty"`   // json output only for http, mqtt
	Timeformat   string `json:"timeformat,omitempty"`  //
	TimeLocation string `json:"tz,omitempty"`          //
	Format       string `json:"format,omitempty"`      //
	Compress     string `json:"compress,omitempty"`    //
	Precision    int    `json:"precision,omitempty"`   //
	Rownum       bool   `json:"rownum,omitempty"`      //
	Heading      bool   `json:"heading,omitempty"`     // deprecated, use Header
	Header       string `json:"header,omitempty"`      //
}

func decodeQueryRequestJSON(r io.Reader, req *QueryRequest) error {
	dec := json.NewDecoder(r)
	dec.UseNumber()
	if err := dec.Decode(req); err != nil {
		return err
	}
	params, err := normalizeQueryParams(req.Params)
	if err != nil {
		return fmt.Errorf("invalid p, %w", err)
	}
	req.Params = params
	return nil
}

func parseQueryParams(raw string) ([]any, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	dec := json.NewDecoder(strings.NewReader(raw))
	dec.UseNumber()
	var params []any
	if err := dec.Decode(&params); err != nil {
		return nil, fmt.Errorf("invalid p, %w", err)
	}
	return normalizeQueryParams(params)
}

func normalizeQueryParams(params []any) ([]any, error) {
	if len(params) == 0 {
		return params, nil
	}
	ret := make([]any, len(params))
	for i, param := range params {
		value, err := normalizeQueryParamValue(param)
		if err != nil {
			return nil, err
		}
		ret[i] = value
	}
	return ret, nil
}

func normalizeQueryParamValue(value any) (any, error) {
	switch v := value.(type) {
	case nil, string, bool,
		int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64,
		float32, float64:
		return value, nil
	case json.Number:
		if !strings.ContainsAny(v.String(), ".eE") {
			if n, err := v.Int64(); err == nil {
				return n, nil
			}
		}
		n, err := v.Float64()
		if err != nil {
			return nil, err
		}
		return n, nil
	case []any, map[string]any:
		return nil, fmt.Errorf("bind parameter must be scalar, got %T", value)
	default:
		return nil, fmt.Errorf("unsupported bind parameter type %T", value)
	}
}

type QueryResponse struct {
	Success         bool       `json:"success"`
	Reason          string     `json:"reason"`
	Elapse          string     `json:"elapse"`
	Data            *QueryData `json:"data,omitempty"`
	ContentType     string     `json:"-"`
	ContentEncoding string     `json:"-"`
	Content         []byte     `json:"-"`
}

type QueryData struct {
	Columns []string       `json:"columns,omitempty"`
	Types   []api.DataType `json:"types,omitempty"`
	Rows    [][]any        `json:"rows"`
}

type WriteRequest struct {
	Table   string            `json:"table"`
	ReplyTo string            `json:"reply,omitempty"` // for mqtt query only
	Data    *WriteRequestData `json:"data"`
}

type WriteRequestData struct {
	Columns []string `json:"columns"`
	Rows    [][]any  `json:"rows"`
}

type WriteResponse struct {
	Success bool               `json:"success"`
	Reason  string             `json:"reason"`
	Elapse  string             `json:"elapse"`
	Data    *WriteResponseData `json:"data,omitempty"`
}

type WriteResponseData struct {
	AffectedRows uint64                   `json:"affectedRows,omitempty"`
	Files        map[string]*UserFileData `json:"files,omitempty"`
}

type UserFileData struct {
	Id          string `json:"ID,omitempty"` // file id
	Filename    string `json:"FN,omitempty"` // file name
	Size        int64  `json:"SZ,omitempty"` // file size
	ContentType string `json:"CT,omitempty"` // content type
	StoreDir    string `json:"SD,omitempty"` // stored dir
}
