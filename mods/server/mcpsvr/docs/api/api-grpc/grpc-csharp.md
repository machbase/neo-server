# Machbase Neo gRPC C# Client

## Setup

### Install dotnet-sdk

```sh
brew install dotnet-sdk
```

### Create project directory

```sh
mkdir example-csharp && cd example-csharp
```

### Create console project

```sh
dotnet new console --framework net7.0
```

### Add gRPC packages

```sh
dotnet add package Grpc.Tools
dotnet add package Grpc.Net.Client
dotnet add package Google.Protobuf
```

### Download machrpc.proto

```sh
curl -o machrpc.proto https://raw.githubusercontent.com/machbase/neo-server/main/api/proto/machrpc.proto
```

After downloading proto file, it is required to add csharp_namespace option in the file.

```proto
option csharp_namespace = "MachRpc";
```

### Add ItemGroup in `example-csharp.csproj` XML file

```xml
  <ItemGroup>
    <Protobuf Include="machrpc.proto" GrpcServices="Client"/>
  </ItemGroup>
```

## X.509 Certificates for TLS

gRPC connection requires TLS by default.
Generate application key for the client.

Execute the command below, it generates a new key and register it to the server automatically.

```sh
machbase-neo shell --server 127.0.0.1:5655 key gen "csharp-client" --output "csharp-client"
```

It generates `csharp-client_cert.pem`, `csharp-client_key.pem` and `csharp-client_token`. The client program requires the both of *.pem files.

And the client requires machbase-neo's server certificate as a CA.

```sh
machbase-neo shell --server 127.0.0.1:5655 key server-cert --output "csharp-server.pem"
```

We needs those 3 .pem files.

- `csharp-client_cert.pem` : A client X.509 certificate, signed by machbase-neo server.
- `csharp-client_key.pem` : The client's private key.
- `csharp-server.pem` : machbase-neo server's X.509 certificate, self-signed by machbase-neo server.

The command below shows how to list the certificates that registered in the server.

```sh
$ machbase-neo shell key list

╭────────┬───────────────┬───────────────────────────────┬───────────────────────────────╮
│ ROWNUM │ ID            │ VALID FROM                    │ EXPIRE                        │
├────────┼───────────────┼───────────────────────────────┼───────────────────────────────┤
│      1 │ csharp-client │ 2023-12-21 07:32:30 +0000 UTC │ 2033-12-18 07:32:30 +0000 UTC │
╰────────┴───────────────┴───────────────────────────────┴───────────────────────────────╯
```

## Connect to server 

### TLS

```csharp
var handler = new HttpClientHandler();
handler.SslProtocols = System.Security.Authentication.SslProtocols.Tls12;
handler.ClientCertificateOptions = ClientCertificateOption.Manual;
handler.ClientCertificates.Add(x509);
handler.UseProxy = false;
handler.ServerCertificateCustomValidationCallback = (HttpRequestMessage msg, X509Certificate2? cert, X509Chain? chain, SslPolicyErrors sslPolicyErrors) =>
{
    if (serverCert.Equals(cert)) {
        return true;
    } else {
        System.Console.WriteLine("Server cert, got " + cert!.SubjectName.Name);
        return false;
    }
};

var channel = GrpcChannel.ForAddress("https://127.0.0.1:5655", new GrpcChannelOptions()
{
    HttpHandler = handler,
    DisposeHttpClient = true
});

var client = new MachRpc.Machbase.MachbaseClient(channel);
```

### DB Connect

```c#
connReq = new MachRpc.ConnRequest
{
    User = "sys",
    Password = "manager",
};
connRsp = client.Conn(connReq);
```

## Query

### Execute query

```c#
queryReq = new MachRpc.QueryRequest
{
    Conn = connRsp.Conn,
    Sql = "select * from example order by time limit ?",
    Params = { Any.Pack(new Int32Value { Value = 10 }) }
};
queryRsp = client.Query(queryReq);
```

### Get columns info of result set

```c#
var cols = client.Columns(queryRsp.RowsHandle);
var headers = new List<string> { "RowNum" };
if (cols.Success)
{
    foreach (var c in cols.Columns)
    {
        headers.Add($"{c.Name}({c.Type})");
    }
}
Console.WriteLine(String.Join("   ", headers));
```

This will print column labels.

```
NAME(string)   TIME(datetime)   VALUE(double)
```

### Fetch results

```c#
int nrow = 0;
while (true)
{
    var fetch = client.RowsFetch(queryRsp.RowsHandle);
    if (fetch.HasNoRows)
    {
        break;
    }
    nrow++;
    var line = new List<string> { $"{nrow}   " };
    foreach (Any v in fetch.Values)
    {
        line.Add(convpb(v));
    };
    Console.WriteLine(String.Join("    ", line));
}
```

**Close rows**: Do not forget to close rows by calling `RowsClose()`.

### Convert Google.Protobuf.WellKnownTypes.Any to string

