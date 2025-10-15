# Machbase Neo MQTT C# Client

## Setup

### Install dotnet-sdk

```sh
brew install dotnet-sdk
```

### Create project directory

```sh
mkdir csharp-mqtt && cd csharp-mqtt
```

### Create console project

```sh
dotnet new console --framework net7.0
```

### Add MQTTnet packages

```sh
dotnet add package MQTTnet --version 4.3.3.952
```

## Connect (non-TLS)

Connect to machbase-neo via MQTT plain socket.

```c#
var mqttFactory = new MqttFactory();
var mqttClient = mqttFactory.CreateMqttClient();
var connectOptions = new MqttClientOptionsBuilder().WithTcpServer("127.0.0.1", 5653).Build();
var connAck = await mqttClient.ConnectAsync(connectOptions, CancellationToken.None);

connAck.DumpToConsole();
```

## Disconnect

```c#
var mqttClientDisconnectOptions = mqttFactory.CreateClientDisconnectOptionsBuilder().Build();
await mqttClient.DisconnectAsync(mqttClientDisconnectOptions, CancellationToken.None);
```

## Publish message

```c#
var msg = new MqttApplicationMessageBuilder()
.WithTopic("db/append/example")
.WithPayload(@"[
                [""temperature"",1677033057000000000,21.1],
                [""humidity"",1677033057000000000,0.53]
            ]")
.Build();

await mqttClient.PublishAsync(msg, CancellationToken.None);
```

## Full source code

```c#
using MQTTnet;
using MQTTnet.Client;
using System.Text.Json;

namespace MqttTest
{
    internal class Program
    {
        private static async Task Main()
        {
            var mqttFactory = new MqttFactory();
            var mqttClient = mqttFactory.CreateMqttClient();
            var connectOptions = new MqttClientOptionsBuilder().WithTcpServer("127.0.0.1", 5653).Build();
            var connAck = await mqttClient.ConnectAsync(connectOptions, CancellationToken.None);
            
            connAck.DumpToConsole();

            var msg = new MqttApplicationMessageBuilder()
                .WithTopic("db/append/example")
                .WithPayload(@"[
                                [""temperature"",1677033057000000000,21.1],
                                [""humidity"",1677033057000000000,0.53]
                            ]")
                .Build();

            await mqttClient.PublishAsync(msg, CancellationToken.None);

            var mqttClientDisconnectOptions = mqttFactory.CreateClientDisconnectOptionsBuilder().Build();
            await mqttClient.DisconnectAsync(mqttClientDisconnectOptions, CancellationToken.None);
        }
    }
}

internal static class ObjectExtensions
{
    public static TObject DumpToConsole<TObject>(this TObject @object)
    {
        var output = "NULL";
        if (@object != null)
        {
            output = JsonSerializer.Serialize(@object, new JsonSerializerOptions
            {
                WriteIndented = true
            });
        }
        
        Console.WriteLine($"[{@object?.GetType().Name}]:\r\n{output}");
        return @object;
    }
}
```