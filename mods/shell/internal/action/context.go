package action

import (
	"context"
	"fmt"
	"time"

	"github.com/machbase/neo-server/v8/api"
	"golang.org/x/text/language"
)

type ActionContext struct {
	Actor        *Actor
	Line         string
	BorrowConn   func() (api.Conn, error)
	Ctx          context.Context
	CtxCancel    context.CancelFunc
	Lang         language.Tag
	TimeLocation *time.Location
	TimeFormat   string
	Interactive  bool // is shell in BATCH or INTERACTIVE mode
	ServeMode    bool // is shell is running in SERVER/PROXY or user shell mode
}

func (ctx *ActionContext) IsUserShellMode() bool {
	return !ctx.ServeMode
}

func (ctx *ActionContext) IsUserShellInteractiveMode() bool {
	return !ctx.ServeMode && ctx.Interactive
}

func (ctx *ActionContext) IsUserShellBatchMode() bool {
	return !ctx.ServeMode && !ctx.Interactive
}

func (ctx *ActionContext) IsServeMode() bool {
	return ctx.ServeMode
}

func (ctx *ActionContext) Pref() *Pref {
	return ctx.Actor.pref
}

func (ctx *ActionContext) PrefTimeformat() string {
	return ctx.Pref().Timeformat().Value()
}

func (ctx *ActionContext) PrefTimeLocation() *time.Location {
	return ctx.Pref().TimeZone().TimezoneValue()
}

// ShutdownServerFunc returns callable function to shutdown server if this instance has ability of shutdown server
// otherwise return nil
func (ctx *ActionContext) ShutdownServerFunc() ShutdownServerFunc {
	return ctx.Actor.ShutdownServer
}

func (ctx *ActionContext) Print(args ...any) {
	fmt.Print(args...)
}

func (ctx *ActionContext) Printf(format string, args ...any) {
	fmt.Printf(format, args...)
}

func (ctx *ActionContext) Println(args ...any) {
	fmt.Println(args...)
}

func (ctx *ActionContext) Printfln(format string, args ...any) {
	fmt.Printf(format+"\n", args...)
}
