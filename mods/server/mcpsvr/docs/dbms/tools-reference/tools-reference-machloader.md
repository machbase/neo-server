# machloader

machloader is used to import/export text file data to the Machbase server. It works with CSV files by default, but it also supports other formats.

The features of machloader are as follows.

* machloader can specify a datetime type in the schema file. The datetime type specified must be of the type supported by the Machbase server. One datetime type can be applied to all fields, and each field can have a different format.
* To delete and input the input target table data, use the "-m replace" option.
* machloader does not verify the schema and data file consistency. The user must check that the schema, tables, and data files meet the consistency.
* machloader supports APPEND mode by default.
* machloader does not use the `_ARRIVAL_TIME` column by default. You must use the "-a" option to import/export the corresponding column data.

The options for machloader can be seen with the following command:

```bash
[mach@localhost]$ machloader -h
```

|Option| Description|
|--|--|
|-s, --server=SERVER|Enters Machbase server IP address (default: 127.0.0.1)|
|-u, --user=USER|Enters connecting user name (default: SYS)|
|-p, --password=PASSWORD|Connecting user password (default: MANAGER)|
|-P, --port=PORT|Machbase server port number (default: 5656)|
|-i, --import|Data import command option|
|-o, --export|Data export command option|
|-c, --schema|Command option to create schema file using the database table information|
|-t, --table=TABLE_NAME|Sets table name that is creating a schema file|
|-f, --form=SCHEMA_FORM_FILE|Specifies schema filename|
|-d, --data=DATA_FILE|Specifies a data file name|
|-l, --log=LOG_FILE|Specifies a machloader execution log file|
|-b, --bad=BAD_FILE|Records the data in which the input error occurred and specifies the file name that records the error description when executing -i option.|
|-m, --mode=MODE|Indicates import method when executing -i option. The append or replace option is available. Append enters the data after the existing data and replace deletes the existing data and enters the data.|
|-D, –delimiter=DELIMITER|Sets each field delimiter. The default value is ','.|
|-n, --newline=NEWLINE|Sets each record separator. The default is '\n'.|
|-e, --enclosure=ENCLOSURE|Sets the enclosing delimiter for each field.|
|-r, --format=FORMAT|Specifies the format for file input/output. (default: csv)|
|-a, --atime|Determines whether to use the built-in column `_ARRIVAL_TIME`. The default value is to not use the column.|
|-z, --timezone|Set timezone ex) +0900 -1230|
|-I, --silent| Does not display copyright-related output and import/export status information.|
|-h, --help	| Displays a list of options.|
|-F, --dateformat=DATEFORMAT| Sets the column dateformat. (`_arrival_time YYYY-MM-DD HH24:MI:SS`)<br> If you set 'unixtimestamp' instead of dateformat, the input value is regarded as the unix timestamp value. ("time_column unixtimestamp")<br> If you set 'nanotimestamp' instead of dateformat, the input value is regarded as a timestamp value in nanoseconds. ("time_column nanotimestamp")|
|-E, --encoding=CHARACTER_SET| Sets the encoding of input/output files. Supported encodings are UTF8 (default), ASCII, MS949, KSC5601, EUCJP, SHIFTJIS, BIG5, GB231280, and UTF16.|
|-C, --create| Creates a table if one does not exist upon import.|
|-H, --header|Sets whether header information is present upon import/export. The default value is unset.|
|-S, --slash|Specifies the backslash delimiter.|

The detailed usages are as follows.

## CSV File Import

Imports CSV file to Machbase server.

Option:

```
-i: import specification options
-d: data file naming options
-t: table name specification option
```

Example:

```
machloader -i -d data.csv -t table_name
```

## CSV File Export

Writes data to a CSV file.

Option:

```
-o: export specification options
-d: data file naming options
-t: table name specification option
```

Example:

```
machloader -o -d data.csv -t table_name
```

## Use CSV File Header

The header-related setting of the CSV file.

Option:

```
-i -H: Upon import, the first line of the csv file is recognized as a header. Therefore, the first line is excluded from input.
-o -H: Upon export, generates the csv header as the column name of the table.e
```

Example:

```
machloader -i -d data.csv -t table_name -H
machloader -o -d data.csv -t table_name -H
```

## Automatic Table Creation

Regards automatic table creation.

Option:

```
-C: Automatically generates the table when importing. The column names are automatically generated as c0, c1, .... The generated column is varchar (32767) type.
-H: Generates column names with csv header name when importing.
```

Example:

```
machloader -i -d data.csv -t table_name -C
machloader -i -d data.csv -t table_name -C -H
```

## Files Not CSV Format

Sets delimiter for files that are not in CSV format.

Option:

```
-D: Delimiter option for each field
-n: Specifies each record delimiter option
-e: Specifies the enclosing character for each field.
```

Example:

```
machloader -i -d data.txt -t table_name -D '^' -n '\n' -e '"'
machloader -o -d data.txt -t table_name -D '^' -n '\n' -e '"'
```

## Specify Input Mode

