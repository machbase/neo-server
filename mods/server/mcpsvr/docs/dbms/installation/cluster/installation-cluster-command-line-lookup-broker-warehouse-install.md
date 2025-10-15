# Lookup / Broker / Warehouse Installation

## Lookup Installation

Add a lookup node in the Coordinator node. Multiple lookup nodes can be registered.

The deployer node must be pre-installed on the server.

When the deployer node is installed, all operations are performed on the coordinator node, and there is nothing to set up by connecting to the server.

```bash
# Add lookup master node
$MACHBASE_COORDINATOR_HOME/bin/machcoordinatoradmin --add-node="192.168.0.84:5301"  \
        --node-type=lookup --lookup-type=master --deployer="192.168.0.84:5201"      \
        --home-path="/home/machbase/lookup1"
 
 
# Add lookup monitor node
$MACHBASE_COORDINATOR_HOME/bin/machcoordinatoradmin --add-node="192.168.0.84:5302"  \
        --node-type=lookup --lookup-type=monitor --deployer="192.168.0.84:5201"         \
        --home-path="/home/machbase/lookupm1"
 
 
# Add lookup slave node
$MACHBASE_COORDINATOR_HOME/bin/machcoordinatoradmin --add-node="192.168.0.84:5303"  \
        --node-type=lookup --lookup-type=slave --deployer="192.168.0.84:5201"       \
        --home-path="/home/machbase/lookup3"
 
  
# Run lookup node
$MACHBASE_COORDINATOR_HOME/bin/machcoordinatoradmin --startup-node="192.168.0.84:5301"
 
# You can run lookup nodes in batches
$MACHBASE_COORDINATOR_HOME/bin/machcoordinatoradmin --startup-lookup
```

|Option items|Description|Example|
|--|--|--|
|--add-node|Specifies the node name to be added as "IP: PORT".|192.168.0.84:5301|
|--node-type|Specifies the node type.<br>There are five types: coordinator, deployer, lookup, broker, and warehouse.|lookup|
|--deployer|Register the deployer node information of the server to be installed.|192.168.0.84:5201|
|--lookup-type|Specify the lookup type<br>There are three types: master, slave, monitor.|master|
|--home-path|Specifies the path to install.<br>Specifies /home/machbase/lookup in machbase account.|/home/machabse/lookup|

### Installation Conditions

There are 3 types of Lookup node, Master, Slave, and Monitor, and it must be installed according to the conditions below.

    1. Lookup Master Node
        a. It is a Lookup node that must have only one.
        b. It must be installed before Monitor and Slave nodes.
    2. Lookup Monitor Node
        a. It is a Lookup node that must exist at least one.
        b.For stable HA, there should be one in each server.
    3. Lookup Slave Node
        a. It is recommended that there be more than one for HA. (If not, HA cannot be guaranteed)

## Delete Lookup

Remove the broker node from the Coordinator node.

```bash
# Delete lookup node
$MACHBASE_COORDINATOR_HOME/bin/machcoordinatoradmin --remove-node="192.168.0.84:5301"
```

## Shut Down/Stop Lookup

Shut down / kill the lookup node on the Coordinator node.

```bash

# Terminate lookup node
$MACHBASE_COORDINATOR_HOME/bin/machcoordinatoradmin --shutdown-node="192.168.0.84:5301"
 
# You can terminate lookup nodes in batches
$MACHBASE_COORDINATOR_HOME/bin/machcoordinatoradmin --shutdown-lookup
```

## Change Lookup Master

You can change the lookup master node in the coordinator node.

Only lookup slaves can be changed to lookup masters, and the existing lookup masters become lookup slaves.

```bash
# Change lookup master
$MACHBASE_COORDINATOR_HOME/bin/machcoordinatoradmin --set-lookup-master="192.168.0.84:5301"
```

## Broker Installation

Add a broker node to the coordinator node. Multi-broker node registration is possible.

The deployer node must be installed on the server in advance.

Once the deployer node is installed, there is no setting by connecting to the server as  all work can be done on the coordinator node.

The first registered node becomes the leader broker, and the additional registered node becomes the follower broker.

```bash
# Add broker node.                                
$MACHBASE_COORDINATOR_HOME/bin/machcoordinatoradmin --add-node="192.168.0.84:5301"  \
        --node-type=broker --deployer="192.168.0.84:5201" --port-no="5656"          \
        --home-path="/home/machbase/broker" --package-name=machbase
  
# Run broker node.
$MACHBASE_COORDINATOR_HOME/bin/machcoordinatoradmin --startup-node="192.168.0.84:5301"
```

|Option items|Description|Example|
|--|--|--|
|--add-node|Specifies the node name to be added as "IP: PORT".<br>The PORT value is set to the CLUSTER_LINK_PORT_NO value.|192.168.0.84:5301|
|--node-type|Specifies the node type.<br>There are five types: coordinator, deployer, lookup, broker, and warehouse.|broker|
|--deployer|Register the deployer node information of the server to be installed.|192.168.0.84:5201|
|--port-no|Specifies 'machbased' port.<br>The Broker specifies a default value of 5656.<br>This port is used when connecting to client and machsql.|5656|
|--home-path|Specifies the path to install.<br>Specifies /home/machbase/broker in machbase account|/home/machbase/broker|
|--package-name|Sets the package name specified when package was added.|machbase|

