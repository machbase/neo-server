# Cluster Edition Upgrade

## Coordinator Upgrade

Coordinator / Deployer must be upgraded manually.

#### Precautions

* You can not issue commands such as adding / starting / terminating / deleting nodes during the upgrade.
* DDL or DELETE must not be in use. (INSERT, APPEND, SELECT do not matter.)

#### Coordinator Shutdown

Coordinator / Deployer does not affect INSERT, APPEND, SELECT in Broker / Warehouse even if it is shut down. 

However, it does not detect that the Broker / Warehouse also shuts down while it is shutting down. (Normally detected after restart)

```bash
machcoordinatoradmin --shutdown
```

#### Coordinator Backup (Optional)

Backup the dbs/ and conf/ directories located in $MACH_COORDINATOR_HOME.

#### Coordinator Upgrade

* Proceed with full package instead of lightweight package.

Unzip and overwrite the package to $MACH_COORDINATOR_HOME.

```bash
tar zxvf machbase-ent-new.official-LINUX-X86-64-release.tgz -C $MACHBASE_COORDINATOR_HOME
```

#### Coordinator Startup

```bash
machcoordinatoradmin --startup
```

## Deployer Upgrade

This has the same process as the Coordinator.

#### Precautions
 
* You can not issue commands such as adding / starting / terminating / deleting nodes during the upgrade.

#### Deployer Shutdown

```bash
machdeployeradmin --shutdown
```

#### Deployer Backup (Optional)

Back up the dbs/ and conf/ directories located in $MACH_DEPLOYER_HOME.

#### Deployer Upgrade

* If you are running MWA or not running Collector on the Host the Deployer is installed, you can proceed with the lightweight package.

Unzip and overwrite the package to $MACH_DEPLOYER_HOME.

```bash
tar zxvf machbase-ent-new.official-LINUX-X86-64-release.tgz -C $MACH_DEPLOYER_HOME
```

#### Deployer Startup

```bash
machdeployeradmin --startup
```

## Package Registration

To upgrade Broker / Warehouse, register the Package in Coordinator and proceed with the upgrade.

It is recommended to register the lightweight package.

First, move the package to the Host where $MACH_COORDINATOR_HOME is located.

Next, add the package using the following command.

```bash
machcoordinatoradmin --add-package=new_package --file-name=./machbase-ent-new.official-LINUX-X86-64-release-lightweight.tgz
```

|Option|Description|
|--|--|
|--add-package|Specifies the name of the package to add.|
|--file-name|Specifies the path to the package file to add.<br>**If a package with the same filename is added, you will receive an error, so check the file name.**|

#### Broker/Warehouse Upgrade

In the Coordinator, run the following command.

## Node Shutdown

```bash
machcoordinatoradmin --shutdown-node=localhost:5656
```

## Node Upgrade

```bash
machcoordinatoradmin --upgrade-node=localhost:5656 --package-name=new_package
```

|Option|Description|
|--|--|
|--upgrade-node|Enters the name of the upgrade target Node.|
|--package-name|Enters the name of the Package to be upgraded.|

* If you upgrade the Node without shutting down the Node, it will automatically shut down the Node and perform the Node upgrade.
  However, for stability, you should explicitly shut down the Node before upgrading.

## Node Startup

```bash
machcoordinatoradmin --startup-node=localhost:5656
```

## Snapshot Failover

From Machbase 6.5 Cluster Edition, the Snapshot Failover function has been added.

Snapshot failover is a function that provides quick recovery by recording snapshots when the DBMS is in a normal condition and performing failover only for the part where the problem occurs, excluding the normal snapshot when a specific warehouse fails.

#### Snapshot basic concept

It is a concept to record the location of normal data between warehouses in the group for each group of Cluster Edition.

All data before the snapshot created in the warehouse in the group are data in a normal state, and each snapshot is recorded for each group.

#### How Snapshot Failover Works

When a problem occurs in a specific warehouse, the warehouse enters scrapped state and data recovery is required.

When performing Snapshot Recovery, data after the snapshot is cleared based on the normal snapshot in the warehouse where the problem occurred, and the data after the baseline snapshot of the warehouse in the normal state in the same group is replicated to the warehouse where the problem occurred to complete the recovery.

#### Automatic Snapshot Execution

By default, automatic snapshot execution is enabled, and the snapshot execution interval is set to 60 seconds. If there are multiple warehouse groups in a cluster, only one group performs snapshots sequentially every snapshot interval.

If the execution interval is set to 0, automatic snapshot execution is disabled.

Snapshot interval setting is reflected immediately when the command is executed.

```bash
## Snapshot Interval Setting
machcoordinatoradmin --snapshot-interval=[sec]
  
## Check the current snapshot interval
machcoordinatoradmin --configuration
```

#### Take Snapshot manually

