package internal

type RowsEncoderBase struct {
	httpHeaders map[string][]string
}

func (reb *RowsEncoderBase) HttpHeaders() map[string][]string {
	return reb.httpHeaders
}

func (reb *RowsEncoderBase) SetHttpHeader(key string, value string) {
	if reb.httpHeaders == nil {
		reb.httpHeaders = make(map[string][]string)
	}
	reb.httpHeaders[key] = append(reb.httpHeaders[key], value)
}

func (reb *RowsEncoderBase) DelHttpHeader(key string) {
	if reb.httpHeaders != nil {
		return
	}
	delete(reb.httpHeaders, key)
}
