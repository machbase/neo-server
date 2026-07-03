package server

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/machbase/neo-client/api"
	"github.com/machbase/neo-server/v8/mods/codec"
	"github.com/machbase/neo-server/v8/mods/codec/opts"
	"github.com/machbase/neo-server/v8/mods/util"
)

type QueryRequest struct {
	SqlText            string `json:"q"`
	Params             []any  `json:"p,omitempty"`
	ReplyTo            string `json:"reply,omitempty"`              // for mqtt query only
	RowsFlatten        bool   `json:"rowsFlatten,omitempty"`        // json output only for http, mqtt
	RowsArray          bool   `json:"rowsArray,omitempty"`          // json output only for http, mqtt
	Transpose          bool   `json:"transpose,omitempty"`          // json output only for http, mqtt
	Timeformat         string `json:"timeformat,omitempty"`         //
	TimeLocation       string `json:"tz,omitempty"`                 //
	Format             string `json:"format,omitempty"`             //
	BinaryFormat       string `json:"binaryformat,omitempty"`       //
	Compress           string `json:"compress,omitempty"`           //
	Precision          int    `json:"precision,omitempty"`          //
	Rownum             bool   `json:"rownum,omitempty"`             //
	Heading            bool   `json:"heading,omitempty"`            // deprecated, use Header
	Header             string `json:"header,omitempty"`             //
	Delimiter          string `json:"delimiter,omitempty"`          // csv output only for http, mqtt
	BoxStyle           string `json:"boxStyle,omitempty"`           // box output only for http, mqtt
	BoxSeparateColumns bool   `json:"boxSeparateColumns,omitempty"` // box output only for http, mqtt
	BoxDrawBorder      bool   `json:"boxDrawBorder,omitempty"`      // box output only for http, mqtt
}

func NewQueryRequest() *QueryRequest {
	return &QueryRequest{
		SqlText:            "",
		Params:             nil,
		ReplyTo:            "",
		RowsFlatten:        false,
		RowsArray:          false,
		Transpose:          false,
		Timeformat:         "ns",
		TimeLocation:       "UTC",
		Format:             "json",
		BinaryFormat:       "hex",
		Compress:           "",
		Precision:          -1,
		Rownum:             false,
		Heading:            true,
		Header:             "",
		Delimiter:          ",",
		BoxStyle:           "default",
		BoxSeparateColumns: true,
		BoxDrawBorder:      true,
	}
}

func (req *QueryRequest) DecodeJSON(r io.Reader) error {
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
	if req.Header == "skip" {
		req.Heading = false
	}
	return nil
}

func (req *QueryRequest) DecodeQuery(ctx *gin.Context) error {
	req.SqlText = ctx.Query("q")
	if p, err := parseQueryParams(ctx.Query("p")); err != nil {
		return err
	} else {
		req.Params = p
	}
	req.Timeformat = strString(ctx.Query("timeformat"), req.Timeformat)
	req.TimeLocation = strString(ctx.Query("tz"), req.TimeLocation)
	req.Format = strString(ctx.Query("format"), req.Format)
	req.BinaryFormat = strString(ctx.Query("binaryformat"), req.BinaryFormat)
	req.Compress = ctx.Query("compress")
	req.Rownum = strBool(ctx.Query("rownum"), req.Rownum)
	req.Heading = strBool(ctx.Query("heading"), req.Heading)
	if h := ctx.Query("header"); h == "skip" {
		req.Heading = false
	}
	req.Precision = strInt(ctx.Query("precision"), req.Precision)
	req.Transpose = strBool(ctx.Query("transpose"), req.Transpose)
	req.RowsFlatten = strBool(ctx.Query("rowsFlatten"), req.RowsFlatten)
	req.RowsArray = strBool(ctx.Query("rowsArray"), req.RowsArray)
	return nil
}