```c#
static string convpb(Any v)
{
    if (v.TypeUrl == "type.googleapis.com/google.protobuf.StringValue")
    {
        var sval = v.Unpack<StringValue>();
        return sval.Value;
    }
    else if (v.TypeUrl == "type.googleapis.com/google.protobuf.Timestamp")
    {
        var ts = v.Unpack<Timestamp>();
        return ts.ToDateTime().ToString("MM/dd/yyyy HH:mm:ss");
    }
    else if (v.TypeUrl == "type.googleapis.com/google.protobuf.DoubleValue")
    {
        var fv = v.Unpack<DoubleValue>();
        return fv.Value.ToString();
    }
    else
    {
        throw new Exception($"Unsupported type {v.TypeUrl}");
    }
}
```

### Output

```
$ dotnet run
RowNum   NAME(string)   TIME(datetime)   VALUE(double)
1       wave.sin    2023. 02. 08 11:36:38    -0.994521
2       wave.cos    2023. 02. 08 11:36:38    -0.104538
3       wave.sin    2023. 02. 08 11:36:37    -0.866021
4       wave.cos    2023. 02. 08 11:36:37    -0.500008
5       wave.cos    2023. 02. 08 11:36:36    -0.809022
6       wave.sin    2023. 02. 08 11:36:36    -0.587778
7       wave.cos    2023. 02. 08 11:36:35    -0.978149
8       wave.sin    2023. 02. 08 11:36:35    -0.207904
9       wave.cos    2023. 02. 08 11:36:34    -0.978146
10       wave.sin    2023. 02. 08 11:36:34    0.207919
```

### Full source code

```csharp
using Grpc.Net.Client;
using Google.Protobuf.WellKnownTypes;
using System.Security.Cryptography.X509Certificates;

internal class Program
{
    private static void Main(string[] args)
    {
        var keyPem = File.ReadAllText("csharp-client_key.pem");
        var certPem = File.ReadAllText("csharp-client_cert.pem");
        var x509 = X509Certificate2.CreateFromPem(certPem, keyPem);
        var serverCert = X509Certificate2.CreateFromCertFile("csharp-server.pem");

        var handler = new HttpClientHandler();
        handler.SslProtocols = System.Security.Authentication.SslProtocols.Tls12;
        handler.ClientCertificateOptions = ClientCertificateOption.Manual;
        handler.ClientCertificates.Add(x509);
        handler.UseProxy = false;
        handler.ServerCertificateCustomValidationCallback = (HttpRequestMessage msg, X509Certificate2? cert, X509Chain? chain, SslPolicyErrors sslPolicyErrors) =>
        {
            if (serverCert.Equals(cert)) {
                return true;
            } else {
                System.Console.WriteLine("Server cert, got " + cert!.SubjectName.Name);
                return false;
            }
        };

        
        var channel = GrpcChannel.ForAddress("https://127.0.0.1:5655", new GrpcChannelOptions()
        {
            HttpHandler = handler,
            DisposeHttpClient = true
        });

        var client = new MachRpc.Machbase.MachbaseClient(channel);

        MachRpc.ConnRequest connReq;
        MachRpc.ConnResponse? connRsp = null;
        MachRpc.QueryRequest queryReq;
        MachRpc.QueryResponse? queryRsp = null;
        try
        {

            connReq = new MachRpc.ConnRequest
            {
                User = "sys",
                Password = "manager",
            };
            connRsp = client.Conn(connReq);
            Console.WriteLine(String.Join("    ", connRsp));

            queryReq = new MachRpc.QueryRequest
            {
                Conn = connRsp.Conn,
                Sql = "select * from example order by time limit ?",
                Params = { Any.Pack(new Int32Value { Value = 10 }) }
            };
            queryRsp = client.Query(queryReq);
            Console.WriteLine(String.Join("    ", queryRsp));

            var cols = client.Columns(queryRsp.RowsHandle);
            var headers = new List<string> { "RowNum" };
            if (cols.Success)
            {
                foreach (var c in cols.Columns)
                {
                    headers.Add($"{c.Name}({c.Type})");
                }
            }
            Console.WriteLine(String.Join("   ", headers));

            int nrow = 0;
            while (true)
            {
                var fetch = client.RowsFetch(queryRsp.RowsHandle);
                if (fetch.HasNoRows)
                {
                    break;
                }
                nrow++;
                var line = new List<string> { $"{nrow}   " };
                foreach (Any v in fetch.Values)
                {
                    line.Add(convpb(v));
                };
                Console.WriteLine(String.Join("    ", line));
            }
        }
        finally
        {
            if (queryRsp != null)
            {
                client.RowsClose(queryRsp.RowsHandle);
            }
            if (connRsp != null)
            {
                client.ConnClose(new MachRpc.ConnCloseRequest { Conn = connRsp.Conn });
            }
        }
    }

    static string convpb(Any v)
    {
        if (v.TypeUrl == "type.googleapis.com/google.protobuf.StringValue")
        {
            var sval = v.Unpack<StringValue>();
            return sval.Value;
        }
        else if (v.TypeUrl == "type.googleapis.com/google.protobuf.Timestamp")
        {
            var ts = v.Unpack<Timestamp>();
            return ts.ToDateTime().ToString("yyyy/MM/dd HH:mm:ss");
        }
        else if (v.TypeUrl == "type.googleapis.com/google.protobuf.DoubleValue")
        {
            var fv = v.Unpack<DoubleValue>();
            return fv.Value.ToString();
        }
        else
        {
            throw new Exception($"Unsupported type {v.TypeUrl}");
        }
    }
}
```

