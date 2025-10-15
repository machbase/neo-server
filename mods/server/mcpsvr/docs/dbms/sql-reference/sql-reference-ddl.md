# DDL

## CREATE TABLE

### Syntax

**create_table_stmt:**

**column_list:**

**column_property_list:**

**table_property_list:**

**column_type:**

**with_rollup:**

#### Create LOG table

```sql
-- create ctest LOG table with 5 columns
CREATE TABLE ctest (id INTEGER, name VARCHAR(20), sipv4 IPV4, dipv6 IPV6, comment TEXT);
```

#### Create TAG table

TAG tables must have PRIMARY KEY, BASETIME, SUMMARIZED columns.

```sql
-- create TAG table
CREATE TAG TABLE tag (name VARCHAR(20) PRIMARY KEY, time DATETIME BASETIME, value DOUBLE SUMMARIZED);
CREATE TAG TABLE tag (name VARCHAR(20) PRIMARY KEY, time DATETIME BASETIME, value DOUBLE SUMMARIZED, value2 FLOAT, int_column INT);
CREATE TAG TABLE tag (name VARCHAR(20) PRIMARY KEY, time DATETIME BASETIME, value DOUBLE SUMMARIZED, value2 FLOAT) METADATA (i1 INT);
```

#### Rules for naming tables or columns

Table names or column names consist of alphanumeric characters. Use double quotation marks (`"`) to use special characters.

```sql
CREATE TABLE special_tbl ( "with.dot" INTEGER ); -- (Possible)
```

#### IF NOT EXISTS

Prevents an error from occurring if the table exists. However, there is no verification that the existing table has a structure identical to that indicated by the CREATE TABLE statement.

This function only takes effect when the types of tables are equal.

### Table Type

|Table Type|Description|
|--|--|
|LOG|If there is no keyword between CREATE TABLE, a log table is created.|
|VOLATILE|VOLATILE_TABLE is a temporary table in which all data resides in temporary memory and joins the log table to improve the results,<br>The Machbase server disappears as soon as it is shut down.|
|LOOKUP|Like VOLATILE_TABLE, LOOKUP_TABLE can perform fast query processing by storing all the data in memory.|

### Table Property

Specifies the attributes for the table.

|**Property Name**|**Available Table Types**|
|--|--|
|TAG_PARTITION_COUNT|TAG table |
|TAG_DATA_PART_SIZE|TAG table |
|TAG_STAT_ENABLE|TAG table |
|TAG_DUPLICATE_CHECK_DURATION| TAG table |
|VARCHAR_FIXED_LENGTH_MAX| TAG table |

#### TAG_PARTITION_COUNT(Default:4)
A supported attribute for the TAG table, determines how many partition tables will store the TAG table internally. It should be set according to the number of tags or the performance of the server.

#### TAG_DATA_PART_SIZE(Default:16MB)
A supported attribute for the TAG table, determines the data size for each partition table.

#### TAG_STAT_ENABLE(Default:1)
A supported attribute for the TAG Table, determines whether to store statistical information for each TAG ID.

#### TAG_DUPLICATE_CHECK_DURATION(Default:0, Max:43200)
A supported attribute for the TAG Table, the period within which duplicates can be removed is set in minutes based on the current system time. Duplicates can be deleted only for data within this specified period from the current system time. If the set period is 0, duplicate removal will not be performed.

#### VARCHAR_FIXED_LENGTH_MAX (Default: 15, Max: 127)

Specifies the length of the maximum varchar column to be stored in the internal file.

### Column Property

Specifies the attribute for the column.

|**Property Name**|**Available Table Types**|
|--|--|
|PART_PAGE_COUNT|LOG TABLE|
|PAGE_VALUE_COUNT|LOG TABLE|
|MAX_CACHE_PART_COUNT|LOG TABLE|
|MINMAX_CACHE_SIZE|LOG TABLE|

**PART_PAGE_COUNT**
This property represents the number of pages a partition has. The number of values ​​that a partition has is PART_PAGE_COUNT * PAGE_VALUE_COUNT.

**PAGE_VALUE_COUNT**
This property represents the number of values ​​that a page has.

**MAX_CACHE_PART_COUNT (Default : 0)**
This property sets the cache area for performance.
When Machbase accesses a partition, it first looks for a structure that contains the meta information of that partition in memory. It determines how many partition information it contains in memory. Larger size will help performance, but memory usage will increase. The minimum value is 1 and the maximum value is 65535.

