
# Index

* [ALTER SYSTEM](#alter-system)
    * [KILL SESSION](#kill-session)
    * [CANCEL SESSION](#cancel-session)
    * [CHECK DISK_USAGE](#check-disk_usage)
    * [INSTALL LICENSE](#install-license)
    * [INSTALL LICENSE (PATH)](#install-license-path)
    * [SET](#set)
* [ALTER SESSION](#alter-session)
    * [SET SQL_LOGGING](#set-sql_logging)
    * [SET DEFAULT_DATE_FORMAT](#set-default_date_format)
    * [SET SHOW_HIDDEN_COLS](#set-show_hidden_cols)
    * [SET FEEDBACK_APPEND_ERROR](#set-feedback_append_error)
    * [SET MAX_QPX_MEM](#set-max_qpx_mem)
    * [SET SESSION_IDLE_TIMEOUT_SEC](#set-session_idle_timeout_sec)
    * [SET QUERY_TIMEOUT](#set-query_timeout)

## ALTER SYSTEM

This statement is the syntax for managing system-wide resources or changing settings.

### KILL SESSION

**alter_system_kill_session_stmt:**

```sql
alter_system_kill_session_stmt: 'ALTER SYSTEM KILL SESSION' number
```

Terminates a specific session with a SessionID.

However, only the SYS user can execute this statement and can not KILL their own session

### CANCEL SESSION

**alter_system_cancel_session_stmt:**

```sql
alter_system_cancel_session_stmt ::= 'ALTER SYSTEM CANCEL SESSION' number
```

Cancels a specific session with a SessionID.

Rather than disconnecting the connection, it cancels the action being performed and returns an error code to the user that the action was aborted. However, like KILL, you can not cancel your own connected sessions.

### CHECK DISK_USAGE

**alter_system_check_disk_stmt:**

```sql
alter_system_check_disk_stmt ::= 'ALTER SYSTEM CHECK DISK_USAGE'
```

Corrects the value of DC_TABLE_FILE_SIZE, which indicates the disk usage of the log table in V$STORAGE.

Disk usage may be inaccurate when process failures or power failures occur. This command reads the correct value from the file system. However, it should be avoided because it can put a considerable load on the file system.

### INSTALL LICENSE

**alter_system_install_license_stmt:**

```sql
alter_system_install_license_stmt ::= 'ALTER SYSTEM INSTALL LICENSE'
```

Installs the license file in the default location of the license file ($MACHBASE_HOME/conf/license.dat).

It is installed after determining whether the license is suitable for installation.

### INSTALL LICENSE (PATH)

**alter_system_install_license_path_stmt:**

```sql
alter_system_install_license_path_stmt: ::= 'ALTER SYSTEM INSTALL LICENSE' '=' "'" path "'"
```

Installs the license file in a specific location.

An error occurs when you enter a license file that does not exist at that location or is incorrect. The path must be an absolute path. It is installed after determining whether the license is suitable for installation.

### SET

**alter_system_set_stmt:**

```sql
alter_system_set_stmt ::= 'ALTER SYSTEM SET' prop_name '=' value
```

The list of properties that can be modified is as follows.
* QUERY_PARALLEL_FACTOR
* DEFAULT_DATE_FORMAT
* TRACE_LOG_LEVEL
* DISK_COLUMNAR_PAGE_CACHE_MAX_SIZE
* MAX_SESSION_COUNT
* SESSION_IDLE_TIMEOUT_SEC
* PROCESS_MAX_SIZE
* TAG_CACHE_MAX_MEMORY_SIZE

## ALTER SESSION

This is the syntax for managing resources or changing settings on a per-session basis.

### SET SQL_LOGGING

**alter_session_sql_logging_stmt:**

```sql
alter_session_sql_logging_stmt ::= 'ALTER SESSION SET SQL_LOGGING' '=' flag
```

Determines whether to leave a message in the Trace Log of the session.

You can use this message as a Bit Flag with the following values:
* 0x1: Parsing, Validation, Optimization.
* 0x2: It leaves the result of performing DDL.

That is, when the value of the corresponding flag is 2, only the DDL is logged, and when the flag is 3, the error and DDL are logged together.
Below is an example of changing the logging flag of the session and leaving error logging.

```sql
Mach> alter session set SQL_LOGGING=1;
Altered successfully.
Mach> exit
```

### SET DEFAULT_DATE_FORMAT

**alter_session_set_defalut_dateformat_stmt:**

```sql
alter_session_set_defalut_dateformat_stmt ::= 'ALTER SESSION SET DEFAULT_DATE_FORMAT' '=' date_format
```
Sets the default format for Datetime data types for this session.

When the server is started, the property **DEFAULT_DATE_FORMAT** is set to the session attribute. 
If the property of the property has not changed, the value of the session will also be "YYYY-MM-DD HH24: MI: SS mmm: uuu: nnn". 
Use this command to modify the default format of a datetime datatype for a specific user, regardless of the system.
V$session has a default date format set for each session and can be checked. Below is an example of checking and changing the value of the session.

```sql
Mach> CREATE TABLE time_table (time datetime);
Created successfully.
 
Mach> SELECT DEFAULT_DATE_FORMAT from v$session;
default_date_format                                                              
-----------------------------------------------
YYYY-MM-DD HH24:MI:SS mmm:uuu:nnn                                                
[1] row(s) selected.
 
Mach> INSERT INTO time_table VALUES(TO_DATE('2016/11/12'));
[ERR-00300: Invalid date value.(2016/11/12)]
 
Mach> ALTER SESSION SET DEFAULT_DATE_FORMAT='YYYY/MM/DD';
Altered successfully.
 
Mach> SELECT DEFAULT_DATE_FORMAT from v$session;
 
default_date_format                                                              
----------------------------------------------
YYYY/MM/DD                                                                       
[1] row(s) selected.
 
Mach> INSERT INTO time_table VALUES(TO_DATE('2016/11/12'));
1 row(s) inserted.
 
Mach> SELECT * FROM time_table;
 
TIME                              
----------------------------------
2016/11/12
 
[1] row(s) selected.
```

### SET SHOW_HIDDEN_COLS

**alter_session_set_hidden_column_stmt:**

```sql
alter_session_set_hidden_column_stmt ::= 'ALTER SESSION SET SHOW_HIDDEN_COLS' '=' ( '0' | '1' )
```

Decides whether to output the hidden column (_arrival_time) in the column represented by * when executing the select of the session.

When the server is started, the value of the global property SHOW_HIDDEN_COLS is set to 0 for the session attribute. 
If you want to change the default behavior of your session, you can set this value to 1.
V$session has a SHOW_HIDDEN_COLS value set for each session.

```sql
Mach> SELECT * FROM  v$session;
ID                   CLOSED      USER_ID     LOGIN_TIME                      SQL_LOGGING SHOW_HIDDEN_COLS
-----------------------------------------------------------------------------------------------------------------
DEFAULT_DATE_FORMAT                                                               HASH_BUCKET_SIZE
------------------------------------------------------------------------------------------------------
1                    0           1           2015-04-29 17:23:56 248:263:000 3           0
YYYY-MM-DD HH24:MI:SS mmm:uuu:nnn                                                 20011
[1] row(s) selected.                            
Mach> ALTER SESSION SET SHOW_HIDDEN_COLS=1;
Altered successfully.
Mach> SELECT * FROM v$session;
_ARRIVAL_TIME                   ID                   CLOSED      USER_ID     LOGIN_TIME                      SQL_LOGGING
--------------------------------------------------------------------------------------------------------------------------------
SHOW_HIDDEN_COLS DEFAULT_DATE_FORMAT                                                               HASH_BUCKET_SIZE
------------------------------------------------------------------------------------------------------------------------
1970-01-01 09:00:00 000:000:000 1                    0           1           2015-04-29 17:23:56 248:263:000 3
1           YYYY-MM-DD HH24:MI:SS mmm:uuu:nnn                                                 20011
[1] row(s) selected.
```

### SET FEEDBACK_APPEND_ERROR

**alter_session_set_feedback_append_err_stmt:**

```sql
alter_session_set_feedback_append_err_stmt ::= 'ALTER SESSION SET FEEDBACK_APPEND_ERROR' '=' ( '0' | '1' )
```

Sets whether to send the session's Append error message to the client program.

Use the following values ​​for the error message.
* 0 = Do not send an error message.
* 1 = Send an error message.
Below is an example of use.

```sql
mach> ALTER SESSION SET FEEDBACK_APPEND_ERROR=0;
Altered successfully.
```

### SET MAX_QPX_MEM

**alter_session_set_max_qpx_mem_stmt:**

```sql
alter_session_set_max_qpx_mem_stmt ::= 'ALTER SESSION SET MAX_QPX_MEM' '=' value
```

Specifies the maximum amount of memory that a single SQL statement in the session will use when performing GROUP BY, DISTINCT, ORDER BY operations.

If you try to allocate more memory than the maximum memory, the system cancels the execution of the SQL statement and treats it as an error. 
In case of error, record the error code and error message in machbase.trc including the query.

```sql
Mach> ALTER SESSION SET MAX_QPX_MEM=1073741824;
Altered successfully.
 
Mach> SELECT * FROM v$session;
ID                   CLOSED      USER_ID     LOGIN_TIME                      CLIENT_TYPE                                                                      
---------------------------------------------------------------------------------------------------------------------------------------------------------------------
USER_NAME                                                                         USER_IP                                                                           SQL_LOGGING SHOW_HIDDEN_COLS
------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------
FEEDBACK_APPEND_ERROR DEFAULT_DATE_FORMAT                                                               HASH_BUCKET_SIZE MAX_QPX_MEM          RS_CACHE_ENABLE      RS_CACHE_TIME_BOUND_MSEC
---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------
RS_CACHE_MAX_MEMORY_PER_QUERY RS_CACHE_MAX_RECORD_PER_QUERY RS_CACHE_APPROXIMATE_RESULT_ENABLE IDLE_TIMEOUT         QUERY_TIMEOUT       
-----------------------------------------------------------------------------------------------------------------------------------------------
14                   0           1           2021-03-08 16:33:01 503:181:809 CLI                                                                              
NULL                                                                              192.168.0.194                                                                     11          0               
1                     YYYY-MM-DD HH24:MI:SS mmm:uuu:nnn                                                 20011            1073741824           1                    1000                    
16777216                      50000                         0                                  0                    0                   
[1] row(s) selected.
Elapsed time: 0.001
```

- trc error when using more than the maximum memory size in an SQL statement

```sql
[2021-03-08 16:36:32 P-69000 T-140515328653056][INFO] DML FAILURE (2E10000084:Memory allocation error (alloc'd: 1048595, max: 1048576).)
```

- machsql error message when using more than the maximum memory size in an SQL statement

```sql
Mach> select * from tag order by value DESC, time ASC;
NAME                  TIME                            VALUE                      
--------------------------------------------------------------------------------------
[ERR-00132: Memory allocation error (alloc'd: 1048595, max: 1048576).]
[0] row(s) selected.
Elapsed time: 0.447
```

### SET SESSION_IDLE_TIMEOUT_SEC

**alter_session_set_session_idle_timeout_sec_stmt:**

```sql
alter_session_set_session_idle_timeout_sec_stmt ::= 'ALTER SESSION SET SESSION_IDLE_TIMEOUT_SEC' '=' value
```

Specifies the duration of the connection when the session is idle.
It is specified in seconds, and the session is terminated when the set time in the idle state elapses.
You can inquire the idle timeout time set in the session in v$session.

```sql
Mach> ALTER SESSION SET SESSION_IDLE_TIMEOUT_SEC=200;
Altered successfully.
 
 
Mach> SELECT IDLE_TIMEOUT FROM V$SESSION;
IDLE_TIMEOUT        
-----------------------
200                                     
[1] row(s) selected.
```

### SET QUERY_TIMEOUT

**alter_session_set_query_timeout_stmt:**

```sql
alter_session_set_query_timeout_stmt ::= 'ALTER SESSION SET QUERY_TIMEOUT' '=' value
```

This is the time to wait for a response from the server when performing query in the session.
It is specified in seconds, and when the response from the server exceeds the specified time after executing the query, the query is terminated.
You can inquire the QUERY_TIME in the session in v$session

```sql
Mach> ALTER SESSION SET QUERY_TIMEOUT=200;
Altered successfully.
 
Mach> SELECT QUERY_TIMEOUT FROM V$SESSION;
QUERY_TIMEOUT        
-----------------------
200                                     
[1] row(s) selected.
```
