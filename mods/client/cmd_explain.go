package client

import "github.com/chzyer/readline"

func (cli *client) pcExplain() *readline.PrefixCompleter {
	return readline.PcItem("explain")
}

func (cli *client) doExplain(sqlText string) {
	plan, err := cli.db.Explain(sqlText)
	if err != nil {
		cli.Writeln(err.Error())
		return
	}
	cli.Writeln(plan)
}