**MINMAX_CACHE_SIZE (Default : 10240)**
This property specifies how much cache memory to use for the MINMAX of the corresponding column. The default is 100MB for _ARRIVAL_TIME, the 0th hidden column. However, other columns are specified as 10KB by default. This size can be changed after the creation of the table through the "ALTER TABLE MODIFY" statement.

**NOT NULL Constraint**
Specifies NOT NULL if the column value does not allow NULL, and omit it if it is allowed (Default).
You can change the constraint with the ALTER TABLE MODIFY COLUMN command to drop or add this constraint defined after the creation of the table.

```sql
## Column c1 is not null and c2 is created without not null constraint.
CREATE TABLE t1(c1 INTEGER NOT NULL, c2 VARCHAR(200));
```

**Pre-defined System Columns**
When you create a table using the Create Table statement, the system creates two additional predefined system columns. _ARRIVAL_TIME and _RID columns.

The _ARRIVAL_TIME column is inserted into the DATETIME column based on the system time at which data is inserted into the INSERT statement or AppendData, and the value can be used as the unique key of the generated record. The value of this column can be inserted by specifying the value in the machloader or INSERT statement if the order is guaranteed (in the order of past-present). When data is retrieved using the DURATION conditional expression, data is retrieved based on the value of this column.
The _RID column is created by the system as a unique value for a particular record. The data type of this column is a 64-bit integer. For this column, the user can not specify a value and can not create an index. It is automatically generated at the time of data INSERT. You can retrieve records by the value of the _RID column.

```sql
create volatile table t1111 (i1 integer);
Created successfully.
Mach> desc t1111;
 
----------------------------------------------------------------
NAME                          TYPE                LENGTH       
----------------------------------------------------------------
_ARRIVAL_TIME                 datetime            8              
I1                            integer             4              
 
Mach>insert into t1111 values (1);
1 row(s) inserted.
Mach>select _rid from t1111;
_rid                
-----------------------
0                   
[1] row(s) inserted.
 
Mach>select i1 from t1111 where _rid = 0;
i1         
--------------
1          
[1] row(s) selected.
```

### Min Max Cache

#### The Concept of Min-Max Cache

In general, in the Disk DBMS, when a specific value is searched using the index, the disk is accessed to access the disk area including the index, and the final disk page including the corresponding value is searched.

On the other hand, Machbase is a chronologically partitioned structure in order to maintain time series information, which means that a particular piece of index information is divided into chunks of files in chronological order. Therefore, when a Machbase index is used, an index file fragmented by such a partition is sequentially searched.
If the range of data to be searched is divided into 1000 partitions, it means that 1000 files should be opened and retrieved every time. Although it is designed as an efficient columnar database structure, the MINMAX_CACHE structure is a way to improve the performance because the I/O cost is proportional to the number of index partitions.
MINMAX_CACHE is a structure that holds the index file information of the partition in memory, and is a contiguous memory space that keeps the minimum and maximum values ​​of the column in memory. By maintaining such structure, when a partition containing a specific value is searched, if the value is smaller than the minimum value of the index or is larger than the maximum value, the corresponding partition can be skipped altogether, thereby enabling high-performance data analysis.

As shown in the figure above, to find the value 85, only the partitions 1 and 5 included in MIN/MAX among the 5 partitions are actually searched, and the partitions 2, 3 and 4 are skipped altogether.

#### Min-Max Cache Column

You can decide whether to use MINMAX Cache for a particular column when creating the table.

If the minmax_cache_size is set to a value other than 0, the MINMAX Cache will be active when the index is searched for that column and will not be active if  MINMAX_CACHE_SIZE = 0.
Please note the following when using this MINMAX Cache.

1. MINMAX Cache does not need to explicitly create an index on the column.
2. As default for all columns, MINMAX_CACHE_SIZE is set to 10KB and the Alter Table syntax can be used to reset the memory size to a reasonable size.
3. The hidden column _arrival_time is 100MB by default and automatically uses MINMAX Cache memory.
4. In the case of VARCHAR type, MINMAX Cache is not covered. Therefore, if you explicitly specify whether the VARCHAR type is cached, an error will occur.
5. When the corresponding table is created, the MINMAX_CACHE_SIZE maximum memory can be used as much as the property is set. As the number of partitions grows, the memory grows gradually and increases by the maximum memory above.
6. If there are no records in the table, MINMAX Cache memory is not allocated at all.

