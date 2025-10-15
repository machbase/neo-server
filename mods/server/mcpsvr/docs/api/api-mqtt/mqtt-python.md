# Machbase Neo MQTT Python Client

## Setup

### Install paho

```sh
pip install paho-mqtt
```

### Create project directory

```sh
mkdir python-mqtt && cd python-mqtt
```

## Publisher

### Client

```python
import paho.mqtt.client as mqtt

mqttClient = mqtt.Client("python_pub") # name of publisher
```

### Connect Callback

```python
def on_connect(client, userdata, flags, rc):
    if rc == 0:
        print("CONNACK OK")
    else:
        print("CONNACK KO code=", rc)
```

### Connect (non-TLS)

Connect to machbase-neo via MQTT plain socket.

```python
mqttClient = mqtt.Client("python_pub", clean_session=True)
mqttClient.on_connect = on_connect
mqttClient.connect("127.0.0.1", port=5653, keepalive=10, clean_session=True)
mqttClient.loop_start()
```

### Disconnect

```python
mqttClient.disconnect()
mqttClient.loop_stop()
```

### Publish Callback

```python
def on_publish(client, userdata, mid):
    print("PUBACK mid=",mid)
```

### Publish

```python
mqttClient.on_publish = on_publish

mqttClient.publish("db/append/example", """[
    ["temperature",1677033057000000000, 21.1],
    ["humidity",   1677033057000000000, 0.53]
]""", qos=1)
```

## Full source code

```python
import paho.mqtt.client as mqtt
import time

def on_connect(client, userdata, flags, rc):
    if rc == 0:
        print("CONNACK OK")
    else:
        print("CONNACK KO code:", rc)

def on_publish(client, userdata, mid):
    print("PUBACK mid:",mid)

mqttClient = mqtt.Client("python_pub", clean_session=True)
mqttClient.on_connect = on_connect
mqttClient.on_publish = on_publish
mqttClient.connect("127.0.0.1", port=5653, keepalive=10)
mqttClient.loop_start()

mqttClient.publish("db/append/example", """[
    ["temperature",1677033057000000000, 21.1],
    ["humidity",   1677033057000000000, 0.53]
]""", qos=1)

time.sleep(1)

mqttClient.disconnect()
mqttClient.loop_stop()
```