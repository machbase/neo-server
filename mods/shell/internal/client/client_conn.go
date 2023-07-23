package client

import (
	context "context"
	"errors"
	"fmt"

	"github.com/machbase/neo-grpc/machrpc"
)

type machbaseClientProvider interface {
	MachbaseClient() machrpc.MachbaseClient
}

func (cli *client) ConnectorClient(ctx context.Context, name string) (Connector, error) {
	if provider, ok := cli.db.(machbaseClientProvider); !ok {
		return nil, errors.New("connector rpc is not supported")
	} else {
		if rpcClient := provider.MachbaseClient(); rpcClient != nil {
			con := &connector{
				Name:      name,
				rpcClient: rpcClient,
			}
			return con, nil
		} else {
			return nil, errors.New("connector rpc is not implemented")
		}
	}
}

type Connector interface {
	Exec(ctx context.Context, command string, params ...any) (*ConnectorResult, error)
}

var _ Connector = &connector{}

type ConnectorResult struct {
	Elapse string
	conn   *connector
	result *machrpc.ConnectorResult
}

type connector struct {
	Name      string
	rpcClient machrpc.MachbaseClient
}

func (conn *connector) Exec(ctx context.Context, command string, params ...any) (*ConnectorResult, error) {
	req := &machrpc.ConnectorExecRequest{
		Name:    conn.Name,
		Command: command,
	}
	if arr, err := machrpc.ConvertToDatum(params); err != nil {
		return nil, err
	} else {
		req.Params = arr
	}
	rsp, err := conn.rpcClient.ConnectorExec(ctx, req)
	if err != nil {
		return nil, err
	}
	if !rsp.Success {
		return nil, fmt.Errorf("connector exec, %s", rsp.Reason)
	}
	ret := &ConnectorResult{
		Elapse: rsp.Elapse,
		result: rsp.Result,
		conn:   conn,
	}
	return ret, nil
}

func (rs *ConnectorResult) Fetch(ctx context.Context) ([]any, error) {
	rsp, err := rs.conn.rpcClient.ConnectorResultFetch(ctx, rs.result)
	if err != nil {
		return nil, err
	}
	if !rsp.Success {
		return nil, fmt.Errorf("connector fetch, %s", rsp.Reason)
	}
	return machrpc.ConvertFromDatum(rsp.Values...)
}

func (rs *ConnectorResult) Close(ctx context.Context) error {
	rsp, err := rs.conn.rpcClient.ConnectorResultClose(ctx, rs.result)
	if err != nil {
		return err
	}
	if !rsp.Success {
		return fmt.Errorf("connector close, %s", rsp.Reason)
	}
	return nil
}
