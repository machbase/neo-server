package server

import "github.com/machbase/neo-server/v8/api"

type QueryRequest struct {
	SqlText      string `json:"q"`
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
