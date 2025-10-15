
# Index

* [Introduction](#introduction)
* [PARALLEL](#parallel)
* [NOPARALLEL](#noparallel)
* [FULL](#full)
* [NO_INDEX](#no_index)
* [RID_RANGE](#rid_range)
* [SCAN_FORWARD, SCAN_BACKWARD](#scan_forward-scan_backward)

Hints that can be used in a SELECT queries are described.

##  PARALLEL

Specifies parallel factor for parallel query execution.

```sql
SELECT /*+ PARALLEL(table_name, parallel_factor) */ ...
```

```sql
Mach> CREATE TABLE log_parallel_test (sensor VARCHAR(32), frequency DOUBLE, value DOUBLE, ts DATETIME);
Mach> CREATE INDEX idx_ts ON log_parallel_test (ts);
 
Mach> EXPLAIN SELECT /*+ PARALLEL(log_parallel_test, 8) */ sensor, frequency, avg(value)
      FROM log_parallel_test
      WHERE ts >= TO_DATE('2007-07-01', 'YYYY-MM-DD') and ts <= TO_DATE('2007-07-31', 'YYYY-MM-DD')
      GROUP BY sensor, frequency;
 
PLAN                                                                             
------------------------------------------------------------------------------------
 PROJECT                                                                         
  GROUP AGGREGATE                                                                
   PARALLEL INDEX SCAN                                                           
    *BITMAP RANGE (table id:3, column id:4, index id:4)                          
    [KEY RANGE]                                                                 
     * ts >= TO_DATE('2007-07-01', 'YYYY-MM-DD')                                
     * ts <= TO_DATE('2007-07-31', 'YYYY-MM-DD')                                
[7] row(s) selected.
```

##  NOPARALLEL

Does not perform in parallel.

```sql
SELECT /*+ NOPARALLEL(table_name) */ ...
```

```sql
Mach> CREATE TABLE log_parallel_test (sensor VARCHAR(32), frequency DOUBLE, value DOUBLE, ts DATETIME);
Mach> CREATE INDEX idx_ts ON log_parallel_test (ts);
 
Mach> EXPLAIN SELECT /*+ NOPARALLEL(log_parallel_test) */ sensor, frequency, avg(value)
      FROM log_parallel_test
      WHERE ts >= TO_DATE('2007-07-01', 'YYYY-MM-DD') and ts <= TO_DATE('2007-07-31', 'YYYY-MM-DD')
      GROUP BY sensor, frequency;
  
PLAN                                                                             
------------------------------------------------------------------------------------
 PROJECT                                                                         
  GROUP AGGREGATE                                                                
   INDEX SCAN                                                                    
    *BITMAP RANGE (table id:5, column id:4, index id:6)                          
    [KEY RANGE]                                                                  
     * ts >= TO_DATE('2007-07-01', 'YYYY-MM-DD')                                 
     * ts <= TO_DATE('2007-07-31', 'YYYY-MM-DD')                                 
[7] row(s) selected.
```

##  FULL

Does not use INDEX SCAN.

```sql
SELECT /*+ FULL(table_name) */ ...
```

```sql
Mach> CREATE TABLE log_full_test (sensor VARCHAR(32), I1 INTEGER);
Mach> CREATE INDEX idx_I1 ON log_full_test (I1);
 
Mach> EXPLAIN SELECT * FROM log_full_test WHERE I1 = 1;
PLAN                                                                             
------------------------------------------------------------------------------------
 PROJECT                                                                         
  INDEX SCAN                                                                     
   *BITMAP RANGE (table id:14, column id:2, index id:15)                         
   [KEY RANGE]                                                                   
    * I1 = 1                                                                     
[5] row(s) selected.
 
Mach> EXPLAIN SELECT /*+ FULL(log_full_test) */ * FROM log_full_test WHERE I1 = 1;
PLAN                                                                             
------------------------------------------------------------------------------------
 PROJECT                                                                         
  FULL SCAN                                                                      
[2] row(s) selected.
```

##  NO_INDEX

Does not use the corresponding INDEX.

```sql
SELECT /*+ NO_INDEX(table_name,index_name) */ ...
```

```sql
Mach> CREATE TABLE log_no_index_test (sensor VARCHAR(32), I1 INTEGER, I2 INTEGER);
Mach> CREATE INDEX idx_I1 ON log_no_index_test (I1);
Mach> CREATE INDEX idx_I2 ON log_no_index_test (I2);
 
Mach> EXPLAIN SELECT * FROM TEST WHERE I1 = 1;
PLAN
------------------------------------------------------------------------------------
 PROJECT
  INDEX SCAN
   *BITMAP RANGE (t:7, c:1, i:8) with BLOOMFILTER
   [KEY RANGE]                                                                  
    * I1 = 1                                                                    
[5] row(s) selected.
 
Mach> EXPLAIN SELECT /*+ NO_INDEX(TEST,TEST_IDX) */ * FROM TEST WHERE I1 = 1;
PLAN
------------------------------------------------------------------------------------
 PROJECT
  FULL SCAN
[2] row(s) selected.
 
Mach> EXPLAIN SELECT * FROM log_no_index_test WHERE I1 = 1 or I2 = 2;
PLAN                                                                             
------------------------------------------------------------------------------------
 PROJECT                                                                         
  INDEX SCAN                                                                     
   INDEX (OR)                                                                    
    *BITMAP RANGE (table id:21, column id:2, index id:22)                        
    *BITMAP RANGE (table id:21, column id:3, index id:23)                        
   [KEY RANGE]                                                                   
    * I1 = 1 or I2 = 2                                                           
[7] row(s) selected.
 
Mach> EXPLAIN SELECT /*+ NO_INDEX(log_no_index_test, idx_I1) */ * FROM log_no_index_test WHERE I1 = 1 or I2 = 2;
PLAN                                                                             
------------------------------------------------------------------------------------
 PROJECT                                                                         
  FULL SCAN                                                                      
[2] row(s) selected.
 
Mach> EXPLAIN SELECT * FROM log_no_index_test WHERE I1 = 1 and I2 = 2;
PLAN                                                                             
------------------------------------------------------------------------------------
 PROJECT                                                                         
  INDEX SCAN                                                                     
   *BITMAP RANGE (table id:21, column id:2, index id:22)                         
   *BITMAP RANGE (table id:21, column id:3, index id:23)                         
   [KEY RANGE]                                                                   
    * I1 = 1                                                                     
    * I2 = 2                                                                     
[7] row(s) selected.
 
Mach> EXPLAIN SELECT /*+ NO_INDEX(log_no_index_test, idx_I1) */ * FROM log_no_index_test WHERE I1 = 1 and I2 = 2;
 
PLAN                                                                             
------------------------------------------------------------------------------------
 PROJECT                                                                         
  INDEX SCAN                                                                     
   *BITMAP RANGE (table id:21, column id:3, index id:23)                         
   [KEY RANGE]                                                                   
    * I2 = 2                                                                     
   [FILTER]                                                                      
    * I1 = 1                                                                     
[7] row(s) selected.
Elapsed time: 0.001
Mach>
Mach>
Mach> EXPLAIN SELECT /*+ NO_INDEX(log_no_index_test, idx_I2) */ * FROM log_no_index_test WHERE I1 = 1 and I2 = 2;
 
PLAN                                                                             
------------------------------------------------------------------------------------
 PROJECT                                                                         
  INDEX SCAN                                                                     
   *BITMAP RANGE (table id:21, column id:2, index id:22)                         
   [KEY RANGE]                                                                   
    * I1 = 1                                                                     
   [FILTER]                                                                      
    * I2 = 2                                                                     
[7] row(s) selected.
```

##  RID_RANGE

Runs within RID range.

```sql
SELECT /*+ RID_RANGE(table_name,number,number) */ ...
```

```sql
Mach> SELECT /*+ RID_RANGE(TEST,45,50) */ _RID, * FROM TEST;
_RID                 I1
------------------------------------
49                   1
48                   1
47                   1
46                   1
45                   1
[5] row(s) selected.
```

##  SCAN_FORWARD, SCAN_BACKWARD

It specifies the direction of scanning for LOG table. With SCAN_FORWARD, the oldest record input is retrived first, whereas with SCAN_BACKWARD, the newest record input is retrieved first.

It affects LOG tables in standard edition only.

```sql
SELECT /*+ SCAN_FORWARD(table_name) */ ...
SELECT /*+ SCAN_BACKWARD(table_name) */ ...
```

```sql
Mach> SELECT /*+ SCAN_FORWARD(mytbl) */  _ARRIVAL_TIME, VALUE FROM mytbl LIMIT 10;
_ARRIVAL_TIME                   VALUE                   
----------------------------------------------------------------
2017-01-01 00:00:49 500:000:000 0                         
2017-01-01 00:01:39 500:000:000 1                         
2017-01-01 00:02:29 500:000:000 2                         
2017-01-01 00:03:19 500:000:000 3                         
2017-01-01 00:04:09 500:000:000 4                         
2017-01-01 00:04:59 500:000:000 5                         
2017-01-01 00:05:49 500:000:000 6                         
2017-01-01 00:06:39 500:000:000 7                         
2017-01-01 00:07:29 500:000:000 8                         
2017-01-01 00:08:19 500:000:000 9                         
[10] row(s) selected.
 
Mach> SELECT /*+ SCAN_BACKWARD(mytbl) */ _ARRIVAL_TIME, VALUE FROM mytbl LIMIT 10;
_ARRIVAL_TIME                   VALUE                   
----------------------------------------------------------------
2017-02-27 20:53:19 500:000:000 9                         
2017-02-27 20:52:29 500:000:000 8                         
2017-02-27 20:51:39 500:000:000 7                         
2017-02-27 20:50:49 500:000:000 6                         
2017-02-27 20:49:59 500:000:000 5                         
2017-02-27 20:49:09 500:000:000 4                         
2017-02-27 20:48:19 500:000:000 3                         
2017-02-27 20:47:29 500:000:000 2                         
2017-02-27 20:46:39 500:000:000 1                         
2017-02-27 20:45:49 500:000:000 0                         
[10] row(s) selected.
```
