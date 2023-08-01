package maps

import (
	"fmt"
	"strings"
	"time"

	"github.com/machbase/neo-server/mods/codec/opts"
	"github.com/machbase/neo-server/mods/util"
)

func TimeLocation(timezone string) (opts.Option, error) {
	switch strings.ToUpper(timezone) {
	case "LOCAL":
		timezone = "Local"
	case "UTC":
		timezone = "UTC"
	}
	if timeLocation, err := time.LoadLocation(timezone); err != nil {
		timeLocation, err := util.GetTimeLocation(timezone)
		if err != nil {
			return nil, fmt.Errorf("f(tz) %s", err.Error())
		}
		return opts.TimeLocation(timeLocation), nil
	} else {
		return opts.TimeLocation(timeLocation), nil
	}
}
