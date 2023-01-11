package bridge

type PluginContext interface{}
type PluginInstance interface{}

type ServeOptions struct {
	GrpcSettings

	QueryHandler QueryHandler
}

type QueryHandler interface{}

type GrpcSettings struct {
	// MaxReceiveMsgSize the max gRPC message size in bytes the plugin can receive.
	// If this is <= 0, gRPC uses the default 16MB.
	MaxReceiveMsgSize int
	// MaxSendMsgSize the max gRPC message size in bytes the plugin can send.
	// If this is <= 0, gRPC uses the default `math.MaxInt32`.
	MaxSendMsgSize int
}

func Serve(opts ServeOptions) error {
	return nil
}
