# Load by SQL

The 'Load Data' statement puts the data in the csv file into Machbase.

First, create a table to store the data, using the first line of the csv file to create the columns.

* The data type of the generated columns is VARCHAR (32768).
* The data file path is a relative path based on $MACHBASE_HOME. It can also be set to an absolute path.

To save the table data as csv file, use the SAVE DATA statement.

If you create the table in advance, the data type for each field in the CSV file must be set as VARCHAR or TEXT.

If you enter the file 'load_sample.csv' into the LOAD DATA statement, the table 'load_sample' is automatically created.

## Loading Data

```sql
LOAD DATA INFILE 'sample/quickstart/load_sample.csv' INTO TABLE load_sample AUTO HEADUSE;
```

## Confirm Data Loading

```sql
SELECT * FROM load_sample;
```

## Sample Example

Using the sample file, you can do the following.

```bash
[mach@localhost ~]$ cd $MACHBASE_HOME/sample/quickstart
[mach@localhost ~]$ ls -l load_sample.csv
-rw-r
--r--- 1 root root 2827 2017-02-23 15:01 load_sample.csv
 
[mach@localhost ~]$ machsql
=================================================================
     Machbase Client Query Utility
     Release Version x.x.x.official
     Copyright 2014, Machbase Inc. or its subsidiaries.
     All Rights Reserved
=================================================================
Machbase server address (Default:127.0.0.1) :
Machbase user ID  (Default:SYS)
Machbase User Password :
MACH_CONNECT_MODE=INET, PORT=5656
 
Mach> LOAD DATA INFILE 'sample/quickstart/load_sample.csv' INTO TABLE load_sample AUTO HEADUSE;
50 row(s) loaded. Failed to load 0 row(s).
Mach> DESC load_sample;
----------------------------------------------------------------
NAME                          TYPE                LENGTH
----------------------------------------------------------------
SENSOR_ID                     varchar             32767
EPOCH_TIME                    varchar             32767
E_YEAR                        varchar             32767
E_MONTH                       varchar             32767
E_DAY                         varchar             32767
E_HOUR                        varchar             32767
E_MINUTE                      varchar             32767
E_SECOND                      varchar             32767
VALUE                         varchar             32767
Mach> SELECT COUNT(*) FROM load_sample;
COUNT(*)
-----------------------
50
[1] row(s) selected.
Mach>
```
