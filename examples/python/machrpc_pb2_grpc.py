# Generated by the gRPC Python protocol compiler plugin. DO NOT EDIT!
"""Client and server classes corresponding to protobuf-defined services."""
import grpc

import machrpc_pb2 as machrpc__pb2


class MachbaseStub(object):
    """Missing associated documentation comment in .proto file."""

    def __init__(self, channel):
        """Constructor.

        Args:
            channel: A grpc.Channel.
        """
        self.Conn = channel.unary_unary(
                '/machrpc.Machbase/Conn',
                request_serializer=machrpc__pb2.ConnRequest.SerializeToString,
                response_deserializer=machrpc__pb2.ConnResponse.FromString,
                )
        self.ConnClose = channel.unary_unary(
                '/machrpc.Machbase/ConnClose',
                request_serializer=machrpc__pb2.ConnCloseRequest.SerializeToString,
                response_deserializer=machrpc__pb2.ConnCloseResponse.FromString,
                )
        self.Ping = channel.unary_unary(
                '/machrpc.Machbase/Ping',
                request_serializer=machrpc__pb2.PingRequest.SerializeToString,
                response_deserializer=machrpc__pb2.PingResponse.FromString,
                )
        self.Exec = channel.unary_unary(
                '/machrpc.Machbase/Exec',
                request_serializer=machrpc__pb2.ExecRequest.SerializeToString,
                response_deserializer=machrpc__pb2.ExecResponse.FromString,
                )
        self.QueryRow = channel.unary_unary(
                '/machrpc.Machbase/QueryRow',
                request_serializer=machrpc__pb2.QueryRowRequest.SerializeToString,
                response_deserializer=machrpc__pb2.QueryRowResponse.FromString,
                )
        self.Query = channel.unary_unary(
                '/machrpc.Machbase/Query',
                request_serializer=machrpc__pb2.QueryRequest.SerializeToString,
                response_deserializer=machrpc__pb2.QueryResponse.FromString,
                )
        self.Columns = channel.unary_unary(
                '/machrpc.Machbase/Columns',
                request_serializer=machrpc__pb2.RowsHandle.SerializeToString,
                response_deserializer=machrpc__pb2.ColumnsResponse.FromString,
                )
        self.RowsFetch = channel.unary_unary(
                '/machrpc.Machbase/RowsFetch',
                request_serializer=machrpc__pb2.RowsHandle.SerializeToString,
                response_deserializer=machrpc__pb2.RowsFetchResponse.FromString,
                )
        self.RowsClose = channel.unary_unary(
                '/machrpc.Machbase/RowsClose',
                request_serializer=machrpc__pb2.RowsHandle.SerializeToString,
                response_deserializer=machrpc__pb2.RowsCloseResponse.FromString,
                )
        self.Appender = channel.unary_unary(
                '/machrpc.Machbase/Appender',
                request_serializer=machrpc__pb2.AppenderRequest.SerializeToString,
                response_deserializer=machrpc__pb2.AppenderResponse.FromString,
                )
        self.Append = channel.stream_unary(
                '/machrpc.Machbase/Append',
                request_serializer=machrpc__pb2.AppendData.SerializeToString,
                response_deserializer=machrpc__pb2.AppendDone.FromString,
                )
        self.Explain = channel.unary_unary(
                '/machrpc.Machbase/Explain',
                request_serializer=machrpc__pb2.ExplainRequest.SerializeToString,
                response_deserializer=machrpc__pb2.ExplainResponse.FromString,
                )
        self.UserAuth = channel.unary_unary(
                '/machrpc.Machbase/UserAuth',
                request_serializer=machrpc__pb2.UserAuthRequest.SerializeToString,
                response_deserializer=machrpc__pb2.UserAuthResponse.FromString,
                )
        self.GetServerInfo = channel.unary_unary(
                '/machrpc.Machbase/GetServerInfo',
                request_serializer=machrpc__pb2.ServerInfoRequest.SerializeToString,
                response_deserializer=machrpc__pb2.ServerInfo.FromString,
                )
        self.GetServicePorts = channel.unary_unary(
                '/machrpc.Machbase/GetServicePorts',
                request_serializer=machrpc__pb2.ServicePortsRequest.SerializeToString,
                response_deserializer=machrpc__pb2.ServicePorts.FromString,
                )
        self.Sessions = channel.unary_unary(
                '/machrpc.Machbase/Sessions',
                request_serializer=machrpc__pb2.SessionsRequest.SerializeToString,
                response_deserializer=machrpc__pb2.SessionsResponse.FromString,
                )


