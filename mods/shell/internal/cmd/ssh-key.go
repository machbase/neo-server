package cmd

import (
	"fmt"
	"strings"

	"github.com/chzyer/readline"
	"github.com/machbase/neo-grpc/mgmt"
	"github.com/machbase/neo-server/mods/shell/internal/client"
	"github.com/machbase/neo-server/mods/util"
)

func init() {
	client.RegisterCmd(&client.Cmd{
		Name:   "ssh-key",
		PcFunc: pcSshKey,
		Action: doSshKey,
		Desc:   "Manage ssh keys",
		Usage:  helpSshKey,
	})
}

const helpSshKey = `  ssh-key command [options] [args...]
  commands:
    list                       list registered ssh keys
    add <type> <key> <comment> add new ssh key
    del <key>                  delete ssh key
`

type SshKeyCmd struct {
	List struct{} `cmd:"" name:"list"`
	Del  struct {
		Fingerprint string `arg:"" name:"fingerprint"`
	} `cmd:"" name:"del"`
	Add struct {
		KeyType string   `arg:"" name:"type"`
		Key     string   `arg:"" name:"key"`
		Comment []string `arg:"" passthrough:"" name:"comment"`
	} `cmd:"" name:"add"`
	Help bool `kong:"-"`
}

func pcSshKey() readline.PrefixCompleterInterface {
	return readline.PcItem("ssh-key")
}

func doSshKey(ctx *client.ActionContext) {
	cmd := &SshKeyCmd{}
	parser, err := client.Kong(cmd, func() error { ctx.Println(helpSshKey); cmd.Help = true; return nil })
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
		doSshKeyList(ctx)
	case "add <type> <key> <comment>":
		doSshKeyAdd(ctx, cmd.Add.KeyType, cmd.Add.Key, strings.Join(cmd.Add.Comment, " "))
	case "del <fingerprint>":
		doSshKeyDel(ctx, cmd.Del.Fingerprint)
	default:
		ctx.Println("ERR", fmt.Sprintf("unhandled command %s", parseCtx.Command()))
		return
	}
}

func doSshKeyList(ctx *client.ActionContext) {
	mgmtCli, err := ctx.NewManagementClient()
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}
	rsp, err := mgmtCli.ListSshKey(ctx, &mgmt.ListSshKeyRequest{})
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}

	if !rsp.Success {
		ctx.Println("ERR", rsp.Reason)
		return
	}

	box := ctx.NewBox([]string{"ROWNUM", "NAME", "KEY TYPE", "FINGERPRINT"})
	for i, k := range rsp.SshKeys {
		box.AppendRow(i+1, k.Comment, k.KeyType, k.Fingerprint)
	}
	box.Render()
}

func doSshKeyDel(ctx *client.ActionContext, fingerprint string) {
	mgmtCli, err := ctx.NewManagementClient()
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}
	rsp, err := mgmtCli.DelSshKey(ctx, &mgmt.DelSshKeyRequest{
		Fingerprint: fingerprint,
	})
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}
	if !rsp.Success {
		ctx.Println("ERR", rsp.Reason)
		return
	}
	ctx.Println("deleted")
}

func doSshKeyAdd(ctx *client.ActionContext, keyType, key, comment string) {
	mgmtCli, err := ctx.NewManagementClient()
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}
	rsp, err := mgmtCli.AddSshKey(ctx, &mgmt.AddSshKeyRequest{
		KeyType: keyType, Key: key, Comment: comment,
	})
	if err != nil {
		ctx.Println("ERR", err.Error())
		return
	}
	if !rsp.Success {
		ctx.Println("ERR", rsp.Reason)
		return
	}
	ctx.Println("Add sshkey", rsp.Reason)
}
