# machsql

machsql is an interactive tool that performs SQL queries through the terminal screen.

## Run Option Description

```
[mach@localhost]$ machsql -h
```

|Short Option|Full Option| Description|
|--|--|--|
|-s | --server | Connecting server IP address (default: 127.0.0.1)|
|-u | --user | User name (default: SYS)|
|-p | --password | User password (default: MANAGER)|
|-P | --port | Server port number (default: 5656)|
|-n | --nls | NLS configuration|
|-f | --script | SQL script file to run|
|-z | --timezone=+-HHMM | Set Timezone ex) +0900   -1230|
|-o | --output | Filename to save query results|
|-i | --silent | Runs without the copyright notice|
|-v | --verbose | Detailed output|
|-r | --format | Specifies output file format (default: csv)|
|-h | --help | Displays options|
|-c | N/A | Add Connection parameter(Supported from version 6.1 or later)|

Example:

```
machsql -s localhost -u sys -p manager
machsql --server=localhost --user=sys --password=manager
machsql -s localhost -u sys -p manager -f script.sql
## Supported from version 6.1 or later
machsql -s 127.0.0.1 -u sys -p manager -P 8888 -c ALTERNATIVE_SERVERS=192.168.0.147:9209;CONNECTION_TIMEOUT=10
```

## Environment Variable MACHBASE_CONNECTION_STRING

Specifies basic connection parameters. For example, to add CONNECTION_TIMEOUT, ALTERNATIVE_SERVERS, you may use environment variable setting below.

```
export MACHBASE_CONNECTION_STRING=ALTERNATIVE_SERVERS=192.168.0.148:8888;CONNECTION_TIMEOUT=3
```

Setting connection parameter with -c option, it takes precedence over environment variables. This option is supported from version 6.1 or later

## Using HEREDOC for SQL Scripts

machsql supports HEREDOC (Here Document) syntax, allowing you to pass SQL commands directly from the shell without creating a separate file. This is particularly useful for automation scripts and one-time SQL execution.

> **Note**: This feature is supported from Machbase version 8.0.50 or later.

### Basic Syntax

```bash
machsql -s <server> -u <user> -p <password> <<'DELIMITER'
SQL statements here
DELIMITER
```

The delimiter can be any string (commonly `EOF`, `SQL`, or `SQLBLOCK`). Using quotes around the delimiter (`<<'DELIMITER'`) prevents shell variable expansion.

### Examples

**Simple query execution:**

```bash
machsql -s 127.0.0.1 -u sys -p manager <<'SQLBLOCK'
select 'WORKS!!!!' from v$tables limit 2;
SQLBLOCK
```

**Multiple statements:**

```bash
machsql -s 127.0.0.1 -u sys -p manager <<'EOF'
CREATE TABLE test_table (id INTEGER, name VARCHAR(100));
INSERT INTO test_table VALUES (1, 'First Record');
INSERT INTO test_table VALUES (2, 'Second Record');
SELECT * FROM test_table;
DROP TABLE test_table;
EOF
```

**Using variables (without quotes on delimiter):**

```bash
TABLE_NAME="my_table"
machsql -s 127.0.0.1 -u sys -p manager <<EOF
SELECT COUNT(*) FROM ${TABLE_NAME};
EOF
```

**With output redirection:**

```bash
machsql -s 127.0.0.1 -u sys -p manager <<'SQL' > output.csv
SELECT name, time, value FROM tag_table
WHERE time >= NOW - INTERVAL 1 HOUR
ORDER BY time DESC;
SQL
```

### Benefits of HEREDOC

1. **No temporary files**: Execute SQL without creating separate script files
2. **Inline scripts**: Embed SQL directly in shell scripts for better readability
3. **Automation**: Simplify deployment and maintenance scripts
4. **Variable substitution**: Use shell variables in SQL when needed (without quotes)

### Notes

- Use quotes around the delimiter (`<<'DELIMITER'`) to prevent variable expansion
- Remove quotes (`<<DELIMITER`) if you want to use shell variables in your SQL
- The delimiter must appear alone on a line to terminate the HEREDOC
- Works with all machsql command-line options

## SHOW Command

Displays information such as tables, tablespaces, and indexes.