class MachbaseServicer(object):
    """Missing associated documentation comment in .proto file."""

    def Conn(self, request, context):
        """Missing associated documentation comment in .proto file."""
        context.set_code(grpc.StatusCode.UNIMPLEMENTED)
        context.set_details('Method not implemented!')
        raise NotImplementedError('Method not implemented!')

    def ConnClose(self, request, context):
        """Missing associated documentation comment in .proto file."""
        context.set_code(grpc.StatusCode.UNIMPLEMENTED)
        context.set_details('Method not implemented!')
        raise NotImplementedError('Method not implemented!')

    def Ping(self, request, context):
        """Missing associated documentation comment in .proto file."""
        context.set_code(grpc.StatusCode.UNIMPLEMENTED)
        context.set_details('Method not implemented!')
        raise NotImplementedError('Method not implemented!')

    def Exec(self, request, context):
        """Missing associated documentation comment in .proto file."""
        context.set_code(grpc.StatusCode.UNIMPLEMENTED)
        context.set_details('Method not implemented!')
        raise NotImplementedError('Method not implemented!')

    def QueryRow(self, request, context):
        """Missing associated documentation comment in .proto file."""
        context.set_code(grpc.StatusCode.UNIMPLEMENTED)
        context.set_details('Method not implemented!')
        raise NotImplementedError('Method not implemented!')

    def Query(self, request, context):
        """Missing associated documentation comment in .proto file."""
        context.set_code(grpc.StatusCode.UNIMPLEMENTED)
        context.set_details('Method not implemented!')
        raise NotImplementedError('Method not implemented!')

    def Columns(self, request, context):
        """Missing associated documentation comment in .proto file."""
        context.set_code(grpc.StatusCode.UNIMPLEMENTED)
        context.set_details('Method not implemented!')
        raise NotImplementedError('Method not implemented!')

    def RowsFetch(self, request, context):
        """Missing associated documentation comment in .proto file."""
        context.set_code(grpc.StatusCode.UNIMPLEMENTED)
        context.set_details('Method not implemented!')
        raise NotImplementedError('Method not implemented!')

    def RowsClose(self, request, context):
        """Missing associated documentation comment in .proto file."""
        context.set_code(grpc.StatusCode.UNIMPLEMENTED)
        context.set_details('Method not implemented!')
        raise NotImplementedError('Method not implemented!')

    def Appender(self, request, context):
        """Missing associated documentation comment in .proto file."""
        context.set_code(grpc.StatusCode.UNIMPLEMENTED)
        context.set_details('Method not implemented!')
        raise NotImplementedError('Method not implemented!')

    def Append(self, request_iterator, context):
        """Missing associated documentation comment in .proto file."""
        context.set_code(grpc.StatusCode.UNIMPLEMENTED)
        context.set_details('Method not implemented!')
        raise NotImplementedError('Method not implemented!')

    def Explain(self, request, context):
        """Missing associated documentation comment in .proto file."""
        context.set_code(grpc.StatusCode.UNIMPLEMENTED)
        context.set_details('Method not implemented!')
        raise NotImplementedError('Method not implemented!')

    def UserAuth(self, request, context):
        """Missing associated documentation comment in .proto file."""
        context.set_code(grpc.StatusCode.UNIMPLEMENTED)
        context.set_details('Method not implemented!')
        raise NotImplementedError('Method not implemented!')

    def GetServerInfo(self, request, context):
        """Missing associated documentation comment in .proto file."""
        context.set_code(grpc.StatusCode.UNIMPLEMENTED)
        context.set_details('Method not implemented!')
        raise NotImplementedError('Method not implemented!')

    def GetServicePorts(self, request, context):
        """Missing associated documentation comment in .proto file."""
        context.set_code(grpc.StatusCode.UNIMPLEMENTED)
        context.set_details('Method not implemented!')
        raise NotImplementedError('Method not implemented!')

    def Sessions(self, request, context):
        """Missing associated documentation comment in .proto file."""
        context.set_code(grpc.StatusCode.UNIMPLEMENTED)
        context.set_details('Method not implemented!')
        raise NotImplementedError('Method not implemented!')


