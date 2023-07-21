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
	Exec(ctx context.Context, command string) (*ConnectorResult, error)
}

var _ Connector = &connector{}

type ConnectorResult struct {
	Elapse string
}

type connector struct {
	Name      string
	rpcClient machrpc.MachbaseClient
}

func (conn *connector) Exec(ctx context.Context, command string) (*ConnectorResult, error) {
	req := &machrpc.ConnectorExecRequest{}
	rsp, err := conn.rpcClient.ConnectorExec(ctx, req)
	if err != nil {
		return nil, err
	}
	if !rsp.Success {
		return nil, fmt.Errorf("connector exec, %s", rsp.Reason)
	}
	ret := &ConnectorResult{
		Elapse: rsp.Elapse,
	}
	return ret, nil

}

func (rs *ConnectorResult) Fetch(ctx context.Context) ([]any, error) {
	return nil, fmt.Errorf("not implemented ConnectorResultFetch")
}

func (rs *ConnectorResult) Close() error {
	return fmt.Errorf("not implemented ConnectorResultClose")
}
