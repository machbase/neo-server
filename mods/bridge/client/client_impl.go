package client

import (
	context "context"
	"fmt"

	"github.com/machbase/neo-server/api"
	bridgerpc "github.com/machbase/neo-server/api/bridge"
	"github.com/machbase/neo-server/mods/bridge"
)

type BridgeClient struct {
	underlying bridgerpc.RuntimeClient
}

func NewBridgeClient(underlying bridgerpc.RuntimeClient) (*BridgeClient, error) {
	return &BridgeClient{
		underlying: underlying,
	}, nil
}

type ConnectorResult struct {
	Elapse         string
	cli            *BridgeClient
	sqlQueryResult *bridgerpc.SqlQueryResult
}

func (cli *BridgeClient) Exec(ctx context.Context, name string, command string, params ...any) (*ConnectorResult, error) {
	return nil, fmt.Errorf("not implemented client.Exec()")
}

func (cli *BridgeClient) Query(ctx context.Context, name string, command string, params ...any) (*ConnectorResult, error) {
	req := &bridgerpc.ExecRequest{
		Name: name,
	}
	cmd := &bridgerpc.ExecRequest_SqlQuery{}
	cmd.SqlQuery.SqlText = command

	if pv, err := bridge.ConvertToDatum(params); err != nil {
		return nil, err
	} else {
		cmd.SqlQuery.Params = pv
	}
	req.Command = cmd

	rsp, err := cli.underlying.Exec(ctx, req)
	if err != nil {
		return nil, err
	}
	if !rsp.Success {
		return nil, fmt.Errorf("connector exec, %s", rsp.Reason)
	}
	ret := &ConnectorResult{
		Elapse:         rsp.Elapse,
		sqlQueryResult: rsp.GetSqlQueryResult(),
		cli:            cli,
	}
	return ret, nil
}

func (rs *ConnectorResult) Columns(ctx context.Context) (api.Columns, error) {
	ret := []*api.Column{}
	for _, c := range rs.sqlQueryResult.Fields {
		ret = append(ret, &api.Column{
			Name:     c.Name,
			DataType: api.DataType(c.Type),
		})
	}
	return ret, nil
}

func (rs *ConnectorResult) Fetch(ctx context.Context) ([]any, error) {
	rsp, err := rs.cli.underlying.SqlQueryResultFetch(ctx, rs.sqlQueryResult)
	if err != nil {
		return nil, err
	}
	if !rsp.Success {
		return nil, fmt.Errorf("bridge fetch, %s", rsp.Reason)
	}
	if rsp.HasNoRows {
		return nil, nil
	}
	return bridge.ConvertFromDatum(rsp.Values...)
}

func (rs *ConnectorResult) Close(ctx context.Context) error {
	rsp, err := rs.cli.underlying.SqlQueryResultClose(ctx, rs.sqlQueryResult)
	if err != nil {
		return err
	}
	if !rsp.Success {
		return fmt.Errorf("bridge close, %s", rsp.Reason)
	}
	return nil
}