## Append

### Prepare new appender

Create appender from the connection.

```c#
var appender = client.Appender(new MachRpc.AppenderRequest
{
    Conn = connRsp.Conn,
    TableName = "example",
});
var stream = client.Append();
```

Do not forget to close the stream.

```c#
try {
    // code that use stream & appender.Handle
}
finally {
    await stream.RequestStream.CompleteAsync();
}
```

Make `Main()` as `async Task Main()` to allow await for async operation.

```c#
private static async Task Main(string[] args) {
    /// use await
}
```

### Write data in high speed

```c#
for (int i = 0; i < 100000; i++)
{
    var fieldName = new MachRpc.AppendDatum() { VString = "csharp.value" };
    var fieldTime = new MachRpc.AppendDatum() { VTime = TimeUtils.GetNanoseconds() };
    var fieldValue = new MachRpc.AppendDatum() { VDouble = 0.1234 };

    var record = new MachRpc.AppendRecord();
    record.Tuple.Add(fieldName);
    record.Tuple.Add(fieldTime);
    record.Tuple.Add(fieldValue);

    var data = new MachRpc.AppendData { Handle = appender.Handle };
    data.Records.Add(record);
    await stream.RequestStream.WriteAsync(data);
}
```

### Run and count written records

```sh
dotnet run
```

```sh
machbase-neo shell "select count(*) from example where name = 'csharp.value'"
 #  COUNT(*)
─────────────
 1  100000
```

### Full source code

```csharp
using Grpc.Net.Client;
using System.Net.Security;
using System.Security.Cryptography.X509Certificates;
using System.Diagnostics;

internal class Program
{
    private static async Task Main(string[] args)
    {
        var keyPem = File.ReadAllText("csharp-client_key.pem");
        var certPem = File.ReadAllText("csharp-client_cert.pem");
        var x509 = X509Certificate2.CreateFromPem(certPem, keyPem);
        var serverCert = X509Certificate2.CreateFromCertFile("csharp-server.pem");

        var handler = new HttpClientHandler();
        handler.SslProtocols = System.Security.Authentication.SslProtocols.Tls12;
        handler.ClientCertificateOptions = ClientCertificateOption.Manual;
        handler.ClientCertificates.Add(x509);
        handler.UseProxy = false;
        handler.ServerCertificateCustomValidationCallback = (HttpRequestMessage msg, X509Certificate2? cert, X509Chain? chain, SslPolicyErrors sslPolicyErrors) =>
        {
            if (serverCert.Equals(cert)) {
                return true;
            } else {
                System.Console.WriteLine("Server cert, got " + cert!.SubjectName.Name);
                return false;
            }
        };

        var channel = GrpcChannel.ForAddress("https://127.0.0.1:5655", new GrpcChannelOptions()
        {
            HttpHandler = handler,
            DisposeHttpClient = true
        });

        var client = new MachRpc.Machbase.MachbaseClient(channel);

        var connReq = new MachRpc.ConnRequest
        {
            User = "sys",
            Password = "manager",
        };
        var connRsp = client.Conn(connReq);
        Console.WriteLine(String.Join("    ", connRsp));

        var appender = client.Appender(new MachRpc.AppenderRequest
        {
            Conn = connRsp.Conn,
            TableName = "example",
        });
        var stream = client.Append();

        var stopwatch = new Stopwatch();
        stopwatch.Start();
        try
        {
            for (int i = 0; i < 100000; i++)
            {
                var fieldName = new MachRpc.AppendDatum() { VString = "csharp.value" };
                var fieldTime = new MachRpc.AppendDatum() { VTime = TimeUtils.GetNanoseconds() };
                var fieldValue = new MachRpc.AppendDatum() { VDouble = 0.1234 };

                var record = new MachRpc.AppendRecord();
                record.Tuple.Add(fieldName);
                record.Tuple.Add(fieldTime);
                record.Tuple.Add(fieldValue);

                var data = new MachRpc.AppendData { Handle = appender.Handle };
                data.Records.Add(record);
                await stream.RequestStream.WriteAsync(data);
            }
        }
        finally
        {
            await stream.RequestStream.CompleteAsync();
            stopwatch.Stop();
            var elapsed_time = stopwatch.ElapsedMilliseconds;
            Console.WriteLine($"Elapse {elapsed_time}ms.");

            if (connRsp != null)
            {
                client.ConnClose(new MachRpc.ConnCloseRequest { Conn = connRsp.Conn });
            }
        }
    }

    public static class TimeUtils
    {
        public static long GetNanoseconds()
        {
            double timestamp = Stopwatch.GetTimestamp();
            double nanoseconds = 1_000_000_000.0 * timestamp / Stopwatch.Frequency;
            return (long)nanoseconds;
        }
    }
}
```