package cmd

import (
	"context"
	"strings"
	"time"

	"github.com/machbase/neo-server/v8/api"
	"github.com/machbase/neo-server/v8/api/mgmt"
	"github.com/machbase/neo-server/v8/mods/shell/internal/action"
	"github.com/machbase/neo-server/v8/mods/util"
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
    list                list sessions
    kill <id>           force to close a session
    stat [--reset]      show session stat
    limit               get limit
    set-limit [--conn=<num>] [--query=<num>] set limit
  options:
    -a,--all            includes detail description`

type SessionCmd struct {
	List struct {
		ShowAll bool `name:"all" short:"a"`
	} `cmd:"" name:"list"`
	Kill struct {
		Id    string `arg:"" name:"id"`
		Force bool   `name:"force" short:"f"`
	} `cmd:"" name:"kill"`
	Stat struct {
		Reset bool `name:"reset"`
	} `cmd:"" name:"stat"`
	Limit struct {
	} `cmd:"" name:"limit"`
	SetLimit struct {
		Conn  int `name:"conn" default:"-2147483648"`
		Query int `name:"query" default:"-2147483648"`
	} `cmd:"" name:"set-limit"`
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
		doSessionStat(ctx, cmd.Stat.Reset)
	case "limit":
		doSessionLimit(ctx, -2147483648, -2147483648)
	case "set-limit":
		doSessionLimit(ctx, cmd.SetLimit.Conn, cmd.SetLimit.Query)
	}
}

func doSessionLimit(ctx *action.ActionContext, newConnLimit int, newQueryLimit int) {
	mgmtClient, err := ctx.Actor.ManagementClient()
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}
	req := &mgmt.LimitSessionRequest{}
	if newConnLimit < -1 && newQueryLimit < -1 {
		req.Cmd = "get"
	} else {
		req.Cmd = "set"
		req.MaxOpenConn = int32(newConnLimit)
		req.MaxOpenQuery = int32(newQueryLimit)
	}

	rsp, err := mgmtClient.LimitSession(ctx.Ctx, req)
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}

	numberOrUnlimited := func(n int32) string {
		if n == -1 {
			return "unlimited"
		}
		return util.NumberFormat(int(n))
	}
	box := ctx.NewBox([]string{"NAME", "VALUE"})
	box.AppendRow("CONN LIMIT", numberOrUnlimited(rsp.MaxOpenConn))
	box.AppendRow("CONN REMAINS", numberOrUnlimited(rsp.RemainedOpenConn))
	box.AppendRow("QUERY LIMIT", numberOrUnlimited(rsp.MaxOpenQuery))
	box.AppendRow("QUERY REMAINS", numberOrUnlimited(rsp.RemainedOpenQuery))
	box.Render()
}

func doSessionList(ctx *action.ActionContext, showAll bool) {
	mgmtClient, err := ctx.Actor.ManagementClient()
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}
	_, sessions, err := serverSessions(mgmtClient, ctx.Ctx, false, true, false)
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}
	sess := map[string]*mgmt.Session{}
	for _, s := range sessions {
		sess[s.Id] = s
	}
	conn, err := ctx.BorrowConn()
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}
	rows, err := conn.Query(ctx.Ctx, `SELECT ID, USER_ID, USER_NAME, STMT_COUNT FROM V$NEO_SESSION`)
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

func doSessionStat(ctx *action.ActionContext, reset bool) {
	mgmtClient, err := ctx.Actor.ManagementClient()
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}
	statz, _, err := serverSessions(mgmtClient, ctx.Ctx, true, false, reset)
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}
	if statz != nil {
		connUse := statz.Conns
		connWaitTimePerUse := time.Duration(0)
		connUseTimePerUse := time.Duration(0)
		if connUse > 0 {
			connWaitTimePerUse = time.Duration(statz.ConnWaitTime / uint64(connUse))
			connUseTimePerUse = time.Duration(statz.ConnUseTime / uint64(connUse))
		}
		box := ctx.NewBox([]string{"NAME", "VALUE"})
		box.AppendRow("CONNS", util.NumberFormat(statz.ConnsInUse))
		box.AppendRow("CONNS_USED", util.NumberFormat(statz.Conns))
		box.AppendRow("CONNS_WAIT_AVG", connWaitTimePerUse.String())
		box.AppendRow("CONNS_USE_AVG", connUseTimePerUse.String())
		box.AppendRow("STMTS", util.NumberFormat(statz.StmtsInUse))
		box.AppendRow("STMTS_USED", util.NumberFormat(statz.Stmts))
		box.AppendRow("APPENDERS", util.NumberFormat(statz.AppendersInUse))
		box.AppendRow("APPENDERS_USED", util.NumberFormat(statz.Appenders))
		box.AppendRow("RAW_CONNS", util.NumberFormat(statz.RawConns))
		box.AppendRow("QUERY_EXEC_HWM", time.Duration(statz.QueryExecHwm).String())
		box.AppendRow("QUERY_EXEC_AVG", time.Duration(statz.QueryExecAvg).String())
		box.AppendRow("QUERY_WAIT_HWM", time.Duration(statz.QueryWaitHwm).String())
		box.AppendRow("QUERY_WAIT_AVG", time.Duration(statz.QueryWaitAvg).String())
		box.AppendRow("QUERY_FETCH_HWM", time.Duration(statz.QueryFetchHwm).String())
		box.AppendRow("QUERY_FETCH_AVG", time.Duration(statz.QueryFetchAvg).String())
		box.AppendRow("QUERY_HWM", time.Duration(statz.QueryHwm).String())
		box.AppendRow("QUERY_HWM_EXEC", time.Duration(statz.QueryHwmExec).String())
		box.AppendRow("QUERY_HWM_WAIT", time.Duration(statz.QueryHwmWait).String())
		box.AppendRow("QUERY_HWM_FETCH", time.Duration(statz.QueryHwmFetch).String())
		box.AppendRow("QUERY_HWM_SQL", api.SqlTidyWidth(80, statz.QueryHwmSql))
		box.AppendRow("QUERY_HWM_SQL_ARG", statz.QueryHwmSqlArg)
		box.Render()
	}
}
func doSessionKill(ctx *action.ActionContext, id string, force bool) {
	mgmtClient, err := ctx.Actor.ManagementClient()
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}
	success, err := serverKillSession(mgmtClient, ctx.Ctx, id, force)
	if err != nil {
		ctx.Println("ERR", err.Error())
	}
	if success {
		ctx.Println("session " + id + " cancelled")
	} else {
		ctx.Println("session " + id + ", failed cancel")
	}
}

func serverSessions(client mgmt.ManagementClient, ctx context.Context, reqStatz, reqSessions, reset bool) (*mgmt.Statz, []*mgmt.Session, error) {
	req := &mgmt.SessionsRequest{Statz: reqStatz, Sessions: reqSessions, ResetStatz: reset}
	rsp, err := client.Sessions(ctx, req)
	if err != nil {
		return nil, nil, err
	}
	return rsp.Statz, rsp.Sessions, nil
}

func serverKillSession(client mgmt.ManagementClient, ctx context.Context, sessionId string, force bool) (bool, error) {
	req := &mgmt.KillSessionRequest{Id: sessionId, Force: force}
	rsp, err := client.KillSession(ctx, req)
	if err != nil {
		return false, err
	}
	return rsp.Success, nil
}
