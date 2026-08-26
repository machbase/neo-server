package publisher

import (
	"context"
	"fmt"

	"github.com/dop251/goja"
	"github.com/machbase/neo-server/v8/mods/bridge"
	"github.com/machbase/neo-server/v8/mods/model"
)

func Module(ctx context.Context, rt *goja.Runtime, module *goja.Object) {
	// m = require("@jsh/publisher")
	o := module.Get("exports").(*goja.Object)
	// m.publisher({bridge: "name"})
	o.Set("publisher", func(optObj map[string]any) goja.Value {
		var cname string
		if len(optObj) > 0 {
			// parse db options `$.publisher({bridge: "name"})`
			if br, ok := optObj["bridge"]; ok {
				cname = br.(string)
			}
		}
		// TEMP: jsh has no exported way yet to resolve the actual logged-in
		// session user from this native module's ctx, so a fixed "sys"
		// scope is used until that wiring exists (see plan-model-bridge.md).
		br, err := bridge.GetBridge(ctx, model.UserScope{User: "sys"}, cname)
		if err != nil || br == nil {
			return rt.NewGoError(fmt.Errorf("publisher: bridge '%s' not found", cname))
		}

		ret := rt.NewObject()
		if mqttC, ok := br.(*bridge.MqttBridge); ok {
			ret.Set("publish", func(topic string, payload any) goja.Value {
				flag, err := mqttC.Publish(topic, payload)
				if err != nil {
					return rt.NewGoError(fmt.Errorf("publisher: %s", err.Error()))
				}
				return rt.ToValue(flag)
			})
		} else if natsC, ok := br.(*bridge.NatsBridge); ok {
			ret.Set("publish", func(subject string, payload any) goja.Value {
				flag, err := natsC.Publish(subject, payload)
				if err != nil {
					return rt.NewGoError(fmt.Errorf("publisher: %s", err.Error()))
				}
				return rt.ToValue(flag)
			})
		} else {
			return rt.NewGoError(fmt.Errorf("publisher: bridge '%s' not supported", cname))
		}
		return ret
	})
}
