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
		var explicitUser string
		if len(optObj) > 0 {
			// parse db options `$.publisher({bridge: "name"})`
			if br, ok := optObj["bridge"]; ok {
				cname = br.(string)
			}
			// optional `user` overrides the ambient scope, for entry points
			// with no wired context (CLI jsh, cgi-bin); see @jsh/db's ClientOptions.User.
			if u, ok := optObj["user"]; ok {
				explicitUser, _ = u.(string)
			}
		}
		// Resolve the user scope for this bridge lookup, in order of precedence:
		//  1. explicit `user` option (script author's own responsibility).
		//  2. ctx-derived scope (TQL SCRIPT({}), CLI machbase-neo shell; see
		//     model.ContextWithUserScope/ContextWithUserScopeFunc).
		//  3. "sys" fallback.
		scope := model.UserScope{User: "sys"}
		if explicitUser != "" {
			scope = model.UserScope{User: explicitUser}
		} else if ctxScope, ok := model.UserScopeFromContext(ctx); ok && ctxScope.User != "" {
			scope = ctxScope
		}
		br, err := bridge.GetBridge(ctx, scope, cname)
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