Below is an example of table creation using actual MINMAX.

```sql

-- MINMAX_CACHE_SIZE = 0 for VARCHAR is allowed semantically.
CREATE TABLE ctest (id INTEGER, name VARCHAR(100) PROPERTY(MINMAX_CACHE_SIZE = 0));
Created successfully.
Mach>
 
-- Cache applied to id column.
CREATE TABLE ctest2 (id INTEGER PROPERTY(MINMAX_CACHE_SIZE = 10240), name VARCHAR(100) PROPERTY(MINMAX_CACHE_SIZE = 0));
Created successfully.
Mach>
 
-- Applied to id1, id2, and id3.
CREATE TABLE ctest3 (id1 INTEGER PROPERTY(MINMAX_CACHE_SIZE = 10240), name VARCHAR(100) PROPERTY(MINMAX_CACHE_SIZE = 0), id2 LONG PROPERTY(MINMAX_CACHE_SIZE = 1024), id3 IPV4 PROPERTY(MINMAX_CACHE_SIZE = 1024), id4 SHORT);
Created successfully.
Mach>
 
-- MINMAX_CACHE_SIZE is specified in column units or set to 0.
CREATE TABLE ctest4 (id1 INTEGER PROPERTY(MINMAX_CACHE_SIZE=10240), name VARCHAR(100) PROPERTY(MINMAX_CACHE_SIZE=0), id2 LONG PROPERTY(MINMAX_CACHE_SIZE=10240), id3 IPV4 PROPERTY(MINMAX_CACHE_SIZE=0), id4 SHORT);
Created successfully.
Mach>
```

### Primary Key

This is a constraint that can be assigned to a Volatile/Lookup table column. The Volatile / Lookup table does not always need to have a primary key, but you can not use the INSERT ON DUPLICATE KEY UPDATE statement without a primary key.

When a primary key is assigned, a red-black tree index corresponding to the primary key is generated.

### Sequence Column

#### SEQUENCE for Lookup Table

Sequence was added to generate a unique record of the Lookup table and determine the order in which the data is entered.

This feature was added to solve problems such as difficulty in distinguishing the order of records if the datetime values overlap in the lookup table and application errors due to data duplication.

#### Configuring Sequence when Creating Lookup Tables

When creating a lookup table with a CreateTable SQL statement, simply specify that you want to set the Sequence by adding a PROPERTY clause to the column to be used as the Sequence.

The columns to be set in Sequence only support LONG datatype (64bit, unsigned) and no other.

In addition, the start value of Sequence can be set, but if it is set to 1, Sequence starts from 1. (No support for 0 or negative numbers)

```sql
CREATE LOOKUP TABLE table_name (v1 LONG PROPERTY(SEQUENCE=1) PRIMARY KEY, v2 VARCHAR(10));
```

#### Use of sequence column

The Sequence column of the Lookup table is basically the same as a regular Long column and when used in this way, the Sequence value does not automatically increase.

It is allowed to enter values directly into the Sequence column, and even duplicate values can be entered.

Instead, if you want to use the Sequence function, you should use a newly added Sequence-only function called nextval to increase the Sequence value.

Internally, it stores the largest value of a column set to Sequence, so when you enter it later using nextval Function, the largest value of the Sequence column value +1 is stored.

**Example of Sequence column**
```sql
-- Insert the following Sequence value using nextval Function in the Sequence column.
INSERT INTO table_name (v1, v2) values (nextval(v1), 'aaaa');
   
-- Insert a value directly into the Sequence column
INSERT INTO table_name (v1, v2) values (100, 'aaaa');
   
-- Insert a the computational value in the Sequence column.
INSERT INTO table_name (v1, v2) values (100 + 1, 'aaaa');
   
-- Success Select of Lookup Tables with Sequence Columns
SELECT v1, v2 FROM table_name;
  
-- Invalid Select for Sequence column (nextval column can only be used in insert query)
SELECT nextval(v1), v2 FROM table_name;
```

## DROP TABLE

**drop_table_stmt:**

