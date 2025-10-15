# Stream Creating and Deleting

##  Create Stream

The stream query can only be generated in the form of Insert... Select. When generating the stream, the query is checked to see if it is a query that can be executed normally.
Use the following stored procedure to create the stream.

Even if the stream is successfully created, execution will not start immediately. For more information, refer to Stream Startup and Shutdown.

```sql
EXEC STREAM_CREATE(stream_name, stream_query_string);
```

The basic stream query is executed on every input data. In this case, statistical queries such as SUM and AVG cannot be used.

```sql
EXEC STREAM_CREATE(normal_query, 'INSERT INTO CEP_LOG_TABLE SELECT * FROM EVENT WHERE C1 = 0');
```

However, if the period at which the stream query is to be executed is set at the end of the insert select statement, the statistical query for the input data can be used at regular intervals.

```sql
EXEC STREAM_CREATE(aggr_1_sec, 'insert into aggr select sum(i1), i2 from base group by i2 BY 1 SECOND');
```

The above stream query executes a group by query on the latest data entered after the last execution every second and inputs the result to the "aggr" log table.

If the user wants to define the execution time of the stream query, specify the following in the execution cycle setting section.

The stream query is not executed before the explicit call of the user.

```sql
EXEC STREAM_CREATE(base_trig, 'insert into aggr select sum(i1), i2 from base group by i2 BY USER');
```

If the condition for executing a stream query is BY USER, it will not be executed until the stream query is explicitly called using the STREAM_EXECUTE procedure.

This  called with STREAM_EXECUTE, executes a stream query only for incremental data added during execution, except for those previously read.

##  Delete Stream

The list of generated streams can be retrieved using the V$STREAMS meta table. To delete a stream, use the following stored procedure with the name of the stream that you determined when you created the stream as a parameter.

```sql
EXEC STREAM_DROP(stream_name);
```

A running stream can not be deleted.  The stream must first be shut down before deleting the stream. For more information, refer to Stream Startup and Shutdown.

## V$STREAMS

This is a meta table to check the current status of streams registered in the DB server. Detailed explanations are provided in the virtual table section of the manual.
