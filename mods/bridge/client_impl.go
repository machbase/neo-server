package bridge

import (
	context "context"
	"fmt"

	bridgerpc "github.com/machbase/neo-grpc/bridge"
	spi "github.com/machbase/neo-spi"
)

type BridgeClient struct {
	underlying bridgerpc.RuntimeClient
}

func NewBridgeClient(ctx context.Context, underlying bridgerpc.RuntimeClient) (*BridgeClient, error) {
	return &BridgeClient{
		underlying: underlying,
	}, nil
}

type ConnectorResult struct {
	Elapse string
	cli    *BridgeClient
	result *bridgerpc.Result
}

func (cli *BridgeClient) Exec(ctx context.Context, name string, command string, params ...any) (*ConnectorResult, error) {
	req := &bridgerpc.ExecRequest{
		Name:    name,
		Command: command,
	}
	if pv, err := bridgerpc.ConvertToDatum(params); err != nil {
		return nil, err
	} else {
		req.Params = pv
	}
	rsp, err := cli.underlying.Exec(ctx, req)
	if err != nil {
		return nil, err
	}
	if !rsp.Success {
		return nil, fmt.Errorf("connector exec, %s", rsp.Reason)
	}
	ret := &ConnectorResult{
		Elapse: rsp.Elapse,
		result: rsp.Result,
		cli:    cli,
	}
	return ret, nil
}

func (rs *ConnectorResult) Columns(ctx context.Context) (spi.Columns, error) {
	ret := []*spi.Column{}
	for _, c := range rs.result.Fields {
		ret = append(ret, &spi.Column{
			Name:   c.Name,
			Type:   c.Type,
			Size:   int(c.Size),
			Length: int(c.Length),
		})
	}
	return ret, nil
}

func (rs *ConnectorResult) Fetch(ctx context.Context) ([]any, error) {
	rsp, err := rs.cli.underlying.ResultFetch(ctx, rs.result)
	if err != nil {
		return nil, err
	}
	if !rsp.Success {
		return nil, fmt.Errorf("bridge fetch, %s", rsp.Reason)
	}
	if rsp.HasNoRows {
		return nil, nil
	}
	return bridgerpc.ConvertFromDatum(rsp.Values...)
}

func (rs *ConnectorResult) Close(ctx context.Context) error {
	rsp, err := rs.cli.underlying.ResultClose(ctx, rs.result)
	if err != nil {
		return err
	}
	if !rsp.Success {
		return fmt.Errorf("bridge close, %s", rsp.Reason)
	}
	return nil
}
