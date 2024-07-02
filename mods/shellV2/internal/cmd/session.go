package cmd

import (
	"strings"
	"time"

	"github.com/machbase/neo-client/machrpc"
	"github.com/machbase/neo-server/mods/shellV2/internal/action"
	"github.com/machbase/neo-server/mods/util"
)

func init() {
	action.RegisterCmd(&action.Cmd{
		Name:   "session",
		PcFunc: pcSession,
		Action: doSession,
		Desc:   "Database session management",
		Usage:  strings.ReplaceAll(helpSession, "\t", "    "),
	})
}

const helpSession = `    session command [options]
  commands:
    list              list sessions
    kill <id>         force to close a session
    stat              show session stat
  options:
    -a,--all          includes detail description`

type SessionCmd struct {
	List struct {
		ShowAll bool `name:"all" short:"a"`
	} `cmd:"" name:"list"`
	Kill struct {
		Id    string `arg:"" name:"id"`
		Force bool   `name:"force" short:"f"`
	} `cmd:"" name:"kill"`
	Stat struct {
	} `cmd:"" name:"stat"`
	Help bool `kong:"-"`
}

func pcSession() action.PrefixCompleterInterface {
	return action.PcItem("session")
}

func doSession(ctx *action.ActionContext) {
	cmd := &SessionCmd{}
	parser, err := action.Kong(cmd, func() error { ctx.Println(helpSession); cmd.Help = true; return nil })
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}
	parseCtx, err := parser.Parse(util.SplitFields(ctx.Line, false))
	if cmd.Help {
		return
	}
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}
	switch parseCtx.Command() {
	case "list":
		doSessionList(ctx, cmd.List.ShowAll)
	case "kill <id>":
		doSessionKill(ctx, cmd.Kill.Id, cmd.Kill.Force)
	case "stat":
		doSessionStat(ctx)
	}
}

func doSessionList(ctx *action.ActionContext, showAll bool) {
	_, sessions, err := ctx.Actor.Database().ServerSessions(false, true)
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}
	sess := map[string]*machrpc.Session{}
	for _, s := range sessions {
		sess[s.Id] = s
	}
	rows, err := ctx.Conn.Query(ctx.Ctx, `SELECT ID, USER_ID, USER_NAME, STMT_COUNT FROM V$NEO_SESSION`)
	if err != nil {
		ctx.Println("ERR", err.Error())
	}
	if showAll {
		ctx.Println("[ V$NEO_SESSION ]")
	}
	now := time.Now().UnixNano()
	box := ctx.NewBox([]string{"ID", "USER_NAME", "USER_ID", "STMT_COUNT", "CREATED", "LAST", "LAST SQL"})
	for rows.Next() {
		var id string
		var userId string
		var userName string
		var stmtCount int
		if err := rows.Scan(&id, &userId, &userName, &stmtCount); err != nil {
			ctx.Println("ERR", err.Error())
			continue
		}
		s := sess[id]
		if s != nil {
			var created, used string
			creDur := time.Duration(now - s.CreTime)
			useDur := time.Duration(now - s.LatestSqlTime)
			if creDur < time.Second {
				created = creDur.String()
			} else {
				created = util.HumanizeDurationWithFormat(creDur, util.HumanizeDurationFormatShortPadding)
			}
			if useDur < time.Second {
				used = useDur.String()
			} else {
				used = util.HumanizeDurationWithFormat(useDur, util.HumanizeDurationFormatShortPadding)
			}
			box.AppendRow(id, userName, userId, stmtCount, created, used, s.LatestSql)
		} else {
			box.AppendRow(id, userName, userId, stmtCount)
		}
	}
	box.Render()
	rows.Close()

	if showAll {
		ctx.Println("[ V$SESSION ]")
		doShowByQuery0(ctx, "SELECT ID, USER_ID, LOGIN_TIME, MAX_QPX_MEM FROM V$SESSION", false)
	}
}

func doSessionStat(ctx *action.ActionContext) {
	statz, _, err := ctx.Actor.Database().ServerSessions(true, false)
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}
	if statz != nil {
		box := ctx.NewBox([]string{"NAME", "VALUE"})
		box.AppendRow("CONNS", util.NumberFormat(statz.ConnsInUse))
		box.AppendRow("CONNS_USED", util.NumberFormat(statz.Conns))
		box.AppendRow("STMTS", util.NumberFormat(statz.StmtsInUse))
		box.AppendRow("STMTS_USED", util.NumberFormat(statz.Stmts))
		box.AppendRow("APPENDERS", util.NumberFormat(statz.AppendersInUse))
		box.AppendRow("APPENDERS_USED", util.NumberFormat(statz.Appenders))
		box.AppendRow("RAW_CONNS", util.NumberFormat(statz.RawConns))
		box.Render()
	}
}
func doSessionKill(ctx *action.ActionContext, id string, force bool) {
	success, err := ctx.Actor.Database().ServerKillSession(id, force)
	if err != nil {
		ctx.Println("ERR", err.Error())
	}
	if success {
		ctx.Println("session " + id + " cancelled")
	} else {
		ctx.Println("session " + id + ", failed cancel")
	}
}
