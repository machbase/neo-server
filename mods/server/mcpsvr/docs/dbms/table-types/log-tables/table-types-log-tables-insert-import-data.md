# Import

With the machloader tool, you can enter a text file that is separated by a CSV or other delimiter.

See the [machloader](/dbms/tools/machloader) documentation for a detailed description of the machloader tool.

## Index

* [Importing Data](#importing-data)
* [Confirm Data Insert](#confirm-data-insert)
* [Sample Example](#sample-example)

## Create Table

```sql
CREATE TABLE import_sample
(
    srcip     IPV4,
    srcport   INTEGER,
    dstip     IPV4,
    dstport   INTEGER,
    protocol  SHORT,
    eventlog  VARCHAR(1024),
    eventcode SHORT,
    eventsize LONG
);
```

## Importing Data

Use the machloader tool to enter the csv file.

```bash
machloader  -i  -t  import_sample   -d  sample_data.csv
```

## Confirm Data Insert 

Check the input data.

``` sql
SELECT  COUNT(*)    FROM    import_sample;
```

## Sample Example

Below is a sample process using the actual machloader and machsql.

```sql
Mach> CREATE TABLE import_sample
     (
         srcip     IPV4,
         srcport   INTEGER,
         dstip     IPV4,
         dstport   INTEGER,
         protocol  SHORT,
         eventlog  VARCHAR(1024),
         eventcode SHORT,
         eventsize LONG
     );
Created successfully.
Mach> quit
```

```bash
[mach@localhost ~]$ cd $MACHBASE_HOME/sample/quickstart
[mach@localhost ~]$ ls -l sample_data.csv
-rw-r--r--- 1 mach mach 110477124 2017-02-23 15:18 sample_data.csv
 
[mach@localhost ~]$ machloader -i -t import_sample -d sample_data.csv
-----------------------------------------------------------------
     Machbase Data Import/Export Utility.
     Release Version x.x.x.official
     Copyright 2014, Machbase Inc. or its subsidiaries.
     All Rights Reserved.
-----------------------------------------------------------------
NLS            : US7ASCII            EXECUTE MODE   : IMPORT
TARGET TABLE   : import_sample
DATA FILE      : sample_data.csv
IMPORT_MODE    : APPEND
FILED TERM     : ,                   ROW TERM       : \n
ENCLOSURE      : "                   ARRIVAL_TIME   : FALSE
ENCODING       : NONE                HEADER         : FALSE
CREATE TABLE   : FALSE
 Progress bar                       Imported records        Error records
                                             1000000                    0
Import time         :  0 hour  0 min  2.39 sec
Load success count  : 1000000
Load fail count     : 0
[mach@localhost ~]$
```

```sql
Mach> SELECT COUNT(*) FROM import_sample;
COUNT(*)
-----------------------
1000000
[1] row(s) selected.
Mach>
```