```sql
drop_table_stmt ::= 'DROP TABLE' table_name
```

Deletes the specified table. However, if there is another session in which the table is being searched, it fails with an error.

```sql
-- Example
DROP TABLE TableName;
```

## CREATE TABLESPACE

**create_tablespace_stmt:**

**datadisk_list:**

**data_disk:**

**data_disk_property:**

```sql
create_tablespace_stmt ::= 'CREATE TABLESPACE' tablespace_name 'DATADISK' datadisk_list
datadisk_list ::= data_disk ( ',' data_disk )*
data_disk ::= disk_name data_disk_property
data_disk_property ::= '(' 'DISK_PATH' '=' '"' path '"' ( ',' 'PARALLEL_IO' '=' number )? ')'
```

```sql
-- Example
create tablespace tbs1 datadisk disk1 (disk_path=""); -- $MACHBASE_HOME/dbs/  (Create in $MACHBASE_HOME/dbs/)
create tablespace tbs1 datadisk disk1 (disk_path="tbs1_disk1"); -- $MACHBASE_HOME/dbs/tbs1_disk1  (Created in $MACHBASE_HOME/dbs/tbs1_disk1. tbs1_disk1 folder must exist)
create tablespace tbs2 datadisk disk1 (disk_path="tbs2_disk1", parallel_io = 5);
create tablespace tbs1 datadisk disk1 (disk_path="tbs1_disk1", parallel_io = 10), disk2 (disk_path="tbs1_disk2"), disk3 (disk_path="tbs1_disk3");
```

The CREATE TABLESPACE statement creates a tablespace in $MACHBASE_HOME/dbs/ where the indexes of the log table or log table will be stored.

Tablespace can have multiple disks. When each Partition File that stores data of Table and Index is stored, it is distributed and stored in Data Disks belonging to Tablespace.
If two or more disks are used, the index and table files are distributed and stored on each disk, and I/O is performed in parallel on each device. As the number of disks increases, disk I / O throughput increases, and a large amount of data can be stored on the disk quickly
Also, if tables and index tablespace are separately created and different disks are defined, I/O of table and index can be logically separated without reconfiguration of physical disk.

### DATA DISK

Defines disk belonging to a tablespace. Each Disk has the following properties.

|Property|Description|
|--|--|
|data_disk_property|Specifies the attributes of the disk.|
|disk_name|Specifies the name of the Disk object. It is used to change the attributes of the Disk object through Alter Tablespace syntax later.|
|disk_path|Specifies the Directory Path of the disk. This Directory must be created. When a path is specified as a relative path, PATH is searched based on $MACHBASE_HOME/dbs. For example, if PATH = 'disk1', Disk Path is recognized as $MACHBASE_HOME/dbs/disk1.|
|parallel_io|Determines how many disk IO requests are allowed to be paralleled. (DEF: 3, MIN: 1, MAX: 128)|

## DROP TABLESPACE

**drop_tablespace_stmt:**

```sql
drop_table_stmt ::= 'DROP TABLESPACE' tablespace_name
```

Deletes the specified tablespace. However, if the object created in Tablespace exists, deletion fails.

```sql
-- Example
DROP TABLESPACE TablespaceName;
```

## CREATE INDEX

**create_index_stmt:**

**index_type:**

**table_space:**

**index_property_list:**

```sql
create_index_stmt ::= 'CREATE' 'INDEX' index_name 'ON' table_name '(' column_name ')' index_type? table_space? index_property_list?
index_type ::= 'INDEX_TYPE' ( 'KEYWORD' | 'BITMAP' | 'REDBLACK' )
table_space ::= 'TABLESPACE' table_space_name
index_property_list ::= ( 'MAX_LEVEL' | 'PAGE_SIZE' | 'BITMAP_ENCODE' | 'PART_VALUE_COUNT' ) '=' value
```

### Index Type

Specifies the Index Type to be created. If it is not Keyword Index, Index Type is created as Default Index Type according to Table Type if Index Type is not specified.

|Table Type|Default Index Type|
|--|--|
|Volatile Table|REDBLACK|
|Lookup Table|REDBLACK|
|Log Table|LSM|

### KEYWORD Index

This can be created only for varchar and text column of log table. It can be created for only one column.

### LSM Index