def add_MachbaseServicer_to_server(servicer, server):
    rpc_method_handlers = {
            'Conn': grpc.unary_unary_rpc_method_handler(
                    servicer.Conn,
                    request_deserializer=machrpc__pb2.ConnRequest.FromString,
                    response_serializer=machrpc__pb2.ConnResponse.SerializeToString,
            ),
            'ConnClose': grpc.unary_unary_rpc_method_handler(
                    servicer.ConnClose,
                    request_deserializer=machrpc__pb2.ConnCloseRequest.FromString,
                    response_serializer=machrpc__pb2.ConnCloseResponse.SerializeToString,
            ),
            'Ping': grpc.unary_unary_rpc_method_handler(
                    servicer.Ping,
                    request_deserializer=machrpc__pb2.PingRequest.FromString,
                    response_serializer=machrpc__pb2.PingResponse.SerializeToString,
            ),
            'Exec': grpc.unary_unary_rpc_method_handler(
                    servicer.Exec,
                    request_deserializer=machrpc__pb2.ExecRequest.FromString,
                    response_serializer=machrpc__pb2.ExecResponse.SerializeToString,
            ),
            'QueryRow': grpc.unary_unary_rpc_method_handler(
                    servicer.QueryRow,
                    request_deserializer=machrpc__pb2.QueryRowRequest.FromString,
                    response_serializer=machrpc__pb2.QueryRowResponse.SerializeToString,
            ),
            'Query': grpc.unary_unary_rpc_method_handler(
                    servicer.Query,
                    request_deserializer=machrpc__pb2.QueryRequest.FromString,
                    response_serializer=machrpc__pb2.QueryResponse.SerializeToString,
            ),
            'Columns': grpc.unary_unary_rpc_method_handler(
                    servicer.Columns,
                    request_deserializer=machrpc__pb2.RowsHandle.FromString,
                    response_serializer=machrpc__pb2.ColumnsResponse.SerializeToString,
            ),
            'RowsFetch': grpc.unary_unary_rpc_method_handler(
                    servicer.RowsFetch,
                    request_deserializer=machrpc__pb2.RowsHandle.FromString,
                    response_serializer=machrpc__pb2.RowsFetchResponse.SerializeToString,
            ),
            'RowsClose': grpc.unary_unary_rpc_method_handler(
                    servicer.RowsClose,
                    request_deserializer=machrpc__pb2.RowsHandle.FromString,
                    response_serializer=machrpc__pb2.RowsCloseResponse.SerializeToString,
            ),
            'Appender': grpc.unary_unary_rpc_method_handler(
                    servicer.Appender,
                    request_deserializer=machrpc__pb2.AppenderRequest.FromString,
                    response_serializer=machrpc__pb2.AppenderResponse.SerializeToString,
            ),
            'Append': grpc.stream_unary_rpc_method_handler(
                    servicer.Append,
                    request_deserializer=machrpc__pb2.AppendData.FromString,
                    response_serializer=machrpc__pb2.AppendDone.SerializeToString,
            ),
            'Explain': grpc.unary_unary_rpc_method_handler(
                    servicer.Explain,
                    request_deserializer=machrpc__pb2.ExplainRequest.FromString,
                    response_serializer=machrpc__pb2.ExplainResponse.SerializeToString,
            ),
            'UserAuth': grpc.unary_unary_rpc_method_handler(
                    servicer.UserAuth,
                    request_deserializer=machrpc__pb2.UserAuthRequest.FromString,
                    response_serializer=machrpc__pb2.UserAuthResponse.SerializeToString,
            ),
            'GetServerInfo': grpc.unary_unary_rpc_method_handler(
                    servicer.GetServerInfo,
                    request_deserializer=machrpc__pb2.ServerInfoRequest.FromString,
                    response_serializer=machrpc__pb2.ServerInfo.SerializeToString,
            ),
            'GetServicePorts': grpc.unary_unary_rpc_method_handler(
                    servicer.GetServicePorts,
                    request_deserializer=machrpc__pb2.ServicePortsRequest.FromString,
                    response_serializer=machrpc__pb2.ServicePorts.SerializeToString,
            ),
            'Sessions': grpc.unary_unary_rpc_method_handler(
                    servicer.Sessions,
                    request_deserializer=machrpc__pb2.SessionsRequest.FromString,
                    response_serializer=machrpc__pb2.SessionsResponse.SerializeToString,
            ),
    }
    generic_handler = grpc.method_handlers_generic_handler(
            'machrpc.Machbase', rpc_method_handlers)
    server.add_generic_rpc_handlers((generic_handler,))


 # This class is part of an EXPERIMENTAL API.
