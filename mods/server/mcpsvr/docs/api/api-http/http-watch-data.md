# Machbase Neo HTTP Watch Latest Data

## Using server-sent events

Clients can receive streaming events from the server,
which is useful for keeping the latest records of the specified table up-to-date.

Server-Sent Events (SSE) is a technology that allows a server to push updates to a client over a single HTTP connection.
It is commonly used for applications that require continuous data updates, such as live feeds, notifications, or real-time analytics.

**Key Features of Server-Sent Events (SSE)**

1. **Unidirectional Communication**: The server can send updates to the client, but the client cannot send data back to the server over the same connection.
2. **Persistent Connection**: The client establishes a single HTTP connection that remains open, allowing the server to send updates as they become available.
3. **Automatic Reconnection**: If the connection is lost, the client will automatically attempt to reconnect.
4. **Simple API**: SSE uses a simple API that is easy to implement and use in web applications.


**How SSE Works**
1. **Client Requests Updates**: The client makes an HTTP request to the server to start receiving updates.
2. **Server Sends Updates**: The server responds with a stream of updates, each formatted as a text/event-stream MIME type.
3. **Client Processes Updates**: The client processes the updates as they are received, typically updating the user interface in real-time.

Web browsers impose limits on the number of concurrent Server-Sent Events (SSE) connections that can be opened to a single host. This is done to prevent resource exhaustion and ensure fair usage of network resources. Here are some key points regarding these limits:

**Browser Limits on SSE Connections**

1. **Connection Limit**: Most modern browsers limit the number of concurrent SSE connections to a single host. This limit is typically set to 6 connections per host.
2. **Resource Management**: Limiting the number of connections helps manage resources such as memory and network bandwidth, preventing a single page from overwhelming the browser or the server.
3. **Fair Usage**: By imposing these limits, browsers ensure that multiple tabs or applications can share network resources fairly without one application monopolizing the connections.

Understanding and respecting the browser limits on SSE connections is crucial for building robust and efficient real-time web applications. By designing your application with these limits in mind, you can ensure a smooth user experience and optimal resource usage.


## Watch the latest data

The endpoint of SSE(server-sent events) is:

```
/db/watch/{table}
```

The *watch* api supports query parameters:

| param       | default | description                   |
|:----------- |---------|:----------------------------- |
| timeformat  | `ns`     | Time format for output: s, ms, us, ns    |
| tz          | `UTC`    | Time Zone for output: UTC, Local and location spec |
| period      | `3s`     | Refresh period                |
| keep-alive  | `30s`    | Interval at which the server sends a comment message to maintain the connection and prevent TCP timeout |


**Tag Table**

If the target table is tag table, the parameter `tag` is required.

| param       | default | description                   |
|:----------- |---------|:----------------------------- |
| **tag**     |         | Specify tag name array        |
| parallelism | `0`     | Determines the number of parallel processing.<br/>If set to 0 or a value greater than the number of tags,<br/>it defaults to the number of tags. |

**Note: This API delivers only the latest data for each tag within the specified *period*. If multiple values are inserted during this period, the server will send only the most recent value.**

**Log Table**

| param       | default | description                   |
|:----------- |---------|:----------------------------- |
| max-rows    | `20`   | Maximum number of records the server sends in a period.<br/>If there are more records than specified,<br/> the server will omit the excess data for that period.<br/>The hard limit is 100.|

## cURL Example

Use *curl* command to receive stream of the lates values of the tags:

```sh
curl -o - -v "http://127.0.0.1:5654/db/watch/example"\
"?tag=neo_load1&tag=neo_load5&period=3s&timeformat=s"
```

The server continuously sends a data stream while the client maintains the connection:

```sh
data: {"NAME":"neo_load1","TIME":1729070964,"VALUE":1.87}

data: {"NAME":"neo_load5","TIME":1729070964,"VALUE":1.37}

data: {"NAME":"neo_load1","TIME":1729070969,"VALUE":1.8}

data: {"NAME":"neo_load5","TIME":1729070969,"VALUE":1.36}

^C
```

## Javascript Example

```html
<html>
<body>
    <h1>Server-Sent Events Example</h1>
    <div id="messages"></div>
    <script>
        // Create a new EventSource instance
        const addr = 'http://127.0.0.1:5654/db/watch/EXAMPLE';
        const params = 'tag=neo_load1&tag=neo_load5&period=3s&keep-alive=30s&timeformat=default';
        const eventSource = new EventSource(`${addr}?${params}`);

        // Get the messages div
        const messagesDiv = document.getElementById('messages');

        // Handle incoming messages
        eventSource.onmessage = function (event) {
            // Create a new element
            const pre = document.createElement('pre');
            const msg = JSON.parse(event.data);
            // Set the text content to the event data
            pre.textContent = event.data + ' => ' + msg.NAME + ':' + msg.VALUE;
            // Append the element to the messages div
            messagesDiv.appendChild(pre);
        };

        // Handle errors
        eventSource.onerror = function (event) {
            console.error('EventSource failed:', event);
        };
    </script>
</body>
</html>
```

## Python Example

```python
import requests
import sseclient

# Define the URL to connect to the server-sent events endpoint
url = 'http://127.0.0.1:5654/db/watch/EXAMPLE'
params = {
    'tag': ['neo_load1', 'neo_load5'],
    'period': '3s',
    'keep-alive': '30s',
    'timeformat': 'default'
}

# Create a streaming request
response = requests.get(url, params=params, stream=True)

# Use sseclient to handle the server-sent events
client = sseclient.SSEClient(response)

# Print the received messages
for event in client.events():
    print(event.data)
```