Specify **group_name** using the machcoordinatoradmin tool and manually perform Snapshot.

**group_name** is preset like group1, group2.

If there are multiple groups in a cluster, snapshots must be performed for each group in order to take a full snapshot.

```bash
## Manually take a snapshot for group_name
machcoordinatoradmin --exec-snapshot --group='group_name'
```

#### Recover scrapped node based on Snapshot

If a scrapped node occurs, it is restored as follows.

```bash
## Change the group state to readonly
## Prevents group from being changed to normal state in later steps
machcoordinatoradmin --set-group-state=readonly --group=[groupname]
  
## Recover based on Snapshot
machcoordinatoradmin --snapshot-recover=[nodename]
  
## Replicate the latest data after snapshot through replication
## When replication is finished, the state of the warehouse is automatically changed to normal.
machcoordinatoradmin --exec-sync=[nodename]
  
## Change the group state to readonly
machcoordinatoradmin --set-group-state=normal --group=[groupname]
```

#### Snapshot-based recovery process of scrapped nodes

When recovering a scrapped node with a snapshot, the following process is performed.

```bash
/* Initial cluster state */
+-------------+-----------------+-----------------+-----------------+-------------------------------+-------------+
|  Node Type  |    Node Name    |   Group Name    |   Group State   |    Desired & Actual State     |  RP State   |
+-------------+-----------------+-----------------+-----------------+-------------------------------+-------------+
| coordinator | localhost:30110 | Coordinator     | normal          | primary       | primary       | ----------- |
| coordinator | localhost:30120 | Coordinator     | normal          | normal        | normal        | ----------- |
| deployer    | localhost:30210 | Deployer        | normal          | normal        | normal        | ----------- |
| broker      | localhost:30310 | Broker          | normal          | leader        | leader        | ----------- |
| broker      | localhost:30320 | Broker          | normal          | normal        | normal        | ----------- |
| warehouse   | localhost:30410 | group1          | normal          | normal        | normal        | ----------- |
| warehouse   | localhost:30420 | group1          | normal          | normal        | normal        | ----------- |
| warehouse   | localhost:30510 | group2          | normal          | normal        | normal        | ----------- |
| warehouse   | localhost:30520 | group2          | normal          | normal        | normal        | ----------- |
+-------------+-----------------+-----------------+-----------------+-------------------------------+-------------+
  
/* warehouse 0 of group1 dies */
+-------------+-----------------+-----------------+-----------------+-------------------------------+-------------+
|  Node Type  |    Node Name    |   Group Name    |   Group State   |    Desired & Actual State     |  RP State   |
+-------------+-----------------+-----------------+-----------------+-------------------------------+-------------+
| coordinator | localhost:30110 | Coordinator     | normal          | primary       | primary       | ----------- |
| coordinator | localhost:30120 | Coordinator     | normal          | normal        | normal        | ----------- |
| deployer    | localhost:30210 | Deployer        | normal          | normal        | normal        | ----------- |
| broker      | localhost:30310 | Broker          | normal          | leader        | leader        | ----------- |
| broker      | localhost:30320 | Broker          | normal          | normal        | normal        | ----------- |
| warehouse   | localhost:30410 | group1          | readonly        | scrapped      | **unknown**   | ----------- |
| warehouse   | localhost:30420 | group1          | readonly        | normal        | normal        | ----------- |
| warehouse   | localhost:30510 | group2          | normal          | normal        | normal        | ----------- |
| warehouse   | localhost:30520 | group2          | normal          | normal        | normal        | ----------- |
+-------------+-----------------+-----------------+-----------------+-------------------------------+-------------+
  
## Change the group state to readonly
machcoordinatoradmin --set-group-state=readonly --group=[groupname]
  
kellen@kellen-ku:~$ machcoordinatoradmin --set-group-state=readonly --group=group1
-------------------------------------------------------------------------
     Machbase Coordinator Administration Tool
     Release Version - 321a012d05.develop
     Copyright 2014, MACHBASE Corp. or its subsidiaries
     All Rights Reserved
-------------------------------------------------------------------------
Group Name: group1
Flag      : 1
+-------------+-----------------+-----------------+-----------------+-------------------------------+-------------+
|  Node Type  |    Node Name    |   Group Name    |   Group State   |    Desired & Actual State     |  RP State   |
+-------------+-----------------+-----------------+-----------------+-------------------------------+-------------+
| coordinator | localhost:30110 | Coordinator     | normal          | primary       | primary       | ----------- |
| coordinator | localhost:30120 | Coordinator     | normal          | normal        | normal        | ----------- |
| deployer    | localhost:30210 | Deployer        | normal          | normal        | normal        | ----------- |
| broker      | localhost:30310 | Broker          | normal          | leader        | leader        | ----------- |
| broker      | localhost:30320 | Broker          | normal          | normal        | normal        | ----------- |
| warehouse   | localhost:30410 | group1          | readonly        | scrapped      | **unknown**   | ----------- |
| warehouse   | localhost:30420 | group1          | readonly        | normal        | normal        | ----------- |
| warehouse   | localhost:30510 | group2          | normal          | normal        | normal        | ----------- |
| warehouse   | localhost:30520 | group2          | normal          | normal        | normal        | ----------- |
+-------------+-----------------+-----------------+-----------------+-------------------------------+-------------+
  
## Restart the dead warehouse
+-------------+-----------------+-----------------+-----------------+-------------------------------+-------------+
|  Node Type  |    Node Name    |   Group Name    |   Group State   |    Desired & Actual State     |  RP State   |
+-------------+-----------------+-----------------+-----------------+-------------------------------+-------------+
| coordinator | localhost:30110 | Coordinator     | normal          | primary       | primary       | ----------- |
| coordinator | localhost:30120 | Coordinator     | normal          | normal        | normal        | ----------- |
| deployer    | localhost:30210 | Deployer        | normal          | normal        | normal        | ----------- |
| broker      | localhost:30310 | Broker          | normal          | leader        | leader        | ----------- |
| broker      | localhost:30320 | Broker          | normal          | normal        | normal        | ----------- |
| warehouse   | localhost:30410 | group1          | readonly        | scrapped      | scrapped      | ----------- |
| warehouse   | localhost:30420 | group1          | readonly        | normal        | normal        | ----------- |
| warehouse   | localhost:30510 | group2          | normal          | normal        | normal        | ----------- |
| warehouse   | localhost:30520 | group2          | normal          | normal        | normal        | ----------- |
+-------------+-----------------+-----------------+-----------------+-------------------------------+-------------+
  
## Recovery based on snapshot
machcoordinatoradmin --snapshot-recover=[nodename]
  
kellen@kellen-ku:~$ machcoordinatoradmin --snapshot-recover=localhost:30410
-------------------------------------------------------------------------
     Machbase Coordinator Administration Tool
     Release Version - 321a012d05.develop
     Copyright 2014, MACHBASE Corp. or its subsidiaries
     All Rights Reserved
-------------------------------------------------------------------------
Node-Name: localhost:30410
+-------------+-----------------+-----------------+-----------------+-------------------------------+-------------+
|  Node Type  |    Node Name    |   Group Name    |   Group State   |    Desired & Actual State     |  RP State   |
+-------------+-----------------+-----------------+-----------------+-------------------------------+-------------+
| coordinator | localhost:30110 | Coordinator     | normal          | primary       | primary       | ----------- |
| coordinator | localhost:30120 | Coordinator     | normal          | normal        | normal        | ----------- |
| deployer    | localhost:30210 | Deployer        | normal          | normal        | normal        | ----------- |
| broker      | localhost:30310 | Broker          | normal          | leader        | leader        | ----------- |
| broker      | localhost:30320 | Broker          | normal          | normal        | normal        | ----------- |
| warehouse   | localhost:30410 | group1          | readonly        | scrapped      | scrapped      | ----------- |
| warehouse   | localhost:30420 | group1          | readonly        | normal        | normal        | ----------- |
| warehouse   | localhost:30510 | group2          | normal          | normal        | normal        | ----------- |
| warehouse   | localhost:30520 | group2          | normal          | normal        | normal        | ----------- |
+-------------+-----------------+-----------------+-----------------+-------------------------------+-------------+
  
## Replicate the latest data after snapshot through replication
machcoordinatoradmin --exec-sync=[nodename]
  
kellen@kellen-ku:~$ machcoordinatoradmin --exec-sync=localhost:30410
-------------------------------------------------------------------------
     Machbase Coordinator Administration Tool
     Release Version - 321a012d05.develop
     Copyright 2014, MACHBASE Corp. or its subsidiaries
     All Rights Reserved
-------------------------------------------------------------------------
Node-Name: localhost:30410
Source:
+-------------+-----------------+-----------------+-----------------+-------------------------------+-------------+
|  Node Type  |    Node Name    |   Group Name    |   Group State   |    Desired & Actual State     |  RP State   |
+-------------+-----------------+-----------------+-----------------+-------------------------------+-------------+
| coordinator | localhost:30110 | Coordinator     | normal          | primary       | primary       | ----------- |
| coordinator | localhost:30120 | Coordinator     | normal          | normal        | normal        | ----------- |
| deployer    | localhost:30210 | Deployer        | normal          | normal        | normal        | ----------- |
| broker      | localhost:30310 | Broker          | normal          | leader        | leader        | ----------- |
| broker      | localhost:30320 | Broker          | normal          | normal        | normal        | ----------- |
| warehouse   | localhost:30410 | group1          | readonly        | scrapped      | scrapped      | stopped     |
| warehouse   | localhost:30420 | group1          | readonly        | normal        | normal        | stopped     |
| warehouse   | localhost:30510 | group2          | normal          | normal        | normal        | ----------- |
| warehouse   | localhost:30520 | group2          | normal          | normal        | normal        | ----------- |
+-------------+-----------------+-----------------+-----------------+-------------------------------+-------------+
+-------------+-----------------+-----------------+-----------------+-------------------------------+-------------+
|  Node Type  |    Node Name    |   Group Name    |   Group State   |    Desired & Actual State     |  RP State   |
+-------------+-----------------+-----------------+-----------------+-------------------------------+-------------+
| coordinator | localhost:30110 | Coordinator     | normal          | primary       | primary       | ----------- |
| coordinator | localhost:30120 | Coordinator     | normal          | normal        | normal        | ----------- |
| deployer    | localhost:30210 | Deployer        | normal          | normal        | normal        | ----------- |
| broker      | localhost:30310 | Broker          | normal          | leader        | leader        | ----------- |
| broker      | localhost:30320 | Broker          | normal          | normal        | normal        | ----------- |
| warehouse   | localhost:30410 | group1          | readonly        | sync-standby  | sync-standby  | running     |
| warehouse   | localhost:30420 | group1          | readonly        | sync-active   | sync-active   | running     |
| warehouse   | localhost:30510 | group2          | normal          | normal        | normal        | ----------- |
| warehouse   | localhost:30520 | group2          | normal          | normal        | normal        | ----------- |
+-------------+-----------------+-----------------+-----------------+-------------------------------+-------------+
+-------------+-----------------+-----------------+-----------------+-------------------------------+-------------+
|  Node Type  |    Node Name    |   Group Name    |   Group State   |    Desired & Actual State     |  RP State   |
+-------------+-----------------+-----------------+-----------------+-------------------------------+-------------+
| coordinator | localhost:30110 | Coordinator     | normal          | primary       | primary       | ----------- |
| coordinator | localhost:30120 | Coordinator     | normal          | normal        | normal        | ----------- |
| deployer    | localhost:30210 | Deployer        | normal          | normal        | normal        | ----------- |
| broker      | localhost:30310 | Broker          | normal          | leader        | leader        | ----------- |
| broker      | localhost:30320 | Broker          | normal          | normal        | normal        | ----------- |
| warehouse   | localhost:30410 | group1          | readonly        | normal        | normal        | stopped     |
| warehouse   | localhost:30420 | group1          | readonly        | normal        | normal        | stopped     |
| warehouse   | localhost:30510 | group2          | normal          | normal        | normal        | ----------- |
| warehouse   | localhost:30520 | group2          | normal          | normal        | normal        | ----------- |
+-------------+-----------------+-----------------+-----------------+-------------------------------+-------------+
  
## Change the group state to readonly
machcoordinatoradmin --set-group-state=normal --group=[groupname]
  
kellen@kellen-ku:~$ machcoordinatoradmin --set-group-state=normal --group=group1
-------------------------------------------------------------------------
     Machbase Coordinator Administration Tool
     Release Version - 321a012d05.develop
     Copyright 2014, MACHBASE Corp. or its subsidiaries
     All Rights Reserved
-------------------------------------------------------------------------
Group Name: group1
Flag      : 0
+-------------+-----------------+-----------------+-----------------+-------------------------------+-------------+
|  Node Type  |    Node Name    |   Group Name    |   Group State   |    Desired & Actual State     |  RP State   |
+-------------+-----------------+-----------------+-----------------+-------------------------------+-------------+
| coordinator | localhost:30110 | Coordinator     | normal          | primary       | primary       | ----------- |
| coordinator | localhost:30120 | Coordinator     | normal          | normal        | normal        | ----------- |
| deployer    | localhost:30210 | Deployer        | normal          | normal        | normal        | ----------- |
| broker      | localhost:30310 | Broker          | normal          | leader        | leader        | ----------- |
| broker      | localhost:30320 | Broker          | normal          | normal        | normal        | ----------- |
| warehouse   | localhost:30410 | group1          | normal          | normal        | normal        | stopped     |
| warehouse   | localhost:30420 | group1          | normal          | normal        | normal        | stopped     |
| warehouse   | localhost:30510 | group2          | normal          | normal        | normal        | ----------- |
| warehouse   | localhost:30520 | group2          | normal          | normal        | normal        | ----------- |
+-------------+-----------------+-----------------+-----------------+-------------------------------+-------------+
```

#### Snapshot related properties

|Property|Description|Applies to|
|--|--|--|
|GROUP_SNAPSHOT_TIMEOUT_SEC|Determines the timeout time when executing Snapshot<br>Default : 60 (sec)<br>Minimum : 0 (wait infinitely)<br>Maximum : uint32_max (sec)|Write in each node's machbase.conf file|