LSM (Log Structure Merge) Index is an index optimized for storing and searching Big Data. The partitions of the LSM indexes are maintained for each level, and the lower level partitions are merged to move to the upper level. Lower partitions used to create a higher level partition are deleted.

This Index Level Partition Building is performed by Background Thread. The upper level partitions are merged with the lower level partitions and are created as one partition, so there are the following advantages when searching through the index.

1. If the key is duplicated, the disk space for key storage is saved because it is stored only once.

2. Searching for multiple partitions reduces the cost of opening and closing the file when searching for one index partition, and the number of index pages accessed is also reduced.

### LSM Index Property

|Item|Description|
|--|--|
|MAX_LEVEL<br>(DEFAULT = 3, MIN = 0, MAX = 3 )|The maximum level of the LSM Index, and the current value of 3 is the maximum value. And the maximum number of records of one partition can not exceed 200 million. The partition size of each level is the number of values ​​of the previous partition * 10. For example, if MAX_LEVEL = 3 and PART_VALUE_COUNT is 100,000, then Level 0 = 100,000, Level 1 = 1,000,0000, Level 2 = 10,000,000, and Level 3 = 100,000,000. If the Partition Size of the last level exceeds 200 million, index creation will fail.|
|PAGE_SIZE<br>(DEFAULT = 512 * 1024, MIN = 32 * 1024,MAX = 1 * 1024 * 1024)|Specifies the size of the page in which the index key value and bitmap value are stored. Default is 512K.|
|BITMAP_ENCODE<br>(DEFAULT = EQUAL, RANGE)|Sets the bitmap type of the index.<br>If BITMAP_ENCODE = EQUAL (default), generates a bitmap for the same value as the key value. If BITMAP = RANGE, generates a bitmap according to the range of the key value.<br>It is better to set as BITMAP_ENCODE = EQUAL when using = as the query condition, and BITMAP_ENCODE = RANGE when using the specific range value as the query condition.<br>In the case of BITMAP = RANGE, the cost of creation increases slightly compared to EQUAL.|

### BITMAP Index

This is an index for data analysis and can be created only in the log table. It can be created on all columns except varchar, text, and binary, and can only be created on a single column.

### RED-BLACK Index

This is a memory index for real-time data retrieval. It can be created only in the Volatile/Lookup table. It can be created in all columns of this table and can only be created for a single column.

### Index Property

The properties that can be applied in the LSM Index are as follows.

**PART_VALUE_COUNT**
Indicates the number of rows stored in the Partition of Index.

```sql
-- Example
-- Index applied to c1 column.
CREATE INDEX index1 on table1 ( c1 )
-- Keyword index applied to var_column of varchar type, and page_size unit is 100000.
CREATE INDEX index2 on table1 (var_column) INDEX_TYPE KEYWORD PAGE_SIZE=100000;
```

## DROP INDEX

**drop_index_stmt:**

```sql
drop_index_stmt ::= 'DROP INDEX' index_name
```

Deletes the specified index. However, if there is another session in which the table is being searched, it fails with an error.

```sql
-- Example
DROP INDEX IndexName;
```

## ALTER TABLE

The ALTER TABLE statement is used to change the schema information of the specified table.

- Most ALTER TABLE operations are available only for Log Tables
- RENAME COLUMN operation is available for both Log Tables and Tag Tables

### ALTER TABLE SET

This syntax changes the properties of a table. Currently there are no dynamically changeable properties.

### ALTER TABLE ADD COLUMN

**alter_table_add_stmt:**

```sql
alter_table_add_stmt ::= 'ALTER TABLE' table_name 'ADD COLUMN' '(' column_name column_type ( 'DEFAULT' value )? ')'
```

This syntax is the ability to add a specific column to the table in real time. You can add the name and type of the column, and set the default data values ​​through the DEFAULT clause.

```sql
-- Example-1
alter table atest2 add column (id4 float);
 
-- Example-2
alter table atest2 add column (id6 double  default 5);
alter table atest2 add column (id7 ipv4  default '192.168.0.1');
alter table atest2 add column (id8 varchar(4) default 'hello');
```

### ALTER TABLE DROP COLUMN

**alter_table_drop_stmt:**

```sql
alter_table_drop_stmt ::= 'ALTER TABLE' table_name 'DROP COLUMN' '(' column_name ')'
```

This syntax is to delete a specific column in the table in real time.