## Delete Broker

Remove the broker node from the Coordinator node.

```bash
# Delete broker node.
$MACHBASE_COORDINATOR_HOME/bin/machcoordinatoradmin --remove-node="192.168.0.84:5301"
```

## Shut Down/Stop Broker

There is a way to shut down / kill the broker node on the Coordinator node.

```bash
# Terminate broker node.
$MACHBASE_COORDINATOR_HOME/bin/machcoordinatoradmin --shutdown-node="192.168.0.84:5301"
  
# Stop broker node.
$MACHBASE_COORDINATOR_HOME/bin/machcoordinatoradmin --kill-node="192.168.0.84:5301"
```

Alternatively, you can shut down / kill the process directly from the server where the broker is installed.

```bash
# Terminate broker node.
$MACHBASE_HOME/bin/machadmin -s
  
# Stop broker node.
$MACHBASE_HOME/bin/machadmin -k
```

## Warehouse Installation

Install the active node and the standby node from the Coordinator node.

They will be installed through a pre-installed deployer.

### Group 1 Installation

Install the first Warehouse Group1 node.

```bash
# Install group1 warehouse.                         
$MACHBASE_COORDINATOR_HOME/bin/machcoordinatoradmin --add-node="192.168.0.83:5401"  \
        --node-type=warehouse --deployer="192.168.0.83:5201" --port-no="5400"       \
        --home-path="/home/machbase/warehouse_g1" --package-name=machbase           \
        --replication="192.168.0.83:5402"  --group="group1" --no-replicate
  
# Run installed node.
$MACHBASE_COORDINATOR_HOME/bin/machcoordinatoradmin --startup-node="192.168.0.84:5401"
```

|Option items|Description|Example|
|--|--|--|
|--add-node|Specifies the node name to be added as "IP: PORT".<br>The PORT value is set to the CLUSTER_LINK_PORT_NO value.|192.168.0.84:5401|
|--node-type|Specifies the node type.<br>There are five types: coordinator, deployer, lookup, broker, and warehouse.|warehouse|
|--deployer|Registers the deployer node information of the server to be installed.|192.168.0.84:5201|
|--port-no|	Specifies the working port of 'machbased'.<br>Since the value was set to 5656 on the Broker, a different port must be specified if it is installed on the same server. The warehouse port number is set as 5400.<br>This port is used when connecting to client and machsql.|5400|
|--home-path|Specifies the path to install. To distinguish the groups, set them in order of warehouse_g1, g2, g3.|/home/machbase/warehouse_g1|
|--package-name|Sets the package name specified when adding the package.|machbase|
|--replication|Specifies the node in charge of replication as "IP: PORT".<br>The port value is set to the warehouse port number 5402.|192.168.0.84:5402|
|--no-replicate|Specifies whether to replicate data when adding a node if there is warehouse data in the group.| |
|--set-group-state|Specifies the state of the group as normal and readonly.<br>Normal is read, write / readonly is read only| |

### Add Node to Group 1

Add another node to Warehouse Group1.

```bash
# Add warehouse node to group1.              
$MACHBASE_COORDINATOR_HOME/bin/machcoordinatoradmin --add-node="192.168.0.84:5401"  \
        --node-type=warehouse --deployer="192.168.0.84:5201" --port-no="5400"       \
        --home-path="/home/machbase/warehouse_g1" --package-name=machbase           \
        --replication="192.168.0.84:5402" --group="group1" --no-replicate
  
# Run installed node.
$MACHBASE_COORDINATOR_HOME/bin/machcoordinatoradmin --startup-node="192.168.0.84:5401"
```

|Option items|Description|Example|
|--|--|--|
|--add-node|Specifies the node name to be added as "IP: PORT".<br>The PORT value is set to the CLUSTER_LINK_PORT_NO value.|192.168.0.84:5401|
|--node-type|Specifies the node type.<br>There are five types: coordinator, deployer, lookup, broker, and warehouse.|warehouse|
|--deployer|Registers the deployer node information of the server to be installed.|192.168.0.84:5201|
|--port-no|Specifies the working port  of 'machbased'.<br>Since the value was set to 5656 on the Broker, a different port must be specified if it is installed on the same server. The warehouse port number is set as 5400.<br>This port is used when connecting to client and machsql.|5400|
|--home-path|Specifies the path to install. To distinguish the groups, set them in order of warehouse_g1, g2, g3.|/home/machabse/warehouse_g1|
|--package-name|Sets the package name specified when adding the package.|machbase|
|--replication|Specify the node in charge of replication as "IP: PORT".<br>The port value is set to the warehouse port number 5402.|192.168.0.84:5402|
|--group|Specifies the Group name.|group1|
|--no-replicate|Specifies whether to replicate data when adding a node if there is warehouse data in the group.| |
|--set-group-state|Specifies the state of the group as normal and readonly.<br>Normal is read, write / readonly is read only| |

