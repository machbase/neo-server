package transcoder

import (
	"errors"
	"fmt"
	"strings"

	"github.com/gofrs/uuid"
	"github.com/machbase/neo-server/mods/logging"
	"github.com/machbase/neo-server/mods/script"
)

type Transcoder interface {
	Process(any) (any, error)
}

func New(name string, opts ...Option) Transcoder {
	var tc Transcoder
	switch {
	case name == "cems":
		tc = cemsTranslatorSingleton
	case strings.HasPrefix(name, "@"):
		tc = newScripTransaltor(strings.TrimPrefix(name, "@"))
	default:
		tc = noTranslatorSingleton
	}
	for _, o := range opts {
		o(tc)
	}
	return tc
}

type Option func(tc Transcoder)

func OptionPath(paths ...string) Option {
	return func(tc Transcoder) {
		if sc, ok := tc.(*scriptTranslator); ok {
			sc.opts = append(sc.opts, script.OptionPath(paths...))
		}
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
	sc   script.Script
	err  error
	log  logging.Log
	opts []script.Option
}

func newScripTransaltor(name string) *scriptTranslator {
	st := &scriptTranslator{
		name: name,
		log:  logging.GetLog("transcoder-" + name),
	}
	return st
}

func (ts *scriptTranslator) Process(r any) (any, error) {
	if ts.sc == nil {
		ld := script.NewLoader(ts.opts...)
		ts.sc, ts.err = ld.Load(ts.name)
	}

	if ts.sc == nil {
		if ts.err != nil {
			return nil, ts.err
		} else {
			return nil, errors.New("script not found")
		}
	}

	ts.sc.SetVar("INPUT", r)
	ts.sc.SetFunc("LOG", func(args ...any) (any, error) {
		if len(args) == 0 {
			return "", nil
		}
		ts.log.Info(args[0])
		return "", nil
	})

	if err := ts.sc.Run(); err != nil {
		return nil, err
	}

	var result any
	if err := ts.sc.GetVar("OUTPUT", &result); err != nil {
		return nil, err
	}
	return result, nil
}
