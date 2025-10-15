# Sample for using stream

##  Download Sample Data

Download sample data by following guide.

```bash
## ##  1. Clone the sample data from MACHBASE git repository.
$ git clone https://www.github.com/MACHBASE/TagTutorial.git MyTutorial
 
## ##  2. Unzip the data you need.
$ cd MyTutorial/
$ gunzip edu_3_plc_stream/*.gz
 
## ##  3. Move to the directory where the sample data exist.
$ cd edu_3_plc_stream/
```

##  Create TAG, LOG Table

To use the STREAM function, modify the following commands according to the environment and execute them to create TAG and LOG tables.

```bash
$ pwd
~/MyTutorial/edu_3_plc_stream
 
## ##  1-1. Create TAG table
$ machsql --server=127.0.0.1 --port=${MACHBASE_PORT_NO} --user=SYS --password=MANAGER --script=1_create_tag.sql
## ##  1-2. Load TAG Meta
$ sh 2_load_meta.sh
 
## ##  2. Create LOG table
$ machsql --server=127.0.0.1 --port=${MACHBASE_PORT_NO} --user=SYS --password=MANAGER --script=3_create_plc_tag_table.sql
```

##  Create and Run STREAM

Execute sample file to run STREAM that are Start STREAM by executing the sample file made for the created TAG and LOG tables.

```bash
$ machsql --server=127.0.0.1 --port=${MACHBASE_PORT_NO} --user=SYS --password=MANAGER --script=4_plc_stream_tag.sql
```

There are two types of query in the sample file, one create STREAM, the other one run STREAM.

```sql
## ##  Create STREAM Query Example
EXEC STREAM_CREATE(event_v0, 'insert into tag select ''MTAG_V00'', tm, v0 from plc_tag_table;');

## ##  Run STREAM Query Example
EXEC STREAM_START(event_v0);
```

If STREAM run normally, when data are inserted to plc_tag_table, every STREAM runs to insert that data to TAG table.

##  Check STREAM Status

Through v$streams, a virtual table supported by Machbase, you can check the number of streams being executed, queries used, status, and error messages.

```sql
Mach> desc v$streams;
[ COLUMN ]
----------------------------------------------------------------------------------------------------
NAME                                                        NULL?    TYPE                LENGTH
----------------------------------------------------------------------------------------------------
NAME                                                                 varchar             100
LAST_EX_TIME                                                         datetime            31
TABLE_NAME                                                           varchar             100
END_RID                                                              long                20
STATE                                                                varchar             10
QUERY_TXT                                                            varchar             2048
ERROR_MSG                                                            varchar             2048
FREQUENCY                                                            ulong               20
```

Checking all of the STREAM status is available like below.

```sql
Mach> select state, name, table_name, query_txt from v$streams;
STATE   NAME     TABLE_NAME    QUERY_TXT
------------------------------------------------------------------------------------------------
RUNNING EVENT_V0 PLC_TAG_TABLE insert into tag select 'MTAG_V00', tm, v0 from plc_tag_table;
RUNNING EVENT_V1 PLC_TAG_TABLE insert into tag select 'MTAG_V00', tm, v1 from plc_tag_table;
RUNNING EVENT_C0 PLC_TAG_TABLE insert into tag select 'MTAG_C00', tm, c0 from plc_tag_table;
RUNNING EVENT_C1 PLC_TAG_TABLE insert into tag select 'MTAG_C01', tm, c1 from plc_tag_table;
RUNNING EVENT_C2 PLC_TAG_TABLE insert into tag select 'MTAG_C02', tm, c2 from plc_tag_table;
RUNNING EVENT_C3 PLC_TAG_TABLE insert into tag select 'MTAG_C03', tm, c3 from plc_tag_table;
RUNNING EVENT_C4 PLC_TAG_TABLE insert into tag select 'MTAG_C04', tm, c4 from plc_tag_table;
RUNNING EVENT_C5 PLC_TAG_TABLE insert into tag select 'MTAG_C05', tm, c5 from plc_tag_table;
RUNNING EVENT_C6 PLC_TAG_TABLE insert into tag select 'MTAG_C06', tm, c6 from plc_tag_table;
RUNNING EVENT_C7 PLC_TAG_TABLE insert into tag select 'MTAG_C07', tm, c7 from plc_tag_table;
RUNNING EVENT_C8 PLC_TAG_TABLE insert into tag select 'MTAG_C08', tm, c8 from plc_tag_table;
RUNNING EVENT_C9 PLC_TAG_TABLE insert into tag select 'MTAG_C09', tm, c9 from plc_tag_table;
RUNNING EVENT_C10 PLC_TAG_TABLE insert into tag select 'MTAG_C10', tm, c10 from plc_tag_table;
RUNNING EVENT_C11 PLC_TAG_TABLE insert into tag select 'MTAG_C11', tm, c11 from plc_tag_table;
RUNNING EVENT_C12 PLC_TAG_TABLE insert into tag select 'MTAG_C12', tm, c12 from plc_tag_table;
RUNNING EVENT_C13 PLC_TAG_TABLE insert into tag select 'MTAG_C13', tm, c13 from plc_tag_table;
RUNNING EVENT_C14 PLC_TAG_TABLE insert into tag select 'MTAG_C14', tm, c14 from plc_tag_table;
RUNNING EVENT_C15 PLC_TAG_TABLE insert into tag select 'MTAG_C15', tm, c15 from plc_tag_table;
```

##  Load Data

After confirming that all STREAM is running, input data using Machloader and check the operation.
Since STREAM works regardless of the input method, it will automatically insert into the TAG table regardless of any input methods such as CLI, JDBC, or Collector.