SHOW command list:

* SHOW INDEX
* SHOW INDEXES
* SHOW INDEXGAP
* SHOW LSM
* SHOW LICENSE
* SHOW STATEMENTS
* SHOW STORAGE
* SHOW TABLE
* SHOW TABLES
* SHOW TABLESPACE
* SHOW TABLESPACES
* SHOW USERS

### SHOW INDEX
Displays index information.

Syntax:

```
SHOW INDEX index_name
```

Example:

```
Mach> CREATE TABLE t1 (c1 INTEGER, c2 VARCHAR(10));
Created successfully.
Mach> CREATE VOLATILE TABLE t2 (c1 INTEGER, c2 VARCHAR(10));
Created successfully.
Mach> CREATE INDEX t1_idx1 ON t1(c1) INDEX_TYPE LSM;
Created successfully.
Mach> CREATE INDEX t1_idx2 ON t1(c1) INDEX_TYPE BITMAP;
Created successfully.
Mach> CREATE INDEX t2_idx1 ON t2(c1) INDEX_TYPE REDBLACK;
Created successfully.
Mach> CREATE INDEX t2_idx2 ON t2(c2) INDEX_TYPE REDBLACK;
Created successfully.
 
Mach> SHOW INDEX t1_idx2;
TABLE_NAME                                COLUMN_NAME                               INDEX_NAME                      
----------------------------------------------------------------------------------------------------------------------------------
INDEX_TYPE   BLOOM_FILTER  KEY_COMPRESS  MAX_LEVEL   PART_VALUE_COUNT BITMAP_ENCODE
--------------------------------------------------------------------------------------------
T1                                        C1                                        T1_IDX2                         
LSM          ENABLE   COMPRESSED    2           100000      EQUAL
[1] row(s) selected.
```

### SHOW INDEXES

Displays entire index list.

**Syntax:**

```
SHOW INDEXES
```

**Example:**

```sql
Mach> CREATE TABLE t1 (c1 INTEGER, c2 VARCHAR(10));
Created successfully.
Mach> CREATE VOLATILE TABLE t2 (c1 INTEGER, c2 VARCHAR(10));
Created successfully.
Mach> CREATE INDEX t1_idx1 ON t1(c1) INDEX_TYPE LSM;
Created successfully.
Mach> CREATE INDEX t1_idx2 ON t1(c1) INDEX_TYPE BITMAP;
Created successfully.
Mach> CREATE INDEX t2_idx1 ON t2(c1) INDEX_TYPE REDBLACK;
Created successfully.
Mach> CREATE INDEX t2_idx2 ON t2(c2) INDEX_TYPE REDBLACK;
Created successfully.
 
Mach> SHOW INDEXES;
TABLE_NAME                                COLUMN_NAME                               INDEX_NAME                      
----------------------------------------------------------------------------------------------------------------------------------
INDEX_TYPE
---------------
T1                                        C1                                        T1_IDX1                         
LSM
T1                                        C1                                        T1_IDX2                         
LSM
T2                                        C2                                        T2_IDX2                         
REDBLACK
T2                                        C1                                        T2_IDX1                         
REDBLACK
[4] row(s) selected.
```

### SHOW INDEXGAP

Displays index building GAP information.

Example:

```
Mach> SHOW INDEXGAP
TABLE_NAME                                INDEX_NAME                                GAP
-------------------------------------------------------------------------------------------------------------
INDEX_TABLE                               T1_IDX1                                   0
INDEX_TABLE                               T1_IDX2                                   0
```

### SHOW LSM

Displays LSM index building information.

Example:

```
Mach> SHOW LSM;
TABLE_NAME                                INDEX_NAME                                LEVEL       COUNT
--------------------------------------------------------------------------------------------------------------------------
T1                                        IDX1                                      0           0
T1                                        IDX1                                      1           100000
T1                                        IDX1                                      2           0
T1                                        IDX1                                      3           0
T1                                        IDX2                                      0           100000
T1                                        IDX2                                      1           0
[6] row(s) selected.
```

### SHOW LICENSE

Displays license information.

Example:

