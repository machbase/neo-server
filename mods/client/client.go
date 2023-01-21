package client

import (
	"fmt"
	"io"
	"time"

	"github.com/machbase/neo-grpc/machrpc"
)

type Client interface {
	Close()
	RunInteractive()
	RunSql(sqlText string)
}

type Config struct {
	ServerAddr   string
	Stdin        io.ReadCloser
	Stdout       io.Writer
	Stderr       io.Writer
	VimMode      bool
	QueryTimeout time.Duration
}

type client struct {
	conf *Config
	db   *machrpc.Client
}

func New(conf *Config) (Client, error) {
	machcli := machrpc.NewClient(machrpc.QueryTimeout(conf.QueryTimeout))
	err := machcli.Connect(conf.ServerAddr)
	if err != nil {
		return nil, err
	}
	cli := &client{
		conf: conf,
		db:   machcli,
	}
	return cli, nil
}

func (cli *client) Close() {
	if cli.db != nil {
		cli.db.Disconnect()
	}
}

func (cli *client) Config() *Config {
	return cli.conf
}

func (cli *client) Println(args ...any) {
	fmt.Fprintln(cli.conf.Stdout, args...)
}

func (cli *client) Printf(format string, args ...any) {
	fmt.Fprintf(cli.conf.Stdout, format, args...)
}

func (cli *client) Writeln(args ...any) {
	fmt.Fprintln(cli.conf.Stdout, args...)
}

func (cli *client) Writef(format string, args ...any) {
	fmt.Fprintf(cli.conf.Stdout, format+"\r\n", args...)
}

func (cli *client) RunSql(sqlText string) {
	cli.doSql(sqlText)
}

func (cli *client) RunInteractive() {
	cli.doPrompt()
}
