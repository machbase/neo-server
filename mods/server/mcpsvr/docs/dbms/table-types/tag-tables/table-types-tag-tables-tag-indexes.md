# Tag Table Indexes

## Overview

Indexes on tag tables significantly improve query performance when searching by additional columns or JSON paths. This guide covers how to create and manage TAG indexes effectively.

## What are TAG Indexes?

TAG index types can be created on Machbase TAG table. 

For more information, refer to the DDL section of the SQL Reference  .

* TAG Index: TAG index can be created in additional columns in TAG table.

## Create Index

Create an index on a specific column using the CREATE INDEX statement.

```sql
CREATE INDEX index_name ON table_name (column_name) [index_type]
    index_type ::= INDEX_TYPE { TAG }
```

```bash
Mach> CREATE INDEX id_index ON tag (id) INDEX_TYPE TAG;
Created successfully.
```

Starting with version 7.5, indexes can be created for each json path for json type columns only in the tag table.

Just connect the json path with the operator to the existing index creation syntax.

Since the return type of the json operator is VARCHAR, indexes are used only when comparing VARCHARs.

```bash
Mach> CREATE TAG TABLE tag (name VARCHAR(20) PRIMARY KEY, time DATETIME BASETIME, jval JSON);
Executed successfully.
  
Mach> CREATE INDEX idx_jval_value1 ON tag (jval->'$.value1');
Created successfully.
  
Mach> CREATE INDEX idx_jval_value2 ON tag (jval->'$.value2');
Created successfully.
  
Mach> EXPLAIN SELECT * FROM tag WHERE jval->'$.value1' = '10';
PLAN                                                                            
------------------------------------------------------------------------------------
 PROJECT                                                                        
  TAG READ (RAW)                                                                
   KEYVALUE INDEX SCAN (_TAG_DATA_0)                                            
    [KEY RANGE]                                                                 
     * jval->'$.value1' = '10'                                                  
   VOLATILE FULL SCAN (_TAG_META)                                               
[6] row(s) selected.
```

## Delete Index

Delete the specified index using the DROP INDEX statement. However, if there is another session in which the table is being searched, it will fail with an error.

```sql
DROP INDEX index_name;
```

```bash
Mach> DROP INDEX id_index;
Dropped successfully.
```