```
-- Example
alter table atest2 drop column (id4);
alter table atest2 drop column (id8);
```

### ALTER TABLE METADATA ADD COLUMN

**alter_table_metadata_add_stmt:**

```sql
alter_table_metadata_add_stmt ::= 'ALTER TABLE' table_name 'METADATA ADD COLUMN' '(' column_name column_type ( 'DEFAULT' value )? ')'
```

This syntax adds a metadata column to a TAG table. Metadata columns store tag-specific information that doesn't change frequently with each data point.

> **Note**: This operation is only available for TAG tables.

```sql
-- Example: Add metadata columns to a TAG table
ALTER TABLE altertbl METADATA ADD COLUMN (m1 DOUBLE);
ALTER TABLE altertbl METADATA ADD COLUMN (m2 VARCHAR(100));
ALTER TABLE altertbl METADATA ADD COLUMN (m3 INTEGER DEFAULT 0);
```

### ALTER TABLE METADATA DROP COLUMN

**alter_table_metadata_drop_stmt:**

```sql
alter_table_metadata_drop_stmt ::= 'ALTER TABLE' table_name 'METADATA DROP COLUMN' '(' column_name ')'
```

This syntax removes a metadata column from a TAG table.

> **Note**: This operation is only available for TAG tables.

```sql
-- Example: Drop metadata columns from a TAG table
ALTER TABLE altertbl METADATA DROP COLUMN (m1);
ALTER TABLE altertbl METADATA DROP COLUMN (m2);
```

### ALTER TABLE RENAME COLUMN

**alter_table_column_rename_stmt:**

```sql
alter_table_column_rename_stmt ::= 'ALTER TABLE' table_name 'RENAME COLUMN' old_column_name 'TO' new_column_name
```

This syntax is a function that changes a specific column name in a table. This operation is available for both Log Tables and Tag Tables.

```sql
-- Example for Log Table
alter table atest2 rename column id7 to id7_rename;

-- Example for Tag Table
alter table tag rename column v0001 to vmax;
```

> **Note**: For Tag Tables, you can rename any column including additional value columns, but PRIMARY KEY, BASETIME, and METADATA column names can also be changed. However, if the tag table has ROLLUP tables defined, renaming columns may be restricted.

> **Note**: RENAME COLUMN operation for Tag Tables is supported from Machbase version 8.0.50 or later.

### ALTER TABLE MODIFY COLUMN

**alter_table_modify_stmt:**

```sql
alter_table_modify_stmt ::= 'ALTER TABLE' table_name 'MODIFY COLUMN' ( '(' column_name 'VARCHAR' '(' new_size ')' ')' | column_name ( 'NOT'? 'NULL' | 'SET' 'MINMAX_CACHE_SIZE' '=' value ) )
```

This syntax changes the properties of a particular column of a table. Currently it is possible to modify MINMAX CACHE attributes and NOT NULL constraints for column lengths and other types of VARCHAR types.

**VARCHAR SIZE**

This syntax supports changing the column length of VARCHAR type only. This operation can not be reduced in length to preserve existing data, and should always be increased.

```sql
ALTER TABLE table_name MODIFY COLUMN (column_name VARCHAR(new_size));
```

```sql
-- Example: Assume TABLE is created like this.
-- create table atest5 (id integer, name varchar(5), id3 double, id4 float);
 
-- Error occurred: Can not change to another type,
alter table atest5 modify column (id varchar(10));
 
-- Error occurred: VARCHAR length can not be made smaller.
alter table atest5 modify column (name varchar(3));
 
-- Error occurred: Maximum size of VARCHAR can not exceed 32767.
alter table atest5 modify column (name varchar(32768));
 
-- Success
alter table atest5 modify column (name varchar(128));
```

**MINMAX_CACHE_SIZE**

This syntax changes MINMAX_CACHE_SIZE for a particular column.

```sql
ALTER TABLE table_name MODIFY COLUMN column_name SET MINMAX_CACHE_SIZE=value;
```

```sql
-- Example: Assume TABLE is created like this.
create table atest9 (id integer, name varchar(100));
 
-- Error: Does not apply to VARCHAR.
alter table atest9 modify column name set minmax_cache_size=0;
[ERR-02139 : MINMAX CACHE is not allowed for VARCHAR column(NAME).]
 
-- Change success
alter table atest9 modify column id set minmax_cache_size=10240;
```

