# Property (Cluster)

Separate from [Property](../property), Property (Cluster) organizes the Property only available in Cluster Edition.

# Index

- [Index](#index)
  - [CLUSTER_LINK_ACCEPT_TIMEOUT](#cluster_link_accept_timeout)
  - [CLUSTER_LINK_BUFFER_SIZE](#cluster_link_buffer_size)
  - [CLUSTER_LINK_CHECK_INTERVAL](#cluster_link_check_interval)
  - [CLUSTER_LINK_CONNECT_RETRY_TIMEOUT](#cluster_link_connect_retry_timeout)
  - [CLUSTER_LINK_CONNECT_TIMEOUT](#cluster_link_connect_timeout)
  - [CLUSTER_LINK_ERROR_ADD_ORIGIN_HOST](#cluster_link_error_add_origin_host)
  - [CLUSTER_LINK_HANDSHAKE_TIMEOUT](#cluster_link_handshake_timeout)
  - [CLUSTER_LINK_SEND_RETRY_COUNT](#cluster_link_send_retry_count)
  - [CLUSTER_LINK_HOST](#cluster_link_host)
  - [CLUSTER_LINK_LONG_TERM_CALLBACK_INTERVAL](#cluster_link_long_term_callback_interval)
  - [CLUSTER_LINK_LONG_WAIT_INTERVAL](#cluster_link_long_wait_interval)
  - [CLUSTER_LINK_MAX_LISTEN](#cluster_link_max_listen)
  - [CLUSTER_LINK_MAX_POLL](#cluster_link_max_poll)
  - [CLUSTER_LINK_PORT_NO](#cluster_link_port_no)
  - [CLUSTER_LINK_RECEIVE_TIMEOUT](#cluster_link_receive_timeout)
  - [CLUSTER_LINK_REQUEST_TIMEOUT](#cluster_link_request_timeout)
  - [CLUSTER_LINK_SEND_TIMEOUT](#cluster_link_send_timeout)
  - [CLUSTER_LINK_SESSION_TIMEOUT](#cluster_link_session_timeout)
  - [CLUSTER_LINK_THREAD_COUNT](#cluster_link_thread_count)
  - [CLUSTER_QUERY_STAT_LOG_ENABLE](#cluster_query_stat_log_enable)
  - [CLUSTER_REPLICATION_BLOCK_SIZE](#cluster_replication_block_size)
  - [CLUSTER_WAREHOUSE_DIRECT_DML_ENABLE](#cluster_warehouse_direct_dml_enable)
  - [COORDINATOR_DBS_PATH](#coordinator_dbs_path)
  - [COORDINATOR_DDL_REQUEST_TIMEOUT](#coordinator_ddl_request_timeout)
  - [COORDINATOR_DDL_TIMEOUT](#coordinator_ddl_timeout)
  - [COORDINATOR_DECISION_DELAY](#coordinator_decision_delay)
  - [COORDINATOR_DECISION_INTERVAL](#coordinator_decision_interval)
  - [COORDINATOR_HOST_RESOURCE_ENABLE](#coordinator_host_resource_enable)
  - [COORDINATOR_HOST_RESOURCE_COLLECT_INTERVAL](#coordinator_host_resource_collect_interval)
  - [COORDINATOR_HOST_RESOURCE_INTERVAL](#coordinator_host_resource_interval)
  - [COORDINATOR_HOST_RESOURCE_REQUEST_TIMEOUT](#coordinator_host_resource_request_timeout)
  - [COORDINATOR_NODE_REQUEST_TIMEOUT](#coordinator_node_request_timeout)
  - [COORDINATOR_NODE_TIMEOUT](#coordinator_node_timeout)
  - [COORDINATOR_STARTUP_DELAY](#coordinator_startup_delay)
  - [COORDINATOR_STATUS_NODE_INTERVAL](#coordinator_status_node_interval)
  - [COORDINATOR_STATUS_NODE_REQUEST_TIMEOUT](#coordinator_status_node_request_timeout)
  - [COORDINATOR_DISK_FULL_UPPER_BOUND_RATIO](#coordinator_disk_full_upper_bound_ratio)
  - [COORDINATOR_DISK_FULL_LOWER_BOUND_RATIO](#coordinator_disk_full_lower_bound_ratio)
  - [DEPLOYER_DBS_PATH](#deployer_dbs_path)
  - [EXECUTION_STAGE_MEMORY_MAX](#execution_stage_memory_max)
  - [HTTP_ADMIN_PORT](#http_admin_port)
  - [HTTP_CONNECT_TIMEOUT](#http_connect_timeout)
  - [HTTP_RECEIVE_TIMEOUT](#http_receive_timeout)
  - [HTTP_SEND_TIMEOUT](#http_send_timeout)
  - [INSERT_BULK_DATA_MAX_SIZE](#insert_bulk_data_max_size)
  - [INSERT_RECORD_COUNT_PER_NODE](#insert_record_count_per_node)
  - [LOOKUPNODE_COMMAND_RETRY_MAX_COUNT](#lookupnode_command_retry_max_count)
  - [STAGE_RESULT_BLOCK_SIZE](#stage_result_block_size)

## CLUSTER_LINK_ACCEPT_TIMEOUT
Timeout until receiving Handshake message after Accept when connecting to a specific Node.

Failure to receive within the timeout will cause the connection to fail. 

The default value is 5 seconds.

<table>
  <thead>
    <th style="background-color: lightyellow;">(usec)</th>
    <th>Value</th>
  </thead>
  <tbody>
    <tr>
      <td>Minimum</td>
      <td>0</td>
    </tr>
    <tr>
      <td>Maximum</td>
      <td>2^64-1</td>
    </tr>
    <tr>
      <td style="background-color: #F0FFFF;">Default</td>
      <td>5000000</td>
    </tr>
  </tbody>
</table>

## CLUSTER_LINK_BUFFER_SIZE

The size of the request/receive buffer.

If this size is insufficient, it will try again until the buffer is empty during transmission.

|(byte)|    Value|
|------|---------|
|Minimum|    1024768|
|Maximum|    2^32 - 1|
|Default|    33554432 (32M)|

## CLUSTER_LINK_CHECK_INTERVAL
Check interval of the Timeout Thread that checks the Sockets connected to a specific Node.

There is a Timeout Thread that checks RECEIVE_TIMEOUT and SESSION_TIMEOUT. 

The shorter the cycle is, the more frequently it is checked but the Timeout determination is made according to the following values.

The default value is 1 second.

<table>
  <thead>
    <th style="background-color: lightyellow;">(usec)</th>
    <th>Value</th>
  </thead>
  <tbody>
    <tr>
      <td>Minimum</td>
      <td>0</td>
    </tr>
    <tr>
      <td>Maximum</td>
      <td>2^64-1</td>
    </tr>
    <tr>
      <td style="background-color: #F0FFFF;">Default</td>
      <td>1000000</td>
    </tr>
  </tbody>
</table>

## CLUSTER_LINK_CONNECT_RETRY_TIMEOUT
Timeout to repeat reconnect attempt after connection failure with a specific Node.

If it is not connected within the timeout, it is determined to be completely disconnected.

The default value is 1 minute.

<table>
  <thead>
    <th style="background-color: lightyellow;">(usec)</th>
    <th>Value</th>
  </thead>
  <tbody>
    <tr>
      <td>Minimum</td>
      <td>0</td>
    </tr>
    <tr>
      <td>Maximum</td>
      <td>2^64-1</td>
    </tr>
    <tr>
      <td style="background-color: #F0FFFF;">Default</td>
      <td>60000000</td>
    </tr>
  </tbody>
</table>

## CLUSTER_LINK_CONNECT_TIMEOUT
Time to wait when trying to connect to a specific Node.

If it does not connect within the Timeout, it will try to reconnect until CLUSTER_LINK_CONNECT_RETRY_TIMEOUT has passed.

The default value is 5 seconds.

<table>
  <thead>
    <th style="background-color: lightyellow;">(usec)</th>
    <th>Value</th>
  </thead>
  <tbody>
    <tr>
      <td>Minimum</td>
      <td>0</td>
    </tr>
    <tr>
      <td>Maximum</td>
      <td>2^64-1</td>
    </tr>
    <tr>
      <td style="background-color: #F0FFFF;">Default</td>
      <td>5000000</td>
    </tr>
  </tbody>
</table>

## CLUSTER_LINK_ERROR_ADD_ORIGIN_HOST
You can choose whether to add an errored host name to error messages that occur during communication between the Cluster.

If you want to display a detailed error message, set the property to 1. 

The default value is 0, which means the host name is not displayed.

<table>
  <thead>
    <th style="background-color: lightyellow;">(boolean)</th>
    <th>Value</th>
  </thead>
  <tbody>
    <tr>
      <td>Minimum</td>
      <td>0</td>
    </tr>
    <tr>
      <td>Maximum</td>
      <td>1</td>
    </tr>
    <tr>
      <td style="background-color: #F0FFFF;">Default</td>
      <td>0</td>
    </tr>
  </tbody>
</table>

## CLUSTER_LINK_HANDSHAKE_TIMEOUT
Timeout until receiving a Handshake message while connected to a specific Node and Cluster Socket.

Two Nodes that have just finished connecting exchange small size Handshake messages to check the connection status. 

The Accept Node sends the Handshake message first, and the time to wait for the response is set here.

The default value is 5 seconds.

<table>
  <thead>
    <th style="background-color: lightyellow;">(usec)</th>
    <th>Value</th>
  </thead>
  <tbody>
    <tr>
      <td>Minimum</td>
      <td>0</td>
    </tr>
    <tr>
      <td>Maximum</td>
      <td>2^64-1</td>
    </tr>
    <tr>
      <td style="background-color: #F0FFFF;">Default</td>
      <td>5000000</td>
    </tr>
  </tbody>
</table>

## CLUSTER_LINK_SEND_RETRY_COUNT
Number of times to retry sending until the send buffer is empty.

Every retry will take 1ms off. If you retry beyond this number, you will be disconnected.

The default value is 5000 (msec).

<table>
  <thead>
    <th style="background-color: lightyellow;">(count)</th>
    <th>Value</th>
  </thead>
  <tbody>
    <tr>
      <td>Minimum</td>
      <td>0</td>
    </tr>
    <tr>
      <td>Maximum</td>
      <td>2^32-1</td>
    </tr>
    <tr>
      <td style="background-color: #F0FFFF;">Default</td>
      <td>5000</td>
    </tr>
  </tbody>
</table>

## CLUSTER_LINK_HOST

Host name of the current Node to connect to a specific Node and Cluster Socket

|(string)|  Value|
|--|--|
|Default|    localhost|

## CLUSTER_LINK_LONG_TERM_CALLBACK_INTERVAL
If the execution time of Receive Callback to process a message received on Cluster Socket exceeds the set value, it is recognized as Long-Term Callback.

Since the number of receive Threads is limited, Receive Callback should not process messages for a long time. 

If Receive Callback processes the message after this time, it recognizes it as Long-Term Callback and records it in Trace Log.

The default value is 1 second.

<table>
  <thead>
    <th style="background-color: lightyellow;">(usec)</th>
    <th>Value</th>
  </thead>
  <tbody>
    <tr>
      <td>Minimum</td>
      <td>0</td>
    </tr>
    <tr>
      <td>Maximum</td>
      <td>2^64-1</td>
    </tr>
    <tr>
      <td style="background-color: #F0FFFF;">Default</td>
      <td>1000000</td>
    </tr>
  </tbody>
</table>

## CLUSTER_LINK_LONG_WAIT_INTERVAL
If the time until the arrival of a message received on Cluster Socket exceeds the set value, it is recognized as Long-Wait Message.

If the time from receiving start to receiving end is long, it can be regarded as a problem of the network environment. 

If the received message does not arrive after this time, it is recognized as a Long-Wait Message and recorded in the Trace Log.

The default value is 1 second.

<table>
  <thead>
    <th style="background-color: lightyellow;">(usec)</th>
    <th>Value</th>
  </thead>
  <tbody>
    <tr>
      <td>Minimum</td>
      <td>0</td>
    </tr>
    <tr>
      <td>Maximum</td>
      <td>2^64-1</td>
    </tr>
    <tr>
      <td style="background-color: #F0FFFF;">Default</td>
      <td>1000000</td>
    </tr>
  </tbody>
</table>

## CLUSTER_LINK_MAX_LISTEN
The maximum number of Socket's Accept Queue when connecting to a specific Node.

<table>
  <thead>
    <th style="background-color: lightyellow;">(count)</th>
    <th>Value</th>
  </thead>
  <tbody>
    <tr>
      <td>Minimum</td>
      <td>0</td>
    </tr>
    <tr>
      <td>Maximum</td>
      <td>2^32-1</td>
    </tr>
    <tr>
      <td style="background-color: #F0FFFF;">Default</td>
      <td>512</td>
    </tr>
  </tbody>
</table>

## CLUSTER_LINK_MAX_POLL
The maximum number of Events that can be retrieved at a time by Poll when communicating with a specific node.

<table>
  <thead>
    <th style="background-color: lightyellow;">(count)</th>
    <th>Value</th>
  </thead>
  <tbody>
    <tr>
      <td>Minimum</td>
      <td>1</td>
    </tr>
    <tr>
      <td>Maximum</td>
      <td>2^32-1</td>
    </tr>
    <tr>
      <td style="background-color: #F0FFFF;">Default</td>
      <td>4096</td>
    </tr>
  </tbody>
</table>

## CLUSTER_LINK_PORT_NO
The port number of the current Node for connecting the specific Node to the Cluster Socket

<table>
  <thead>
    <th style="background-color: lightyellow;">(port)</th>
    <th>Value</th>
  </thead>
  <tbody>
    <tr>
      <td>Minimum</td>
      <td>1024</td>
    </tr>
    <tr>
      <td>Maximum</td>
      <td>65535</td>
    </tr>
    <tr>
      <td style="background-color: #F0FFFF;">Default</td>
      <td>3868</td>
    </tr>
  </tbody>
</table>

## CLUSTER_LINK_RECEIVE_TIMEOUT
Timeout until the Timeout Thread determines that the connection has been disconnected since the last reception.

Connections that exist in the 'Linked List' should be continuously receiving because the connection between Cluster Nodes is terminated when the reception is complete.

If the last received time is not updated after the set time has elapsed, the Timeout Thread records its contents in the Trace Log and closes the Socket.

<table>
  <thead>
    <th style="background-color: lightyellow;">(usec)</th>
    <th>Value</th>
  </thead>
  <tbody>
    <tr>
      <td>Minimum</td>
      <td>0</td>
    </tr>
    <tr>
      <td>Maximum</td>
      <td>2^64-1</td>
    </tr>
    <tr>
      <td style="background-color: #F0FFFF;">Default</td>
      <td>30000000</td>
    </tr>
  </tbody>
</table>

## CLUSTER_LINK_REQUEST_TIMEOUT
Timeout from when a request message is sent from the Cluster Socket to when a response to the request is received.

For specific messages, specify the time to wait for a response after the request.

If the response message does not arrive at this time, write log to the Trace Log and close the Socket.

The default value is 60 seconds, Timeout is long enough because it is not known what kind of message and receive processing will happen.

<table>
  <thead>
    <th style="background-color: lightyellow;">(usec)</th>
    <th>Value</th>
  </thead>
  <tbody>
    <tr>
      <td>Minimum</td>
      <td>0</td>
    </tr>
    <tr>
      <td>Maximum</td>
      <td>2^64-1</td>
    </tr>
    <tr>
      <td style="background-color: #F0FFFF;">Default</td>
      <td>60000000</td>
    </tr>
  </tbody>
</table>

## CLUSTER_LINK_SEND_TIMEOUT
Timeout to set when sending messages through Cluster Socket.

Set the corresponding timeout when transmitting.

If transmission is not completed until Timeout, it is recorded in the Trace Log.

<table>
  <thead>
    <th style="background-color: lightyellow;">(usec)</th>
    <th>Value</th>
  </thead>
  <tbody>
    <tr>
      <td>Minimum</td>
      <td>0</td>
    </tr>
    <tr>
      <td>Maximum</td>
      <td>2^64-1</td>
    </tr>
    <tr>
      <td style="background-color: #F0FFFF;">Default</td>
      <td>30000000</td>
    </tr>
  </tbody>
</table>

## CLUSTER_LINK_SESSION_TIMEOUT
Timeout until the Timeout thread determines that the connection has been disconnected since the last receive in a specific session.

Cluster connection manages the session of all messages internally, which is a necessary property in case the session can suddenly not be fixed. 

If the last receive time for the session is not updated after this time, the Timeout Thread writes to the Trace Log and closes the session.

The default value is 1 hour.

<table>
  <thead>
    <th style="background-color: lightyellow;">(usec)</th>
    <th>Value</th>
  </thead>
  <tbody>
    <tr>
      <td>Minimum</td>
      <td>0</td>
    </tr>
    <tr>
      <td>Maximum</td>
      <td>2^64-1</td>
    </tr>
    <tr>
      <td style="background-color: #F0FFFF;">Default</td>
      <td>3600000000</td>
    </tr>
  </tbody>
</table>

## CLUSTER_LINK_THREAD_COUNT
The number of Threads to process the received messages when communicating with a specific Node.

If the size of the Cluster grows or the number of operations to be processed increases, you can increase the number of receive threads.

<table>
  <thead>
    <th style="background-color: lightyellow;">(count)</th>
    <th>Value</th>
  </thead>
  <tbody>
    <tr>
      <td>Minimum</td>
      <td>1</td>
    </tr>
    <tr>
      <td>Maximum</td>
      <td>4096</td>
    </tr>
    <tr>
      <td style="background-color: #F0FFFF;">Default</td>
      <td>8</td>
    </tr>
  </tbody>
</table>

## CLUSTER_QUERY_STAT_LOG_ENABLE
Outputs statistical information about the executed query to the trace log.

<table>
  <thead>
    <th style="background-color: lightyellow;">(boolean)</th>
    <th>Value</th>
  </thead>
  <tbody>
    <tr>
      <td>Minimum</td>
      <td>0</td>
    </tr>
    <tr>
      <td>Maximum</td>
      <td>1</td>
    </tr>
    <tr>
      <td style="background-color: #F0FFFF;">Default</td>
      <td>0</td>
    </tr>
  </tbody>
</table>

## CLUSTER_REPLICATION_BLOCK_SIZE
The size of the data to be sent at once when the Replication for adding Node is performed in the Cluster Edition.

The Property must be applied directly to the warehouse (=Transmitting Warehouse) that becomes the Replication Active.

The default value is 640 KB.

<table>
  <thead>
    <th style="background-color: lightyellow;">(size)</th>
    <th>Value</th>
  </thead>
  <tbody>
    <tr>
      <td>Minimum</td>
      <td>64 * 1024</td>
    </tr>
    <tr>
      <td>Maximum</td>
      <td>100 * 1024 * 1024</td>
    </tr>
    <tr>
      <td style="background-color: #F0FFFF;">Default</td>
      <td>640 * 1024 (655360)</td>
    </tr>
  </tbody>
</table>

## CLUSTER_WAREHOUSE_DIRECT_DML_ENABLE
It is made possible to connect directly to the Warehouse to perform DML in Cluster Edition.

* 1: Executable
* 2: Not executable. An error is returned.

When directly performing the DML in Warehouse, there are performance advantages over Brokers but there is an issue where the DML is not propagated to the same Group. 

Therefore, it is used only for emergency recovery due to data discrepancies, or if the data discrepancies of the Group can be taken into account.

You must apply Properties directly to the specific Warehouse you want.

The default value is 0.

The Coordinator does not check for data discrepancies, even if there is a data difference between the Warehouses in the Group with the corresponding Property turned on.

<table>
  <thead>
    <th style="background-color: lightyellow;">(boolean)</th>
    <th>Value</th>
  </thead>
  <tbody>
    <tr>
      <td>Minimum</td>
      <td>0</td>
    </tr>
    <tr>
      <td>Maximum</td>
      <td>1</td>
    </tr>
    <tr>
      <td style="background-color: #F0FFFF;">Default</td>
      <td>0</td>
    </tr>
  </tbody>
</table>

## COORDINATOR_DBS_PATH
Specifies the directory where the Coordinator data file will be created.

The default value is set to ?/dbs, and ? is replaced with the $ MACHBASE_COORDINATOR_HOME environment variable. 

This is an environment variable $MACHBASE_COORDINATOR_HOME/dbs directory.

It must be applied to the Coordinator, and it has no effect on other Nodes.

<table>
  <thead>
    <th style="background-color: lightyellow;">(path)</th>
    <th>Value</th>
  </thead>
  <tbody>
    <tr>
      <td style="background-color: #F0FFFF;">Default</td>
      <td>?/dbs</td>
    </tr>
  </tbody>
</table>

## COORDINATOR_DDL_REQUEST_TIMEOUT
Timeout until the Coordinator waits after requesting the Node to execute DDL.

This value refers to the time the Coordinator waits after requesting each Node to perform DDL.

<table>
  <thead>
    <th style="background-color: lightyellow;">(usec)</th>
    <th>Value</th>
  </thead>
  <tbody>
    <tr>
      <td>Minimum</td>
      <td>0</td>
    </tr>
    <tr>
      <td>Maximum</td>
      <td>2^64 - 1</td>
    </tr>
    <tr>
      <td style="background-color: #F0FFFF;">Default</td>
      <td>3600000000</td>
    </tr>
  </tbody>
</table>

## COORDINATOR_DDL_TIMEOUT

Timeout until the broker waits after requesting the coordinator to perform DDL.  

This value means the time it takes to wait after the broker requests the coordinator to perform DDL for the entire cluster nodes.

|(usec)|Value|
|--|--|
|Minimum|0|  
|Maximum|2^64 - 1|
|Default|3600000000|

## COORDINATOR_DECISION_DELAY
Timeout until the Coordinator requests the status change and effectively reflects it.

If the status does not actually change over this time, disable the cluster status. 

If the status of the Warehouse Active is not changed but the connected Standby exists, the Fail-Over operation starts.

<table>
  <thead>
    <th style="background-color: lightyellow;">(usec)</th>
    <th>Value</th>
  </thead>
  <tbody>
    <tr>
      <td>Minimum</td>
      <td>0</td>
    </tr>
    <tr>
      <td>Maximum</td>
      <td>2^64 - 1</td>
    </tr>
    <tr>
      <td style="background-color: #F0FFFF;">Default</td>
      <td>1000000</td>
    </tr>
  </tbody>
</table>

## COORDINATOR_DECISION_INTERVAL
Time to determine how often the Coordinator changes status.

<table>
  <thead>
    <th style="background-color: lightyellow;">(usec)</th>
    <th>Value</th>
  </thead>
  <tbody>
    <tr>
      <td>Minimum</td>
      <td>0</td>
    </tr>
    <tr>
      <td>Maximum</td>
      <td>2^64 - 1</td>
    </tr>
    <tr>
      <td style="background-color: #F0FFFF;">Default</td>
      <td>1000000</td>
    </tr>
  </tbody>
</table>

## COORDINATOR_HOST_RESOURCE_ENABLE
Whether the Coordinator collects Host Resources for Cluster Nodes.

<table>
  <thead>
    <th style="background-color: lightyellow;">(boolean)</th>
    <th>Value</th>
  </thead>
  <tbody>
    <tr>
      <td>Minimum</td>
      <td>0 (false)</td>
    </tr>
    <tr>
      <td>Maximum</td>
      <td>1 (true)</td>
    </tr>
    <tr>
      <td style="background-color: #F0FFFF;">Default</td>
      <td>0 (false)</td>
    </tr>
  </tbody>
</table>

## COORDINATOR_HOST_RESOURCE_COLLECT_INTERVAL
Interval at which Cluster Nodes collect Host Resources.

<table>
  <thead>
    <th style="background-color: lightyellow;">(usec)</th>
    <th>Value</th>
  </thead>
  <tbody>
    <tr>
      <td>Minimum</td>
      <td>0</td>
    </tr>
    <tr>
      <td>Maximum</td>
      <td>2^64 - 1</td>
    </tr>
    <tr>
      <td style="background-color: #F0FFFF;">Default</td>
      <td>1000000</td>
    </tr>
  </tbody>
</table>

## COORDINATOR_HOST_RESOURCE_INTERVAL
Interval at which the Coordinator exchanges Host Resources with Nodes.

<table>
  <thead>
    <th style="background-color: lightyellow;">(usec)</th>
    <th>Value</th>
  </thead>
  <tbody>
    <tr>
      <td>Minimum</td>
      <td>0</td>
    </tr>
    <tr>
      <td>Maximum</td>
      <td>2^64 - 1</td>
    </tr>
    <tr>
      <td style="background-color: #F0FFFF;">Default</td>
      <td>1000000</td>
    </tr>
  </tbody>
</table>

## COORDINATOR_HOST_RESOURCE_REQUEST_TIMEOUT
Time that the Coordinator waits after requesting the Host Resource information from the Nodes.

<table>
  <thead>
    <th style="background-color: lightyellow;">(usec)</th>
    <th>Value</th>
  </thead>
  <tbody>
    <tr>
      <td>Minimum</td>
      <td>0</td>
    </tr>
    <tr>
      <td>Maximum</td>
      <td>2^64 - 1</td>
    </tr>
    <tr>
      <td style="background-color: #F0FFFF;">Default</td>
      <td>10000000</td>
    </tr>
  </tbody>
</table>

## COORDINATOR_NODE_REQUEST_TIMEOUT
Timeout until the Coordinator waits after requesting the Node to execute the command.

Because the Add/Remove-node and Add/Remove-Package includes the Node command execution, if it is caught in a short time, the command processing may not be completed.

<table>
  <thead>
    <th style="background-color: lightyellow;">(usec)</th>
    <th>Value</th>
  </thead>
  <tbody>
    <tr>
      <td>Minimum</td>
      <td>0</td>
    </tr>
    <tr>
      <td>Maximum</td>
      <td>2^64 - 1</td>
    </tr>
    <tr>
      <td style="background-color: #F0FFFF;">Default</td>
      <td>600000000</td>
    </tr>
  </tbody>
</table>

## COORDINATOR_NODE_TIMEOUT
Time the Coordinator waits before determining that the Node has failed.

<table>
  <thead>
    <th style="background-color: lightyellow;">(usec)</th>
    <th>Value</th>
  </thead>
  <tbody>
    <tr>
      <td>Minimum</td>
      <td>0</td>
    </tr>
    <tr>
      <td>Maximum</td>
      <td>2^64 - 1</td>
    </tr>
    <tr>
      <td style="background-color: #F0FFFF;">Default</td>
      <td>30000000</td>
    </tr>
  </tbody>
</table>

## COORDINATOR_STARTUP_DELAY
Grace time until activating the Decision Thread immediately after Coordinator startup.

If it takes a long time to run the entire Cluster, you can start the Node control of Coordinator later by setting a larger value. 

If the Decision Thread runs before the entire drive, there is a high likelihood that the Coordinator will be misplaced.

<table>
  <thead>
    <th style="background-color: lightyellow;">(usec)</th>
    <th>Value</th>
  </thead>
  <tbody>
    <tr>
      <td>Minimum</td>
      <td>0</td>
    </tr>
    <tr>
      <td>Maximum</td>
      <td>2^64 - 1</td>
    </tr>
    <tr>
      <td style="background-color: #F0FFFF;">Default</td>
      <td>3000000</td>
    </tr>
  </tbody>
</table>

## COORDINATOR_STATUS_NODE_INTERVAL
Interval in which the Coordinator exchanges status inquiry messages with the Nodes.

<table>
  <thead>
    <th style="background-color: lightyellow;">(usec)</th>
    <th>Value</th>
  </thead>
  <tbody>
    <tr>
      <td>Minimum</td>
      <td>0</td>
    </tr>
    <tr>
      <td>Maximum</td>
      <td>2^64 - 1</td>
    </tr>
    <tr>
      <td style="background-color: #F0FFFF;">Default</td>
      <td>1000000</td>
    </tr>
  </tbody>
</table>

## COORDINATOR_STATUS_NODE_REQUEST_TIMEOUT
Time the Coordinator waits after requesting status inquiries from Nodes.

If there is no status inquiry response during that time, the Coordinator proceeds without updating the status of the corresponding Node. 

If the network situation is not good and you need to update the state, you could consider increasing the value. 

Instead, if there is no status query response, the Coordinator will wait for as much as the value was increased.

<table>
  <thead>
    <th style="background-color: lightyellow;">(usec)</th>
    <th>Value</th>
  </thead>
  <tbody>
    <tr>
      <td>Minimum</td>
      <td>0</td>
    </tr>
    <tr>
      <td>Maximum</td>
      <td>2^64 - 1</td>
    </tr>
    <tr>
      <td style="background-color: #F0FFFF;">Default</td>
      <td>15000000</td>
    </tr>
  </tbody>
</table>

## COORDINATOR_DISK_FULL_UPPER_BOUND_RATIO
If the disk usage of some servers configured in the cluster exceeds the property value, the group to which the warehouse belongs will enter the DISKFULL state.

Input is restricted for the group in the DISKFULL state, and only inquiry and deletion are possible.

If the property value is 0, the function is disabled.

<table>
  <thead>
    <th style="background-color: lightyellow;">(percent)</th>
    <th>Value</th>
  </thead>
  <tbody>
    <tr>
      <td>Minimum</td>
      <td>0</td>
    </tr>
    <tr>
      <td>Maximum</td>
      <td>99</td>
    </tr>
    <tr>
      <td style="background-color: #F0FFFF;">Default</td>
      <td>0</td>
    </tr>
  </tbody>
</table>

## COORDINATOR_DISK_FULL_LOWER_BOUND_RATIO
If the disk usage of the server operating in the DISKFULL state falls below the property value, the group state transitions to the normal.

If the property value is 0, the function is disabled.

<table>
  <thead>
    <th style="background-color: lightyellow;">(percent)</th>
    <th>Value</th>
  </thead>
  <tbody>
    <tr>
      <td>Minimum</td>
      <td>0</td>
    </tr>
    <tr>
      <td>Maximum</td>
      <td>99</td>
    </tr>
    <tr>
      <td style="background-color: #F0FFFF;">Default</td>
      <td>0</td>
    </tr>
  </tbody>
</table>

## DEPLOYER_DBS_PATH
Specifies the directory where the Deployer's data files will be created.

The default value is set to?/dbs, and ? is replaced with the $ MACHBASE_DEPLOYER_HOME environment variable. 

This is an environment variable $MACHBASE_DEPLOYER_HOME /dbs directory.

It must be applied to Deployer, and it has no effect on other Nodes.

<table>
  <thead>
    <th style="background-color: lightyellow;">(path)</th>
    <th>Value</th>
  </thead>
  <tbody>
    <tr>
      <td style="background-color: #F0FFFF;">Default</td>
      <td>?/dbs</td>
    </tr>
  </tbody>
</table>

## EXECUTION_STAGE_MEMORY_MAX
The maximum amount of Memory used by the Stage Thread performing the SELECT query in Cluster Edition.

Because it is the maximum size of each Stage, the complexity of the SELECT query with an increase in the number of Stages can lead to a larger memory requirement. 

If there is a Stage that exceeds the maximum size, the Stage is canceled and the Query is canceled with an error.

You must apply Properties directly to the specific Warehouse you want.

The default value is 1GB.

<table>
  <thead>
    <th style="background-color: lightyellow;">(size)</th>
    <th>Value</th>
  </thead>
  <tbody>
    <tr>
      <td>Minimum</td>
      <td>1024</td>
    </tr>
    <tr>
      <td>Maximum</td>
      <td>2^64 - 1</td>
    </tr>
    <tr>
      <td style="background-color: #F0FFFF;">Default</td>
      <td>1024 *1024 * 1024</td>
    </tr>
  </tbody>
</table>

## HTTP_ADMIN_PORT
Port number to receive requests from MWA or machcoordinatoradmin.

<table>
  <thead>
    <th style="background-color: lightyellow;">(port)</th>
    <th>Value</th>
  </thead>
  <tbody>
    <tr>
      <td>Minimum</td>
      <td>1024</td>
    </tr>
    <tr>
      <td>Maximum</td>
      <td>65535</td>
    </tr>
    <tr>
      <td style="background-color: #F0FFFF;">Default</td>
      <td>5779</td>
    </tr>
  </tbody>
</table>

## HTTP_CONNECT_TIMEOUT
Timeout used when connecting to machcoordinatoradmin.

<table>
  <thead>
    <th style="background-color: lightyellow;">(usec)</th>
    <th>Value</th>
  </thead>
  <tbody>
    <tr>
      <td>Minimum</td>
      <td>0</td>
    </tr>
    <tr>
      <td>Maximum</td>
      <td>2^64 - 1</td>
    </tr>
    <tr>
      <td style="background-color: #F0FFFF;">Default</td>
      <td>30000000</td>
    </tr>
  </tbody>
</table>

## HTTP_RECEIVE_TIMEOUT
Timeout used when communicating with machcoordinatoradmin.

<table>
  <thead>
    <th style="background-color: lightyellow;">(usec)</th>
    <th>Value</th>
  </thead>
  <tbody>
    <tr>
      <td>Minimum</td>
      <td>0</td>
    </tr>
    <tr>
      <td>Maximum</td>
      <td>2^64 - 1</td>
    </tr>
    <tr>
      <td style="background-color: #F0FFFF;">Default</td>
      <td>3600000000</td>
    </tr>
  </tbody>
</table>

## HTTP_SEND_TIMEOUT
Timeout used when communicating with machcoordinatoradmin.

<table>
  <thead>
    <th style="background-color: lightyellow;">(usec)</th>
    <th>Value</th>
  </thead>
  <tbody>
    <tr>
      <td>Minimum</td>
      <td>0</td>
    </tr>
    <tr>
      <td>Maximum</td>
      <td>2^64 - 1</td>
    </tr>
    <tr>
      <td style="background-color: #F0FFFF;">Default</td>
      <td>60000000</td>
    </tr>
  </tbody>
</table>

## INSERT_BULK_DATA_MAX_SIZE
Maximum size of input data block when executing Append or INSERT-SELECT.

<table>
  <thead>
    <th style="background-color: lightyellow;">(size)</th>
    <th>Value</th>
  </thead>
  <tbody>
    <tr>
      <td>Minimum</td>
      <td>1024</td>
    </tr>
    <tr>
      <td>Maximum</td>
      <td>10 * 1024 * 1024</td>
    </tr>
    <tr>
      <td style="background-color: #F0FFFF;">Default</td>
      <td>1024 * 1024</td>
    </tr>
  </tbody>
</table>

## INSERT_RECORD_COUNT_PER_NODE
Number of data inputs that lead to the warehouse group conversion when performing the input.

<table>
  <thead>
    <th style="background-color: lightyellow;">(count)</th>
    <th>Value</th>
  </thead>
  <tbody>
    <tr>
      <td>Minimum</td>
      <td>1</td>
    </tr>
    <tr>
      <td>Maximum</td>
      <td>2^64 - 1</td>
    </tr>
    <tr>
      <td style="background-color: #F0FFFF;">Default</td>
      <td>10000</td>
    </tr>
  </tbody>
</table>

## LOOKUPNODE_COMMAND_RETRY_MAX_COUNT
Number of retry when command and connection to Lookup node fails

<table>
  <thead>
    <th style="background-color: lightyellow;">(count)</th>
    <th>Value</th>
  </thead>
  <tbody>
    <tr>
      <td>Minimum</td>
      <td>1</td>
    </tr>
    <tr>
      <td>Maximum</td>
      <td>3600</td>
    </tr>
    <tr>
      <td style="background-color: #F0FFFF;">Default</td>
      <td>30</td>
    </tr>
  </tbody>
</table>

## STAGE_RESULT_BLOCK_SIZE
Maximum block size created in one stage.

<table>
  <thead>
    <th style="background-color: lightyellow;">(size)</th>
    <th>Value</th>
  </thead>
  <tbody>
    <tr>
      <td>Minimum</td>
      <td>1024</td>
    </tr>
    <tr>
      <td>Maximum</td>
      <td>2^64 - 1</td>
    </tr>
    <tr>
      <td style="background-color: #F0FFFF;">Default</td>
      <td>1024 * 1024</td>
    </tr>
  </tbody>
</table>

