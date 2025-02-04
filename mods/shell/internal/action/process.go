package action

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/machbase/neo-server/v8/api"
	"github.com/machbase/neo-server/v8/mods/util"
)

func (cli *Actor) Process(line string) {
	fields := util.SplitFields(line, true)
	if len(fields) == 0 {
		return
	}
	if runtime.GOOS == "windows" {
		// on windows, command line keeps the trailing ';'
		fields[len(fields)-1] = strings.TrimSuffix(fields[len(fields)-1], ";")
	}

	cmdName := strings.ToLower(fields[0])
	var cmd *Cmd
	var ok bool
	if cmd, ok = globalCommands[cmdName]; ok {
		line = strings.TrimSpace(line[len(cmdName):])
	} else if IsSqlCommand(cmdName) {
		cmd, ok = globalCommands["sql"]
	}

	if !ok || cmd == nil {
		fmt.Printf("Command %q not found.\n", cmdName)
		return
	}

	if !cmd.ClientAction {
		if err := cli.checkDatabase(); err != nil {
			fmt.Println("ERR", err.Error())
			return
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var conn api.Conn
	closeOnce := sync.Once{}
	closeConn := func() {
		closeOnce.Do(func() {
			if conn != nil {
				conn.Close()
			}
		})
	}
	defer closeConn()

	actCtx := &ActionContext{
		Line:  line,
		Actor: cli,
		BorrowConn: func() (api.Conn, error) {
			if conn == nil {
				if c, err := cli.db.Connect(ctx, api.WithPassword(cli.conf.User, cli.conf.Password)); err != nil {
					fmt.Println("ERR", err.Error())
				} else {
					conn = c
				}
			}
			return conn, nil
		},
		Ctx:          ctx,
		CtxCancel:    cancel,
		Lang:         cli.conf.Lang,
		TimeLocation: time.UTC,
		TimeFormat:   "ns",
		Interactive:  cli.interactive,
	}

	done := make(chan bool, 1)
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	interrupted := false
	go func() {
		for {
			select {
			case <-c:
				interrupted = true
				goto exit
			case <-done:
				goto exit
			}
		}
	exit:
		closeConn()
		actCtx.CtxCancel()
		close(c)
	}()

	cmd.Action(actCtx)

	signal.Reset(os.Interrupt)
	done <- true
	close(done)

	if interrupted {
		fmt.Printf("    command %q is interrupted.\n", cmd.Name)
	}

	if cmd.Deprecated {
		fmt.Printf("\n    command %q is deprecated, %s\n", cmd.Name, cmd.DeprecatedMessage)
	}
}
