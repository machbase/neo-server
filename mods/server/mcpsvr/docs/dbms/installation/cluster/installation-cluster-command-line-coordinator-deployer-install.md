# Coordinator / Deployer Installation, Add Package

## Installing Coordinator

### Configuration

Log in with machbase account and execute with machbase privilege.

Configure installation directory and path information.

```bash
# Edit .bashrc.
export MACHBASE_COORDINATOR_HOME=~/coordinator
export MACHBASE_DEPLOYER_HOME=~/deployer
export MACHBASE_HOME=~/coordinator
export PATH=$MACHBASE_HOME/bin:$PATH
export LD_LIBRARY_PATH=$MACHBASE_HOME/lib:$LD_LIBRARY_PATH

# Reflect changes.
source .bashrc
```

### Create and Unzip Directory

Create a dedicated directory and unzip the package archive into that directory.

```bash
# Create directory.
mkdir $MACHBASE_COORDINATOR_HOME

# Unzip.
tar zxvf machbase-ent-x.y.z.official-LINUX-X86-64-release.tgz -C $MACHBASE_COORDINATOR_HOME
```

### Port Configuration and Service Activation

Modify the machbase.conf file to set the port and start the service.

```bash
# Set port from machbase.conf file.
cd $MACHBASE_COORDINATOR_HOME/conf
vi machbase.conf
CLUSTER_LINK_HOST       = 192.168.0.83 (Node ip to be added)
CLUSTER_LINK_PORT_NO    = 5101
HTTP_ADMIN_PORT         = 5102

# Create meta information and run service.
$MACHBASE_COORDINATOR_HOME/bin/machcoordinatoradmin -c
$MACHBASE_COORDINATOR_HOME/bin/machcoordinatoradmin -u
```

### Node Registration and Verification

Add and confirm the Coordinator node.

```bash
# Register node.
$MACHBASE_COORDINATOR_HOME/bin/machcoordinatoradmin --add-node="192.168.0.83:5101" --node-type=coordinator

# Check node.
$MACHBASE_COORDINATOR_HOME/bin/machcoordinatoradmin --cluster-status
```

| Optianl     | Description                                                                                             | Example           |
| ----------- | ------------------------------------------------------------------------------------------------------- | ----------------- |
| --add-node  | Specifies the node name to be added as "IP: PORT".<br>The PORT value is the CLUSTER_LINK_PORT_NO value. | 192.168.0.83:5101 |
| --node-type | Specifies the node type.<br>There are four types: coordinator / deployer / broker / warehouse.          | coordinator       |

## Delete Coordinator

Connect to the server where the Coordinator is installed, terminate the Coordinator process properly, and delete the Coordinator directory.

```bash
# Terminate coordinator and delete directory.
process$MACHBASE_COORDINATOR_HOME/bin/machcoordinatoradmin -s
rm -rf $MACHBASE_COORDINATOR_HOME
```

## Secondary Coordinator Installation

If you install an additional Coordinator in addition to the Primary Coordinator, note the following:

- Before the Startup of the Secondary Coordinator, you must go to the Primary Coordinator and Add-Node the Secondary Coordinator.
- When you start the Secondary Coordinator, you must specify the Primary Coordinator as the --primary option.
- Do not add-node the Primary Coordinator to the Secondary Coordinator.

If this is not followed, the Secondary Coordinator will behave like a Primary Coordinator.

### Create and Unzip Directory

Create a dedicated directory and unzip the package archive into that directory.

```bash

# Create directory.
mkdir $MACHBASE_COORDINATOR_HOME

# Unzip.
tar zxvf machbase-ent-x.y.z.official-LINUX-X86-64-release.tgz -C $MACHBASE_COORDINATOR_HOME
```

### Port Settings

Modify the machbase.conf file to set the port only. **When the service starts, it will work like the Primary Coordinator.**

```bash
# Set port from machbase.conf file.
cd $MACHBASE_COORDINATOR_HOME/conf
vi machbase.conf
CLUSTER_LINK_HOST       = 192.168.0.83 (ip address to be added)
CLUSTER_LINK_PORT_NO    = 5111
HTTP_ADMIN_PORT         = 5112
```

### Node Registration and Verification

**In the Primary Coordinator**, add and confirm the Secondary Coordinator node.

```bash
# Register node.
$MACHBASE_COORDINATOR_HOME/bin/machcoordinatoradmin --add-node="192.168.0.83:5111" --node-type=coordinator

# Check node.
$MACHBASE_COORDINATOR_HOME/bin/machcoordinatoradmin --cluster-status
```

### Run Service

Now run the Secondary Coordinator. During startup, the Primary Coordinator must be specified as a **--primary** option.

```bash
# Create meta information and run service.
$MACHBASE_COORDINATOR_HOME/bin/machcoordinatoradmin -c
$MACHBASE_COORDINATOR_HOME/bin/machcoordinatoradmin -u --primary="192.168.0.83:5101"
```

