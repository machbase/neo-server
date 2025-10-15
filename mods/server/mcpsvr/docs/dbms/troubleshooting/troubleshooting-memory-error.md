# Insufficient Memory

This section describes how to fix properties when an insufficient memory error occurs after executing a query.

## An error occurred due to insufficient memory when executing the query

The memory required to execute the query is limited for the following reasons.

If a specific query uses too much memory, other queries running at the same time may not be executed due to insufficient memory.

To prevent this, the error can be resolved by increasing the property value of the maximum size of memory that can be used by one query.

The MAX_QPX_MEM Property manages the maximum available memory that can be used in one SQL.

Refer to the SET MAX_QPX_MEM page for how to set during execution, as well as error messages and TRC messages that occur due to insufficient memory.

If the property value is set with the SET command, the set value is not applied when the machbase is restarted, so the machbase.conf file must also be modified as follows.

**Standard Edition**

Modify MAX_QPX_MEM in machbase.conf to a larger value.

**Cluster Edition**

Same as Standard edtion. However, machbase.conf of all cluster nodes must be modified.

