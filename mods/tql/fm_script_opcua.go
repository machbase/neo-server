package tql

import (
	"fmt"

	js "github.com/dop251/goja"
	"github.com/gopcua/opcua"
	"github.com/gopcua/opcua/ua"
)

func (ctx *JSContext) nativeModuleOpcua(r *js.Runtime, module *js.Object) {
	// m = require("opcua")
	o := module.Get("exports").(*js.Object)
	// m.Client({})
	o.Set("connect", ctx.opcua_connect)
	o.Set("MessageSecurityModeNone", ua.MessageSecurityModeNone)
	o.Set("MessageSecurityModeSign", ua.MessageSecurityModeSign)
	o.Set("MessageSecurityModeSignAndEncrypt", ua.MessageSecurityModeSignAndEncrypt)
}

func (ctx *JSContext) opcua_connect(call js.FunctionCall) js.Value {
	opts := struct {
		Endpoint            string                 `json:"endpoint"`
		MessageSecurityMode ua.MessageSecurityMode `json:"messageSecurityMode"`
	}{
		Endpoint:            "opc.tcp://localhost:4840",
		MessageSecurityMode: ua.MessageSecurityModeNone,
	}
	if len(call.Arguments) > 0 {
		if err := ctx.vm.ExportTo(call.Arguments[0], &opts); err != nil {
			return ctx.vm.NewGoError(fmt.Errorf("opcua.Client: %s", err.Error()))
		}
	}
	c, err := opcua.NewClient(opts.Endpoint, opcua.SecurityMode(opts.MessageSecurityMode))
	if err != nil {
		return ctx.vm.NewGoError(fmt.Errorf("opcua.NewClient: %s", err.Error()))
	}

	if err := c.Connect(ctx); err != nil {
		return ctx.vm.NewGoError(fmt.Errorf("opcua.Connect: %s", err.Error()))
	}
	defer c.Close(ctx)

	ret := ctx.vm.NewObject()
	return ret
}
