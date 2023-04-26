package client

import (
	"io"
	"time"

	"github.com/machbase/neo-grpc/machrpc"
	"github.com/machbase/neo-grpc/mgmt"
	spi "github.com/machbase/neo-spi"
	"golang.org/x/net/context"
	"golang.org/x/text/language"
)

type ActionContext struct {
	Line         string
	Client       Client
	DB           spi.Database
	Lang         language.Tag
	TimeLocation *time.Location
	TimeFormat   string
	Interactive  bool // is shell in BATCH or INTERACTIVE mode
	ServeMode    bool // is shell is running in SERVER/PROXY or user shell mode

	Stdin  io.ReadCloser
	Stdout io.Writer
	Stderr io.Writer

	parent     context.Context
	cancelFunc func()
	cli        *client
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

func (ctx *ActionContext) Deadline() (deadline time.Time, ok bool) {
	return ctx.parent.Deadline()
}

func (ctx *ActionContext) Done() <-chan struct{} {
	return ctx.parent.Done()
}

func (ctx *ActionContext) Err() error {
	return ctx.parent.Err()
}

func (ctx *ActionContext) Value(key any) any {
	return ctx.parent.Value(key)
}

func (ctx *ActionContext) Cancel() {
	ctx.cancelFunc()
}

func (ctx *ActionContext) Write(p []byte) (int, error) {
	return ctx.Client.Write(p)
}
func (ctx *ActionContext) Print(args ...any) {
	ctx.Client.Print(args...)
}
func (ctx *ActionContext) Printf(format string, args ...any) {
	ctx.Client.Printf(format, args...)
}
func (ctx *ActionContext) Println(args ...any) {
	ctx.Client.Println(args...)
}
func (ctx *ActionContext) Printfln(format string, args ...any) {
	ctx.Client.Printfln(format, args...)
}

func (ctx *ActionContext) Config() *Config {
	return ctx.cli.conf
}

func (ctx *ActionContext) Pref() *Pref {
	return ctx.cli.pref
}

func (ctx *ActionContext) NewManagementClient() (mgmt.ManagementClient, error) {
	conn, err := machrpc.MakeGrpcConn(ctx.cli.conf.ServerAddr)
	if err != nil {
		return nil, err
	}
	return mgmt.NewManagementClient(conn), nil
}

// ShutdownServerFunc returns callable function to shutdown server if this instance has ability of shutdown server
// otherwise return nil
func (ctx *ActionContext) ShutdownServerFunc() ShutdownServerFunc {
	return ctx.cli.ShutdownServer
}
