package client

type CmdExplain struct {
	Sql string `arg:"" passthrough:""`
}

func (cli *client) doExplain(sqlText string) {
	plan, err := cli.db.Explain(sqlText)
	if err != nil {
		cli.Writeln(err.Error())
		return
	}
	cli.Writeln(plan)
}
