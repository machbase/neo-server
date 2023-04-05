package client

import (
	"reflect"
	"strings"
	"time"

	"github.com/alecthomas/kong"
)

type TimezoneParser struct {
}

// implements kong.TypeMapper
func (tp *TimezoneParser) Decode(ctx *kong.DecodeContext, target reflect.Value) error {
	token, err := ctx.Scan.PopValue("tz")
	if err != nil {
		return err
	}
	tz := token.String()
	if strings.ToLower(tz) == "local" {
		target.Set(reflect.ValueOf(time.Local))
		return nil
	} else if tz == "UTC" {
		target.Set(reflect.ValueOf(time.UTC))
		return nil
	}
	if tz, err := time.LoadLocation(tz); err != nil {
		return err
	} else {
		target.Set(reflect.ValueOf(tz))
	}
	return nil
}

func KongOptions(helpFunc func() error) []kong.Option {
	return []kong.Option{
		kong.HelpOptions{Compact: true},
		kong.Exit(func(int) {}),
		kong.TypeMapper(reflect.TypeOf((*time.Location)(nil)), &TimezoneParser{}),
		kong.Help(func(options kong.HelpOptions, ctx *kong.Context) error { return helpFunc() }),
	}
}

func Kong(grammar any, helpFunc func() error) (*kong.Kong, error) {
	return kong.New(grammar, KongOptions(helpFunc)...)
}