**NOT NULL**

Adds a NOT NULL constraint to the column. If you add a NOT NULL constraint, the DDL operation fails for columns with NULL values.
If you want to allow NULL values ​​in a column, use the MODIFY COLUMN NULL command in the next section.

```sql
ALTER TABLE table_name MODIFY COLUMN column_name NOT NULL;
```

```sql
-- Add NOT NULL constraint to t1.c1.
alter table t1 modify column c1 not null;
```

**NULL**

Releases the NOT NULL constraint. Performance improvement due to min_max cache of LSM index can not be obtained. NULL values ​​can be input.

```sql
ALTER TABLE table_name MODIFY COLUMN column_name NULL;
```

```sql
-- Release NOT NULL constraint at t1.c1.
alter table t1 modify column c1 null;
```

### ALTER TABLE RENAME TO

**alter_table_rename_stmt:**

```sql
alter_table_rename_stmt ::= 'ALTER TABLE' table_name 'RENAME TO' new_name
```

Changes the name of the table.

Metatables can not be renamed, and you can not use the $ character in the name to be changed. Table renaming is only possible for Log tables.

```sql
-- Change the name of worker table to employee.
ALTER TABLE worker RENAME TO employee;
```

### ALTER TABLE ADD RETENTION

**alter_table_add_retention_stmt:**

```sql
alter_table_add_retention_stmt ::=  'ALTER TABLE' table_name 'ADD RETENTION' policy_name
```

```sql
ALTER TABLE tag ADD RETENTION policy_1d_1h;
```

### ALTER TABLE DROP RETENTION

**alter_table_drop_retention_stmt:**

```sql
alter_table_drop_retention_stmt ::=  'ALTER TABLE' table_name 'DROP RETENTION'
```

```sql
ALTER TABLE tag DROP RETENTION;
```

## ALTER TABLESPACE

The ALTER TABLESPACE statement is used to change the information associated with the specified tablespace.

### ALTER TABLESPACE MODIFY DATADISK

This syntax is used to change the properties of DATADISK in Tablespace.

**alter_tablespace_stmt:**

```sql
alter_tablespace_stmt ::= 'ALTER TABLESPACE' table_name 'MODIFY DATADISK' disk_name 'SET' 'PARALLEL_IO' '=' value
```

```sql
-- Example
ALTER TABLESPACE tbs1 MODIFY DATADISK disk1 SET PARALLEL_IO = 10;
```

## TRUNCATE TABLE

**truncate_table_stmt:**

```sql
truncate_table_stmt ::= 'TRUNCATE TABLE' table_name
```

```sql
-- Delete all data in ctest table.
Mach> truncate table ctest;
Truncated successfully.
```

Deletes all data in the specified table. However, if there is another session in which the table is being searched, it fails with an error.

## CREATE ROLLUP

**create_rollup_stmt:**

```sql
create_rollup_stmt ::= 'CREATE ROLLUP' rollup_name 'ON' src_table_name '('src_table_column')' 'INTERVAL' number ('SEC' | 'MIN' | 'HOUR')
```

```sql
-- Creates a rollup targeting the value column of the tag table.
Mach> CREATE ROLLUP _rollup_tag_value_sec ON tag(value) INTERVAL 1 SEC;
Executed successfully
```

## DROP ROLLUP

**drop_rollup_stmt:**

```sql
drop_rollup_stmt ::= 'DROP ROLLUP' rollup_name
```

```
-- drop rollup.
Mach> DROP ROLLUP _rollup_tag_value_sec;
Executed successfully
```

## CREATE RETENTION

**create_retention_stmt:**

```sql
create_retention_stmt ::= 'CREATE RETENTION' policy_name 'DURATION' duration ( 'MONTH' | 'DAY' ) 'INTERVAL' interval ( 'DAY' | 'HOUR' )
```

```sql
Mach> CREATE RETENTION policy_1d_1h DURATION 1 DAY INTERVAL 1 HOUR;
Executed successfully
```

## DROP RETENTION

**drop_retention_stmt:**

```sql
drop_retention_stmt ::= 'DROP RETENTION' policy_name
```

```sql
Mach> DROP RETENTION policy_1d_1h;
Executed successfully
```