## Delete Secondary Coordinator

After removing the Secondary Coordinator registered in the Primary Coordinator, the Secondary Coordinator must be terminated properly.

```bash
# Delete node.
$MACHBASE_COORDINATOR_HOME/bin/machcoordinatoradmin --remove-node="192.168.0.83:5101"

# Terminate secondary coordinator and delete directory.
$MACHBASE_COORDINATOR_HOME/bin/machcoordinatoradmin -s
rm -rf $MACHBASE_COORDINATOR_HOME

# Check node.
$MACHBASE_COORDINATOR_HOME/bin/machcoordinatoradmin --cluster-status
```

| Optional      | Description                                                                                                      | Example           |
| ------------- | ---------------------------------------------------------------------------------------------------------------- | ----------------- |
| --remove-node | Specifies the name of the node to be deleted as "IP: PORT".<br>The PORT value is the CLUSTER_LINK_PORT_NO value. | 192.168.0.84:5201 |

## Deployer Installation

- **Information**
  The Deployer must be installed in advance on all hosts (= servers) where the broker and warehouse are installed

### Configuration

Configure installation directory and path information.

```bash
# Edit .bashrc.
export MACHBASE_DEPLOYER_HOME=~/deployer
export MACHBASE_HOME=~/deployer
export PATH=$MACHBASE_HOME/bin:$PATH
export LD_LIBRARY_PATH=$MACHBASE_HOME/lib:$LD_LIBRARY_PATH

# Reflect changes.
source .bashrc
```

### Create and Unzip Directory

Create a dedicated directory and unzip the package archive into that directory.

```bash
# Create directory.
mkdir $MACHBASE_DEPLOYER_HOME

# Unzip.
tar zxvf machbase-ent-x.y.z.official-LINUX-X86-64-release.tgz -C $MACHBASE_DEPLOYER_HOME
```

### Port Configuration and Service Activation

Modify the machbase.conf file to set the port and start the service.

```bash
# Set port from machbase.conf file.
cd $MACHBASE_DEPLOYER_HOME/conf
vi machbase.conf
CLUSTER_LINK_HOST       = 192.168.0.84
CLUSTER_LINK_PORT_NO    = 5201
HTTP_ADMIN_PORT         = 5202

# Create meta information and run service.
$MACHBASE_DEPLOYER_HOME/bin/machdeployeradmin -c
$MACHBASE_DEPLOYER_HOME/bin/machdeployeradmin -u
```

### Node Registration and Verification

- **Caution**
  This should be done at the coordinator node.

Add and verify the Deployer node.

```bash
# Register node.
$MACHBASE_COORDINATOR_HOME/bin/machcoordinatoradmin --add-node="192.168.0.84:5201" --node-type=deployer

# Check node.
$MACHBASE_COORDINATOR_HOME/bin/machcoordinatoradmin --cluster-status
```

| Optional    | Description                                                                                             | Example           |
| ----------- | ------------------------------------------------------------------------------------------------------- | ----------------- |
| --add-node  | Specifies the node name to be added as "IP: PORT".<br>The PORT value is the CLUSTER_LINK_PORT_NO value. | 192.168.0.84:5201 |
| --node-type | Specifies the node type.<br>There are four types: coordinator / deployer / broker / warehouse.          | deployer          |

## Delete Deployer

You must delete the Deployer node from the Coordinator node and properly terminate the Deployer process on the server where the Deployer is located.

```bash
# Delete node.
$MACHBASE_COORDINATOR_HOME/bin/machcoordinatoradmin --remove-node="192.168.0.84:5201"

# Terminate deployer and delete directory.
$MACHBASE_DEPLOYER_HOME/bin/machdeployeradmin -d
rm -rf $MACHBASE_DEPLOYER_HOME

# Check node.
$MACHBASE_COORDINATOR_HOME/bin/machcoordinatoradmin --cluster-status
```

## Add Package

Add a package to be installed as broker and warehouse to Coordinator. At this time, the registered package registers the lightweight version excluding MWA.

```bash
# Register installation package.
$MACHBASE_COORDINATOR_HOME/bin/machcoordinatoradmin --add-package=machbase \
    --file-name="/home/machbase/machbase-ent-x.y.z.official-LINUX-X86-64-release-lightweight.tgz"
```

| Optional      | Description                                                                                                                                                                               | Example                                                                         |
| ------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------- |
| --add-package | Specifies the package name to be added.                                                                                                                                                   | machbase                                                                        |
| --file-name   | Specifies the full path and file name of the package file.<br>Specifies a lightweight package that excludes MWA files, since this package is for broker and warehouse installations only. | /home/machbase/machbase-ent-5.0.0.official-LINUX-X86-64-release-lightweight.tgz |

## Delete Package

Delete the package registered in Coordinator.

```bash
# Delete registered package.
$MACHBASE_COORDINATOR_HOME/bin/machcoordinatoradmin --remove-package=machbase
```
