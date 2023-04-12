package transcoder

import (
	"fmt"
	"strings"

	"github.com/gofrs/uuid"
)

type Transcoder interface {
	Process(any) (any, error)
}

func New(name string) Transcoder {
	switch {
	case name == "cems":
		return cemsTranslatorSingleton
	case strings.HasPrefix(name, "@"):
		return &scriptTranslator{
			name: strings.TrimPrefix(name, "@"),
		}
	default:
		return noTranslatorSingleton
	}
}

type noTranslator struct {
}

var noTranslatorSingleton = &noTranslator{}

func (ts *noTranslator) Process(r any) (any, error) {
	return r, nil
}

type cemsTranslator struct {
	idgen *uuid.Gen
}

var cemsTranslatorSingleton = &cemsTranslator{
	idgen: uuid.NewGen(),
}

func (ts *cemsTranslator) Process(r any) (any, error) {
	orgValues, ok := r.([]any)
	if !ok {
		return nil, fmt.Errorf("unuspported input data '%T'", r)
	}
	newValues := make([]any, 10)

	id, _ := ts.idgen.NewV6()
	idstr := id.String()
	payload := fmt.Sprintf(`{"@type":"type.googleapis.com/google.protobuf.DoubleValue", "value":%f}`, orgValues[2])

	newValues[0] = orgValues[0] // name
	newValues[1] = orgValues[1] // time
	newValues[2] = orgValues[2] // value
	newValues[3] = "float64"    // type
	newValues[4] = nil          // ivalue
	newValues[5] = nil          // svalue
	newValues[6] = idstr        // id
	newValues[7] = "mqtt"       // pname
	newValues[8] = 0            // sampling_period
	newValues[9] = payload      // payload
	return newValues, nil
}

type scriptTranslator struct {
	name string
}

func (ts *scriptTranslator) Process(r any) (any, error) {
	return r, nil
}