## Group 2 Installation

Install the second Warehouse Group2 node.

```bash
# Install group1 warehouse.                         
$MACHBASE_COORDINATOR_HOME/bin/machcoordinatoradmin --add-node="192.168.0.84:5411"  \
        --node-type=warehouse --deployer="192.168.0.84:5201" --port-no="5410"       \
        --home-path="/home/machbase/warehouse_g2" --package-name=machbase           \
        --replication="192.168.0.84:5412"  --group="group2" --no-replicate
  
# Run installed node.
$MACHBASE_COORDINATOR_HOME/bin/machcoordinatoradmin --startup-node="192.168.0.84:5411"
```

|Option items|Description|Example|
|--|--|--|
|--add-node|Specifies the node name to be added as "IP: PORT".<br>The PORT value is set to the CLUSTER_LINK_PORT_NO value.|192.168.0.84:5411|
|--node-type|Specifies the node type.<br>There are five types: coordinator, deployer, lookup, broker, and warehouse.|warehouse|
|--deployer|Registers the deployer node information of the server to be installed.|192.168.0.84:5201|
|--port-no|Specifies the working port  of 'machbased'.<br>Since the value was set to 5656 on the Broker, a different port must be specified if it is installed on the same server. The warehouse port number is set as 5410.<br>This port is used when connecting to client and machsql.|5410|
|--home-path|Specifies the path to install. To distinguish the groups, set them in order of warehouse_g1, g2, g3.|/home/machbase/warehouse_g2
|--package-name|Sets the package name specified when adding the package.|machbase|
|--replication|Specifies the node in charge of replication as "IP: PORT".<br>The port value is set to the warehouse port number 5412.|192.168.0.84:5412|
|--group|Specifies the Group name.|group|
|--no-replicate|Specifies whether to replicate data when adding a node if there is warehouse data in the group.| |
|--set-group-state|Specifies the state of the group as normal and readonly.<br>Normal is read, write / readonly is read only| |

### Add Node to Group 2

Add another node to Warehouse Group2.

```bash
# Add warehouse node to group1.              
$MACHBASE_COORDINATOR_HOME/bin/machcoordinatoradmin --add-node="192.168.0.83:5411"  \
        --node-type=warehouse --deployer="192.168.0.83:5201" --port-no="5410"       \
        --home-path="/home/machbase/warehouse_g2" --package-name=machbase           \
        --replication="192.168.0.83:5412" --group="group2" --no-replicate
  
# Run installed node.
$MACHBASE_COORDINATOR_HOME/bin/machcoordinatoradmin --startup-node="192.168.0.83:5411"
```

|Option items|Description|Example|
|--|--|--|
|--add-node|Specifies the node name to be added as "IP: PORT".<br>The PORT value is set to the CLUSTER_LINK_PORT_NO value.|192.168.0.83:5411|
|--node-type|Specifies the node type.<br>There are five types: coordinator, deployer, lookup, broker, and warehouse.|warehouse|
|--deployer|Registers the deployer node information of the server to be installed.|192.168.0.83:5201|
|--port-no|Specifies the working port of 'machbased'.<br>Since the value was set to 5656 on the Broker, a different port must be specified if it is installed on the same server. The warehouse port number is set as 5400.<br>This port is used when connecting to client and machsql.|5410|
|--home-path|Specifies the path to install. To distinguish the groups, set them in order of warehouse_g1, g2, g3.|/home/machbase/warehouse_g2|
|--package-name|Sets the package name specified when adding the package.|machbase|
|--replication|Specifies the node in charge of replication as "IP: PORT".<br>The port value is set to the warehouse port number 5412.|192.168.0.83:5412|
|--group|Specifies the Group name.|group2|
|--no-replicate|Specifies whether to replicate data when adding a node if there is warehouse data in the group.| |
|--set-group-state|Specifies the state of the group as normal and readonly.<br>Normal is read, write / readonly is read only| |

## Delete Warehouse

Delete the warehouse node from the Coordinator node.

```bash
# Delete warehouse node.
$MACHBASE_COORDINATOR_HOME/bin/machcoordinatoradmin --remove-node="192.168.0.83:5401"
```

## Shut Down/Stop Warehouse

There is a way to shut down / kill the warehouse node at the Coordinator node.

```bash
# Terminate warehouse node.
$MACHBASE_COORDINATOR_HOME/bin/machcoordinatoradmin --shutdown-node="192.168.0.83:5401"
  
# Stop warehouse node.
$MACHBASE_COORDINATOR_HOME/bin/machcoordinatoradmin --kill-node="192.168.0.83:5401"
```

Otherwise, the process can be shut down / killed directly from the server where the warehouse is installed.

```bash
# Terminate warehouse node.
$MACHBASE_HOME/bin/machadmin -s
  
# Stop warehouse node.
$MACHBASE_HOME/bin/machadmin -k
```