```bash
$ cat 5_plc_tag_load.sh
machloader  -t plc_tag_table -i -d 5_plc_tag.csv -F "tm YYYY-MM-DD HH24:MI:SS mmm:uuu:nnn"
 
$ sh 5_plc_tag_load.sh
-----------------------------------------------------------------
     Machbase Data Import/Export Utility.
     Release Version 6.5.1.official
     Copyright 2014, MACHBASE Corporation or its subsidiaries.
     All Rights Reserved.
-----------------------------------------------------------------
NLS            : US7ASCII            EXECUTE MODE   : IMPORT
TARGET TABLE   : plc_tag_table       DATA FILE      : 5_plc_tag.csv
IMPORT MODE    : APPEND              FIELD TERM     : ,
ROW TERM       : \n                  ENCLOSURE      : "
ESCAPE         : \                   ARRIVAL_TIME   : FALSE
ENCODING       : NONE                HEADER         : FALSE
CREATE TABLE   : FALSE
 
 Progress bar                       Imported records        Error records
                                               80000                    0
```

If you check the TAG table data during data loading, you can see that the data is inserted in real time.

```sql
Mach> select count(*) from TAG;
count(*)
-----------------------
16775979
[1] row(s) selected.
Mach> select count(*) from TAG;
count(*)
-----------------------
17609187
[1] row(s) selected.
Mach> select count(*) from TAG;
count(*)
-----------------------
18238357
[1] row(s) selected.
Elapsed time: 0.000
```

##  Result of STREAM

You can check how far STREAM has read the data of the source table (plc_tag_table) just like below.

```sql
Mach> select name, state, end_rid from v$streams;
name      state   end_rid
---------------------------------------------------------
EVENT_V0  RUNNING 909912
EVENT_V1  RUNNING 1584671
EVENT_C0  RUNNING 1312416
EVENT_C1  RUNNING 1268520
EVENT_C2  RUNNING 1636800
EVENT_C3  RUNNING 1197840
EVENT_C4  RUNNING 622728
EVENT_C5  RUNNING 972780
EVENT_C6  RUNNING 1021512
EVENT_C7  RUNNING 1287474
EVENT_C8  RUNNING 826956
EVENT_C9  RUNNING 1639032
EVENT_C10 RUNNING 725954
EVENT_C11 RUNNING 1511436
EVENT_C12 RUNNING 531079
EVENT_C13 RUNNING 1004400
EVENT_C14 RUNNING 741768
EVENT_C15 RUNNING 746604
[18] row(s) selected.
```

If end_rid column value is same as record number of source table, it means there is nothing more to read in source table;

```sql
Mach> select name, state, end_rid from v$streams;
name      state   end_rid
---------------------------------------------------------
EVENT_V0  RUNNING 2000000
EVENT_V1  RUNNING 2000000
EVENT_C0  RUNNING 2000000
EVENT_C1  RUNNING 2000000
EVENT_C2  RUNNING 2000000
EVENT_C3  RUNNING 2000000
EVENT_C4  RUNNING 2000000
EVENT_C5  RUNNING 2000000
EVENT_C6  RUNNING 2000000
EVENT_C7  RUNNING 2000000
EVENT_C8  RUNNING 2000000
EVENT_C9  RUNNING 2000000
EVENT_C10 RUNNING 2000000
EVENT_C11 RUNNING 2000000
EVENT_C12 RUNNING 2000000
EVENT_C13 RUNNING 2000000
EVENT_C14 RUNNING 2000000
EVENT_C15 RUNNING 2000000
[18] row(s) selected.
```

Since the number of data in the TAG table is the same as 'the number of source tables' * 'the number of STREAMs', it can be confirmed that the STREAM has read all the data normally.

```sql
Mach> select count(*) from TAG;
count(*)
-----------------------
36000000
[1] row(s) selected.
```

You can also check the time range of the input data as follows.

```sql
Mach> select min(time), max(time) from TAG;
min(time)                       max(time)
-------------------------------------------------------------------
2009-01-28 07:03:34 000:000:000 2009-01-28 12:36:58 020:000:000
[1] row(s) selected.
```

##  Add Data

You can check through the insert statement to see if STREAM actually responds to each data input.

```sql
Mach> insert into plc_tag_table values(TO_DATE('2009-01-28 12:37:00 000:000:000'), 50000, 50000, 50000, 50000, 50000, 50000, 50000, 50000, 50000, 50000, 50000, 50000, 50000, 50000, 50000, 50000, 50000, 50000);
1 row(s) inserted.
```

As soon as one more record is added to PLC_TAG_TABLE, it can be seen that the end_rid of each stream increases to 2000001 as shown below.

```sql
Mach> select name, state, end_rid from v$streams;
name      state   end_rid
---------------------------------------------------------------
EVENT_V0  RUNNING 2000001
EVENT_V1  RUNNING 2000001
EVENT_C0  RUNNING 2000001
EVENT_C1  RUNNING 2000001
EVENT_C2  RUNNING 2000001
EVENT_C3  RUNNING 2000001
EVENT_C4  RUNNING 2000001
EVENT_C5  RUNNING 2000001
EVENT_C6  RUNNING 2000001
EVENT_C7  RUNNING 2000001
EVENT_C8  RUNNING 2000001
EVENT_C9  RUNNING 2000001
EVENT_C10 RUNNING 2000001
EVENT_C11 RUNNING 2000001
EVENT_C12 RUNNING 2000001
EVENT_C13 RUNNING 2000001
EVENT_C14 RUNNING 2000001
EVENT_C15 RUNNING 2000001
[18] row(s) selected.
```