```
Mach> SHOW LICENSE
INSTALL_DATE          ISSUE_DATE            EXPIRY_DATE  TYPE        POLICY    
---------------------------------------------------------------------------------------
2016-07-01 10:24:37   20160325              20170325    2           0         
[1] row(s) selected.
```

### SHOW STATEMENTS

Displays all query statements (Prepare, Execute, Fetch) registered in the server.

Example:

```
Mach> SHOW STATEMENTS
USER_ID     SESSION_ID  QUERY                                                                           
--------------------------------------------------------------------------------------------------------------
0           2           SELECT ID USER_ID, SESS_ID SESSION_ID, QUERY FROM V$STMT                        
[1] row(s) selected.
```

### SHOW STORAGE

Displays the disk usage for each table created by the user.

Syntax:

```
SHOW STORAGE
```

Example:

```
Mach> CREATE TAGDATA TABLE TAG (name varchar(20) primary key, time datetime basetime, value double summarized);
Created successfully.
  
Mach> SHOW STORAGE
TABLE_NAME                                          DATA_SIZE            INDEX_SIZE           TOTAL_SIZE         
------------------------------------------------------------------------------------------------------------------------ 
_TAG_DATA_0                                         50335744             0                    50335744           
_TAG_DATA_1                                         50335744             0                    50335744           
_TAG_DATA_2                                         50335744             0                    50335744           
_TAG_DATA_3                                         50335744             0                    50335744           
_TAG_META                                           0                    0                    0
```

### SHOW TABLE

Displays information about the table created by the user.

Syntax:

```
SHOW TABLE table_name
```

Example:

```
Mach> CREATE TABLE t1 (c1 INTEGER, c2 VARCHAR(10));
Created successfully.
Mach> CREATE INDEX t1_idx1 ON t1(c1) INDEX_TYPE LSM;
Created successfully.
Mach> CREATE INDEX t1_idx2 ON t1(c1) INDEX_TYPE BITMAP;
Created successfully.
 
Mach> SHOW TABLE T1
[ COLUMN ]
----------------------------------------------------------------
NAME                          TYPE                LENGTH
----------------------------------------------------------------
C1                            integer             11
C2                            varchar             10
 
[ INDEX ]
----------------------------------------------------------------
NAME                          TYPE                COLUMN
----------------------------------------------------------------
T1_IDX1                       LSM                 C1
T1_IDX2                       LSM                 C1
```

### SHOW TABLES

Displays a list of all tables created by the user.

Example:

```
Mach> SHOW TABLES
NAME                                    
--------------------------------------------
BONUS                                   
DEPT                                    
EMP                                     
SALGRADE                                
[4] row(s) selected.
```

### SHOW TABLESPACE

Displays tablespace information.

Example:

```
Mach> CREATE TABLE t1 (id integer);
Created successfully.
Mach> CREATE INDEX t1_idx_id ON t1(id);
Created successfully.
 
Mach> SHOW TABLESPACE SYSTEM_TABLESPACE;
[TABLE]
NAME                                      TYPE
-------------------------------------------------------
T1                                        LOG
[1] row(s) selected.
 
[INDEX]
TABLE_NAME                                COLUMN_NAME                               INDEX_NAME                      
----------------------------------------------------------------------------------------------------------------------------------
T1                                        ID                                        T1_IDX_ID                   
[1] row(s) selected.
```

### SHOW TABLESPACES

Displays a complete list of tablespaces.

Example:

```
Mach> CREATE TABLESPACE tbs1 DATADISK disk1 (DISK_PATH="tbs1_disk1"), disk2 (DISK_PATH="tbs1_disk2"), disk3 (DISK_PATH="tbs1_disk3");
Created successfully.
 
-- Insert data here
...
...
 
 
Mach> SHOW TABLESPACES;
NAME                                                                              DISK_COUNT  USAGE
-----------------------------------------------------------------------------------------------------------------------
SYSTEM_TABLESPACE                                                                 1           0
TBS1                                                                              3           25824256
[2] row(s) selected.
```

### SHOW USERS

Displays a list of users.

Example:

```
Mach> CREATE USER testuser IDENTIFIED BY 'test1234';
Created successfully.
 
Mach> SHOW USERS;
USER_NAME                               
--------------------------------------------
SYS                                     
TESTUSER
[2] row(s) selected.
```