When importing (with -i option), there are two modes, REPLACE and APPEND. APPEND is the default. Use REPLACE mode with caution because it deletes existing data.

Option:

```
-m: Specifies import mode
```

Example:

```
machloader -i -d data.csv -t table_name -m replace
```

## Specify Connection Information

Specifies server IP, user, and password separately.

Option:

```
-s: Specifies server IP address (default: 127.0.0.1)
-P: Specifies server port number (default: 5656)
-u: Specifies the connecting user name (default: SYS)
-p: Specifies the password of the connecting user (default: MANAGER)
```

Example:

```
machloader -i -s 192.168.0.10 -P 5656 -u mach -p machbase -d data.csv -t table_name
```

## Create Log File

Creates the execution log file for machloader.

Option:

```
-b: Sets the name of the log file to generate the data that is not input when importing.
-l: Sets the name of the log file to generate the data and error message that were not input when importing.
```

Example:

```
machloader -i -d data.csv -t table_name -b table_name.bad -l table_name.log
```

## Create Schema File

The machloader schema file can be created. Import/export is possible even if the data type format is changed using a schema file or the number of columns in the table and data file is different.

Option:

```
-c: schema file creation options
-t: table name specification option
-f: created schema file name specification option
```

Example:

```
machloader -c -t table_name -f table_name.fmt
machloader -c -t table_name -f table_name.fmt -a
```

## Set datetime Format in Schema File

The date format can be set to preference with the DATEFORMAT option.

Syntax:

```
## Set for all datetime columns.
DATEFORMAT <dateformat>
```
## Set for individual datetime column.

```
DATEFORMAT <column_name> <format>
```

Example:

```
-- Set dateformat for each field in datetest.csv file in the schema file (datetest.fmt).
datetest.fmt
table datetest
{
INS_DT datetime;
UPT_DT datetime;
}
DATEFORMAT ins_dt "YYYY/MM/DD HH12:MI:SS"
DATEFORMAT upt_dt "YYYY DD MM HH12:MI:SS"
 
datetest.csv
2017/02/20 11:05:23,2017 20 02 11:05:23
2017/02/20 11:06:34,2017 20 02 11:06:34
 
-- Import datetest.csv file and check input data.
machloader -i -f datetest.fmt -d datetest.csv
-----------------------------------------------------------------
Machbase Data Import/Export Utility.
Release Version 5.1.9.community
Copyright 2014, MACHBASE Corporation or its subsidiaries.
All Rights Reserved.
-----------------------------------------------------------------
Import time : 0 hour 0 min 0.39 sec
Load success count : 2
Load fail count : 0
 
mach> SELECT * FROM datetest;
INS_DT UPT_DT
-------------------------------------------------------------------
2017-02-20 11:06:34 000:000:000 2017-02-20 11:06:34 000:000:000
2017-02-20 11:05:23 000:000:000 2017-02-20 11:05:23 000:000:000
[2] row(s) selected.
Elapsed time: 0.000
```

## IGNORE

When you do not want to enter a specific field in the CSV file, you can set the IGNORE option in the fmt file.
The ignoretest.csv file has three fields, but if the last field is not needed, specify IGNORE in the column that is not needed in the fmt file.

Example:

```
-- Set ignore option for last field in ignoretest.fmt file.
ignoretest.fmt
table ignoretest
{
ID integer;
MSG varchar(40);
SUB_ID integer IGNORE;
}
 
ignoretest.csv
1, "msg1", 3
2, "msg2", 4
 
 
-- Import ignoretest.csv file and check input data.
machloader -i -f ignoretest.fmt -d ignoretest.csv
-----------------------------------------------------------------
Machbase Data Import/Export Utility.
Release Version 5.1.9.community
Copyright 2014, MACHBASE Corporation or its subsidiaries.
All Rights Reserved.
-----------------------------------------------------------------
NLS : US7ASCII EXECUTE MODE : IMPORT
SCHMEA FILE : ignoretest.fmt DATA FILE : ignoretest.csv
IMPORT_MODE : APPEND FILED TERM : ,
ROW TERM : \n ENCLOSURE : "
ARRIVAL_TIME : FALSE ENCODING : NONE
HEADER : FALSE CREATE TABLE : FALSE
 
Progress bar Imported records Error records
2 0
 
Import time : 0 hour 0 min 0.39 sec
Load success count : 2
Load fail count : 0
 
 
mach> SELECT * FROM ignoretest;
ID MSG
---------------------------------------------------------
2 msg2
1 msg1
[2] row(s) selected.
Elapsed time: 0.000
```

## If Number of Columns Is More Than Number of Fields

If the number of columns in the table is greater than the number of fields in the data file, only the columns specified in the schema file are entered, and the other columns are entered as NULL.

## If Number of Columns Is Less Than Number of Fields

If the number of columns in the table is less than the number of fields in the data file, fields not in the table must be excluded with the IGNORE option

Example:

```
-- Import ignoretest.csv file and exclude input data by setting ignore option for last field.
loader_test.fmt
table loader_test
{
ID integer;
MSG varchar (40);
SUB_ID integer IGNORE;
}
```
