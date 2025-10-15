# Simple Join

Log tables, volatile tables, lookup tables and meta tables can be searched by Join.

## Simple Join

```sql
Mach> CREATE TABLE logtable (code INT,value INT);
Created successfully.
 
Mach> INSERT INTO logtable VALUES(1,20 );
1 row(s) inserted.
 
Mach> INSERT INTO logtable VALUES(2,10 );
1 row(s) inserted.
 
Mach> INSERT INTO logtable VALUES(3,15 );
1 row(s) inserted.
 
Mach> INSERT INTO logtable VALUES(4,20 );
1 row(s) inserted.
 
Mach> INSERT INTO logtable VALUES(5,10 );
1 row(s) inserted.
 
Mach> CREATE VOLATILE table VTABLE (code INT,name VARCHAR(32));
Created successfully.
 
Mach> INSERT INTO vtable VALUES(1, 'Sam');
1 row(s) inserted.
 
Mach> INSERT INTO vtable VALUES(3, 'Thomas');
1 row(s) inserted.
 
Mach> INSERT INTO vtable VALUES(5, 'Micheal');
1 row(s) inserted.
 
Mach> INSERT INTO vtable VALUES(7, 'Jessica');
1 row(s) inserted.
 
Mach> SELECT name,value FROM logtable, vtable WHERE logtable.code=vtable.code;
name                              value
-------------------------------------------------
Micheal                           10
Thomas                            15
Sam                               20
[3] row(s) selected.
```

## Join Using Alias

When using Join, an alias can be used for the join target table.

```sql
SELECT c.name FROM m$sys_tables t, m$sys_columns c WHERE t.id = c.table_id AND t.name = 'T1'
AND c.id NOT IN(0, 65534) ORDER BY c.name;
 
c.name                                  
--------------------------------------------
ADDR
ISTYPE
SRCIP                        
[3] row(s) selected.
```

## GROUP BY/ORDER BY 

GROUP BY, ORDER BY, and aggregate functions are also available.

```sql
Mach> SELECT t.name, COUNT(c.name) FROM m$sys_columns c, m$sys_tables t WHERE t.id = c.table_id GROUP BY t.name ORDER BY t.name;
t.name                                    count(c.name)
------------------------------------------------------------------
COMMON_TABLE                              5
DURATIONT                                 3
[2] row(s) selected.
```

## Join without JOIN clause 

A join query without a JOIN clause causes an error. Because there is so much data in the log table, the speed of queries without join conditionality is unpredictably slow.

Also, two log table joins can be very slow. So, when designing a database, it is better to design so that join does not occur considering denormalization.

```sql
Mach> CREATE TABLE log_table1(i1 INTEGER);
Created successfully.
Mach> INSERT INTO log_table1 VALUES(1);
1 row(s) inserted.
Mach> INSERT INTO log_table1 VALUES(20);
1 row(s) inserted.
Mach> INSERT INTO log_table1 VALUES(30);
1 row(s) inserted.
 
 
Mach>CREATE TABLE log_table2(i1 INTEGER);
Created successfully.
Mach> INSERT INTO log_table2 VALUES(1);
1 row(s) inserted.
Mach> INSERT INTO log_table2 VALUES(30);
1 row(s) inserted.
Mach> INSERT INTO log_table2 VALUES(50);
1 row(s) inserted.
 
Mach> SELECT log_table1.i1 FROM log_table1, log_table2;
[ERR-02101 : Error in joining tables. Cannot join without join predicate.]
 
Mach> SELECT log_table1.i1 FROM log_table1, log_table2 where log_table1.i1 = 1;
[ERR-02101 : Error in joining tables. Cannot join without join predicate.]
 
Mach> SELECT log_table1.i1 from log_table1, log_table2 WHERE log_table1.i1 = log_table2.i1;
i1
--------------
30
1
[2] row(s) selected.
```

## Inner Join / Outer Join

ANSI type INNER, LEFT OUTER, or RIGHT OUTER join can be used, but FULL OUTER JOIN can not be used.

```sql
FROM    TABLE_1 [INNER|LEFT OUTER|RIGHT OUTER]  JOIN    TABLE_2 ON  expression
```

```sql
SELECT t1.i1 t2.i1 FROM t1 LEFT OUTER JOIN t2 ON (t1.i1 = t2.i1) WHERE t2.i2 = 1;
```

The above query is changed to Inner Join by t2.i2 = 1 condition in the where clause.