func (req *QueryRequest) DecodePostForm(ctx *gin.Context) error {
	req.SqlText = ctx.PostForm("q")
	if p, err := parseQueryParams(ctx.PostForm("p")); err != nil {
		return err
	} else {
		req.Params = p
	}
	req.Timeformat = strString(ctx.PostForm("timeformat"), req.Timeformat)
	req.TimeLocation = strString(ctx.PostForm("tz"), req.TimeLocation)
	req.Format = strString(ctx.PostForm("format"), req.Format)
	req.BinaryFormat = strString(ctx.PostForm("binaryformat"), req.BinaryFormat)
	req.Compress = ctx.PostForm("compress")
	req.Rownum = strBool(ctx.PostForm("rownum"), req.Rownum)
	req.Heading = strBool(ctx.PostForm("heading"), req.Heading)
	if h := ctx.PostForm("header"); h == "skip" {
		req.Heading = false
	}
	req.Precision = strInt(ctx.PostForm("precision"), req.Precision)
	req.Transpose = strBool(ctx.PostForm("transpose"), req.Transpose)
	req.RowsFlatten = strBool(ctx.PostForm("rowsFlatten"), req.RowsFlatten)
	req.RowsArray = strBool(ctx.PostForm("rowsArray"), req.RowsArray)
	return nil
}

type QueryHook struct {
	SetStatusCode      func(int)
	SetContentType     func(string)
	SetContentEncoding func(string)
	SetUserMessage     func(string)
}

func (req *QueryRequest) Execute(ctx context.Context, w io.Writer, hook *QueryHook) error {
	if hook == nil {
		hook = &QueryHook{}
	}

	if len(req.SqlText) == 0 {
		if hook.SetStatusCode != nil {
			hook.SetStatusCode(400)
		}
		return fmt.Errorf("sql text is empty")
	}

	timeLocation, err := util.ParseTimeLocation(req.TimeLocation, time.UTC)
	if err != nil {
		if hook.SetStatusCode != nil {
			hook.SetStatusCode(400)
		}
		return err
	}

	var output io.Writer
	switch req.Compress {
	case "gzip":
		output = gzip.NewWriter(w)
	default:
		req.Compress = ""
		output = w
	}

	encoder := codec.NewEncoder(req.Format,
		opts.OutputStream(output),
		opts.Timeformat(req.Timeformat),
		opts.Binaryformat(req.BinaryFormat),
		opts.Precision(req.Precision),
		opts.Rownum(req.Rownum),
		opts.Header(req.Heading),
		opts.TimeLocation(timeLocation),
		opts.Delimiter(req.Delimiter),
		opts.BoxStyle(req.BoxStyle),
		opts.BoxSeparateColumns(req.BoxSeparateColumns),
		opts.BoxDrawBorder(req.BoxDrawBorder),
		opts.RowsFlatten(req.RowsFlatten),
		opts.RowsArray(req.RowsArray),
		opts.Transpose(req.Transpose),
	)
	conn, err := getPoolConn(ctx)
	if err != nil {
		if hook.SetStatusCode != nil {
			hook.SetStatusCode(http.StatusServiceUnavailable)
		}
		return err
	}
	defer conn.Close()

	rows, err := conn.Query(ctx, req.SqlText, req.Params...)
	if err != nil {
		if hook.SetStatusCode != nil {
			hook.SetStatusCode(http.StatusInternalServerError)
		}
		return err
	}
	defer rows.Close()

	if hook.SetContentType != nil {
		hook.SetContentType(encoder.ContentType())
	}
	if hook.SetContentEncoding != nil && len(req.Compress) > 0 {
		hook.SetContentEncoding(req.Compress)
	}

	if !rows.IsFetchable() {
		if hook.SetStatusCode != nil {
			hook.SetStatusCode(http.StatusOK)
		}
		if hook.SetUserMessage != nil {
			hook.SetUserMessage(rows.Message())
		}
		return nil
	}

	var columns api.Columns
	if cols, err := rows.Columns(); err != nil {
		if hook.SetStatusCode != nil {
			hook.SetStatusCode(http.StatusInternalServerError)
		}
		return err
	} else {
		columns = cols
		codec.SetEncoderColumns(encoder, cols)
	}

	encoder.Open()
	defer encoder.Close()

	for rows.Next() {
		values, err := columns.MakeBuffer()
		if err != nil {
			if hook.SetStatusCode != nil {
				hook.SetStatusCode(http.StatusInternalServerError)
			}
			return err
		}
		if err := rows.Scan(values...); err != nil {
			if hook.SetStatusCode != nil {
				hook.SetStatusCode(http.StatusInternalServerError)
			}
			return err
		}
		if err := encoder.AddRow(values); err != nil {
			if hook.SetStatusCode != nil {
				hook.SetStatusCode(http.StatusInternalServerError)
			}
			return err
		}
	}
	if hook.SetStatusCode != nil {
		hook.SetStatusCode(http.StatusOK)
	}
	if hook.SetUserMessage != nil {
		hook.SetUserMessage(rows.Message())
	}
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
