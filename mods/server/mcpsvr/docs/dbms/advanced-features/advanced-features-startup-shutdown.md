# Stream Startup and Shutdown

##  Stream Startup

Executes the registered stream using the stored procedure. Once executed, the stream is continuously executed. Even if the server is restarted, the continuous stream query is executed for the data inputted after the last execution.

```sql
EXEC STREAM_START(stream_name);
```

##  Direct execution of the stream

If the stream execution condition is set to BY USER, the query will not be executed without explicit call by the user. The following stored procedure is used to execute this stream query.

```sql
EXEC STREAM_EXECUTE(stream_name);
```

When creating a stream query to be called, an error occurs if the stream is not created as a BY USER condition or if the stream has not been executed with STREAM_START.

##  Stream Shutdown

Use the following stored procedure to shut down a running stream.

```sql
EXEC STREAM_STOP(stream_name);
```