class Machbase(object):
    """Missing associated documentation comment in .proto file."""

    @staticmethod
    def Conn(request,
            target,
            options=(),
            channel_credentials=None,
            call_credentials=None,
            insecure=False,
            compression=None,
            wait_for_ready=None,
            timeout=None,
            metadata=None):
        return grpc.experimental.unary_unary(request, target, '/machrpc.Machbase/Conn',
            machrpc__pb2.ConnRequest.SerializeToString,
            machrpc__pb2.ConnResponse.FromString,
            options, channel_credentials,
            insecure, call_credentials, compression, wait_for_ready, timeout, metadata)

    @staticmethod
    def ConnClose(request,
            target,
            options=(),
            channel_credentials=None,
            call_credentials=None,
            insecure=False,
            compression=None,
            wait_for_ready=None,
            timeout=None,
            metadata=None):
        return grpc.experimental.unary_unary(request, target, '/machrpc.Machbase/ConnClose',
            machrpc__pb2.ConnCloseRequest.SerializeToString,
            machrpc__pb2.ConnCloseResponse.FromString,
            options, channel_credentials,
            insecure, call_credentials, compression, wait_for_ready, timeout, metadata)

    @staticmethod
    def Ping(request,
            target,
            options=(),
            channel_credentials=None,
            call_credentials=None,
            insecure=False,
            compression=None,
            wait_for_ready=None,
            timeout=None,
            metadata=None):
        return grpc.experimental.unary_unary(request, target, '/machrpc.Machbase/Ping',
            machrpc__pb2.PingRequest.SerializeToString,
            machrpc__pb2.PingResponse.FromString,
            options, channel_credentials,
            insecure, call_credentials, compression, wait_for_ready, timeout, metadata)

    @staticmethod
    def Exec(request,
            target,
            options=(),
            channel_credentials=None,
            call_credentials=None,
            insecure=False,
            compression=None,
            wait_for_ready=None,
            timeout=None,
            metadata=None):
        return grpc.experimental.unary_unary(request, target, '/machrpc.Machbase/Exec',
            machrpc__pb2.ExecRequest.SerializeToString,
            machrpc__pb2.ExecResponse.FromString,
            options, channel_credentials,
            insecure, call_credentials, compression, wait_for_ready, timeout, metadata)

    @staticmethod
    def QueryRow(request,
            target,
            options=(),
            channel_credentials=None,
            call_credentials=None,
            insecure=False,
            compression=None,
            wait_for_ready=None,
            timeout=None,
            metadata=None):
        return grpc.experimental.unary_unary(request, target, '/machrpc.Machbase/QueryRow',
            machrpc__pb2.QueryRowRequest.SerializeToString,
            machrpc__pb2.QueryRowResponse.FromString,
            options, channel_credentials,
            insecure, call_credentials, compression, wait_for_ready, timeout, metadata)

    @staticmethod
    def Query(request,
            target,
            options=(),
            channel_credentials=None,
            call_credentials=None,
            insecure=False,
            compression=None,
            wait_for_ready=None,
            timeout=None,
            metadata=None):
        return grpc.experimental.unary_unary(request, target, '/machrpc.Machbase/Query',
            machrpc__pb2.QueryRequest.SerializeToString,
            machrpc__pb2.QueryResponse.FromString,
            options, channel_credentials,
            insecure, call_credentials, compression, wait_for_ready, timeout, metadata)

    @staticmethod
    def Columns(request,
            target,
            options=(),
            channel_credentials=None,
            call_credentials=None,
            insecure=False,
            compression=None,
            wait_for_ready=None,
            timeout=None,
            metadata=None):
        return grpc.experimental.unary_unary(request, target, '/machrpc.Machbase/Columns',
            machrpc__pb2.RowsHandle.SerializeToString,
            machrpc__pb2.ColumnsResponse.FromString,
            options, channel_credentials,
            insecure, call_credentials, compression, wait_for_ready, timeout, metadata)

    @staticmethod
    def RowsFetch(request,
            target,
            options=(),
            channel_credentials=None,
            call_credentials=None,
            insecure=False,
            compression=None,
            wait_for_ready=None,
            timeout=None,
            metadata=None):
        return grpc.experimental.unary_unary(request, target, '/machrpc.Machbase/RowsFetch',
            machrpc__pb2.RowsHandle.SerializeToString,
            machrpc__pb2.RowsFetchResponse.FromString,
            options, channel_credentials,
            insecure, call_credentials, compression, wait_for_ready, timeout, metadata)

    @staticmethod
    def RowsClose(request,
            target,
            options=(),
            channel_credentials=None,
            call_credentials=None,
            insecure=False,
            compression=None,
            wait_for_ready=None,
            timeout=None,
            metadata=None):
        return grpc.experimental.unary_unary(request, target, '/machrpc.Machbase/RowsClose',
            machrpc__pb2.RowsHandle.SerializeToString,
            machrpc__pb2.RowsCloseResponse.FromString,
            options, channel_credentials,
            insecure, call_credentials, compression, wait_for_ready, timeout, metadata)

    @staticmethod
    def Appender(request,
            target,
            options=(),
            channel_credentials=None,
            call_credentials=None,
            insecure=False,
            compression=None,
            wait_for_ready=None,
            timeout=None,
            metadata=None):
        return grpc.experimental.unary_unary(request, target, '/machrpc.Machbase/Appender',
            machrpc__pb2.AppenderRequest.SerializeToString,
            machrpc__pb2.AppenderResponse.FromString,
            options, channel_credentials,
            insecure, call_credentials, compression, wait_for_ready, timeout, metadata)

    @staticmethod
    def Append(request_iterator,
            target,
            options=(),
            channel_credentials=None,
            call_credentials=None,
            insecure=False,
            compression=None,
            wait_for_ready=None,
            timeout=None,
            metadata=None):
        return grpc.experimental.stream_unary(request_iterator, target, '/machrpc.Machbase/Append',
            machrpc__pb2.AppendData.SerializeToString,
            machrpc__pb2.AppendDone.FromString,
            options, channel_credentials,
            insecure, call_credentials, compression, wait_for_ready, timeout, metadata)

    @staticmethod
    def Explain(request,
            target,
            options=(),
            channel_credentials=None,
            call_credentials=None,
            insecure=False,
            compression=None,
            wait_for_ready=None,
            timeout=None,
            metadata=None):
        return grpc.experimental.unary_unary(request, target, '/machrpc.Machbase/Explain',
            machrpc__pb2.ExplainRequest.SerializeToString,
            machrpc__pb2.ExplainResponse.FromString,
            options, channel_credentials,
            insecure, call_credentials, compression, wait_for_ready, timeout, metadata)

    @staticmethod
    def UserAuth(request,
            target,
            options=(),
            channel_credentials=None,
            call_credentials=None,
            insecure=False,
            compression=None,
            wait_for_ready=None,
            timeout=None,
            metadata=None):
        return grpc.experimental.unary_unary(request, target, '/machrpc.Machbase/UserAuth',
            machrpc__pb2.UserAuthRequest.SerializeToString,
            machrpc__pb2.UserAuthResponse.FromString,
            options, channel_credentials,
            insecure, call_credentials, compression, wait_for_ready, timeout, metadata)

    @staticmethod
    def GetServerInfo(request,
            target,
            options=(),
            channel_credentials=None,
            call_credentials=None,
            insecure=False,
            compression=None,
            wait_for_ready=None,
            timeout=None,
            metadata=None):
        return grpc.experimental.unary_unary(request, target, '/machrpc.Machbase/GetServerInfo',
            machrpc__pb2.ServerInfoRequest.SerializeToString,
            machrpc__pb2.ServerInfo.FromString,
            options, channel_credentials,
            insecure, call_credentials, compression, wait_for_ready, timeout, metadata)

    @staticmethod
    def GetServicePorts(request,
            target,
            options=(),
            channel_credentials=None,
            call_credentials=None,
            insecure=False,
            compression=None,
            wait_for_ready=None,
            timeout=None,
            metadata=None):
        return grpc.experimental.unary_unary(request, target, '/machrpc.Machbase/GetServicePorts',
            machrpc__pb2.ServicePortsRequest.SerializeToString,
            machrpc__pb2.ServicePorts.FromString,
            options, channel_credentials,
            insecure, call_credentials, compression, wait_for_ready, timeout, metadata)

    @staticmethod
    def Sessions(request,
            target,
            options=(),
            channel_credentials=None,
            call_credentials=None,
            insecure=False,
            compression=None,
            wait_for_ready=None,
            timeout=None,
            metadata=None):
        return grpc.experimental.unary_unary(request, target, '/machrpc.Machbase/Sessions',
            machrpc__pb2.SessionsRequest.SerializeToString,
            machrpc__pb2.SessionsResponse.FromString,
            options, channel_credentials,
            insecure, call_credentials, compression, wait_for_ready, timeout, metadata)