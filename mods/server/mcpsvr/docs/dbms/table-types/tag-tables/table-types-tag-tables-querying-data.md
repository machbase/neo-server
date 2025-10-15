# Querying Tag Data

## Overview

Machbase excels at high-speed tag data extraction, especially for time-range queries on specific tags. This guide covers all the query patterns you'll need for working with tag tables.

## Quick Start

Machbase provides high-speed tag data extraction, especially for the time range of a specific tag.

##  Sample Schema

In the following example, we created a TAG table and created two tags as shown below.

For each tag, data from January 1, 2018 to February 10, 2018 were inserted.

```sql
create tag table TAG (name varchar(20) primary key, time datetime basetime, value double summarized);
 
insert into tag metadata values ('TAG_0001');
insert into tag metadata values ('TAG_0002');
 
insert into tag values('TAG_0001', '2018-01-01 01:00:00 000:000:000', 1);
insert into tag values('TAG_0001', '2018-01-02 02:00:00 000:000:000', 2);
insert into tag values('TAG_0001', '2018-01-03 03:00:00 000:000:000', 3);
insert into tag values('TAG_0001', '2018-01-04 04:00:00 000:000:000', 4);
insert into tag values('TAG_0001', '2018-01-05 05:00:00 000:000:000', 5);
insert into tag values('TAG_0001', '2018-01-06 06:00:00 000:000:000', 6);
insert into tag values('TAG_0001', '2018-01-07 07:00:00 000:000:000', 7);
insert into tag values('TAG_0001', '2018-01-08 08:00:00 000:000:000', 8);
insert into tag values('TAG_0001', '2018-01-09 09:00:00 000:000:000', 9);
insert into tag values('TAG_0001', '2018-01-10 10:00:00 000:000:000', 10);
 
insert into tag values('TAG_0002', '2018-02-01 01:00:00 000:000:000', 11);
insert into tag values('TAG_0002', '2018-02-02 02:00:00 000:000:000', 12);
insert into tag values('TAG_0002', '2018-02-03 03:00:00 000:000:000', 13);
insert into tag values('TAG_0002', '2018-02-04 04:00:00 000:000:000', 14);
insert into tag values('TAG_0002', '2018-02-05 05:00:00 000:000:000', 15);
insert into tag values('TAG_0002', '2018-02-06 06:00:00 000:000:000', 16);
insert into tag values('TAG_0002', '2018-02-07 07:00:00 000:000:000', 17);
insert into tag values('TAG_0002', '2018-02-08 08:00:00 000:000:000', 18);
insert into tag values('TAG_0002', '2018-02-09 09:00:00 000:000:000', 19);
insert into tag values('TAG_0002', '2018-02-10 10:00:00 000:000:000', 20);
```

## Extract all TAG data

```bash
Mach> select * from tag;
NAME TIME VALUE
--------------------------------------------------------------------------------------
TAG_0001 2018-01-01 01:00:00 000:000:000 1
TAG_0001 2018-01-02 02:00:00 000:000:000 2
TAG_0001 2018-01-03 03:00:00 000:000:000 3
TAG_0001 2018-01-04 04:00:00 000:000:000 4
TAG_0001 2018-01-05 05:00:00 000:000:000 5
TAG_0001 2018-01-06 06:00:00 000:000:000 6
TAG_0001 2018-01-07 07:00:00 000:000:000 7
TAG_0001 2018-01-08 08:00:00 000:000:000 8
TAG_0001 2018-01-09 09:00:00 000:000:000 9
TAG_0001 2018-01-10 10:00:00 000:000:000 10
TAG_0002 2018-02-01 01:00:00 000:000:000 11
TAG_0002 2018-02-02 02:00:00 000:000:000 12
TAG_0002 2018-02-03 03:00:00 000:000:000 13
TAG_0002 2018-02-04 04:00:00 000:000:000 14
TAG_0002 2018-02-05 05:00:00 000:000:000 15
TAG_0002 2018-02-06 06:00:00 000:000:000 16
TAG_0002 2018-02-07 07:00:00 000:000:000 17
TAG_0002 2018-02-08 08:00:00 000:000:000 18
TAG_0002 2018-02-09 09:00:00 000:000:000 19
TAG_0002 2018-02-10 10:00:00 000:000:000 20
[20] row(s) selected.
```

If there is no special condition as described above, data can be extracted for each tag arranged in each time order.

## Extract data for a specific tag name

Below is an example of data with TAG name TAG_0002.

```sql
Mach> select * from tag where name='TAG_0002';
NAME                  TIME                            VALUE                      
--------------------------------------------------------------------------------------
TAG_0002              2018-02-01 01:00:00 000:000:000 11                         
TAG_0002              2018-02-02 02:00:00 000:000:000 12                         
TAG_0002              2018-02-03 03:00:00 000:000:000 13                         
TAG_0002              2018-02-04 04:00:00 000:000:000 14                         
TAG_0002              2018-02-05 05:00:00 000:000:000 15                         
TAG_0002              2018-02-06 06:00:00 000:000:000 16                         
TAG_0002              2018-02-07 07:00:00 000:000:000 17                         
TAG_0002              2018-02-08 08:00:00 000:000:000 18                         
TAG_0002              2018-02-09 09:00:00 000:000:000 19                         
TAG_0002              2018-02-10 10:00:00 000:000:000 20                         
[10] row(s) selected.
```

## Query for time range

The following is a query of a time range for TAG_0002 and receives data.

> It is common to give time range by using between clause. Of course, using '<' or '>' to get time range will get same result.

```bash
Mach> select * from tag where name = 'TAG_0002' and time between to_date('2018-02-01') and to_date('2018-02-05');
NAME                  TIME                            VALUE                      
--------------------------------------------------------------------------------------
TAG_0002              2018-02-01 01:00:00 000:000:000 11                         
TAG_0002              2018-02-02 02:00:00 000:000:000 12                         
TAG_0002              2018-02-03 03:00:00 000:000:000 13                         
TAG_0002              2018-02-04 04:00:00 000:000:000 14                         
[4] row(s) selected.
 
Mach> select * from tag where name = 'TAG_0002' and time > to_date('2018-02-01') and time < to_date('2018-02-05');
NAME                  TIME                            VALUE                      
--------------------------------------------------------------------------------------
TAG_0002              2018-02-01 01:00:00 000:000:000 11                         
TAG_0002              2018-02-02 02:00:00 000:000:000 12                         
TAG_0002              2018-02-03 03:00:00 000:000:000 13                         
TAG_0002              2018-02-04 04:00:00 000:000:000 14                         
[4] row(s) selected.
```

## Time range search for multiple tags

Below is an example of retrieving the same time range data for two or more tags.

If you want to get fast results for a large number of tags at the same time, it is preferable to perform the following type of query.

```bash
Mach> select * from tag where name in ('TAG_0002', 'TAG_0001') and time between to_date('2018-01-05') and to_date('2018-02-05');
NAME                  TIME                            VALUE                      
--------------------------------------------------------------------------------------
TAG_0001              2018-01-05 05:00:00 000:000:000 5                          
TAG_0001              2018-01-06 06:00:00 000:000:000 6                          
TAG_0001              2018-01-07 07:00:00 000:000:000 7                          
TAG_0001              2018-01-08 08:00:00 000:000:000 8                          
TAG_0001              2018-01-09 09:00:00 000:000:000 9                          
TAG_0001              2018-01-10 10:00:00 000:000:000 10                         
TAG_0002              2018-02-01 01:00:00 000:000:000 11                         
TAG_0002              2018-02-02 02:00:00 000:000:000 12                         
TAG_0002              2018-02-03 03:00:00 000:000:000 13                         
TAG_0002              2018-02-04 04:00:00 000:000:000 14                         
[10] row(s) selected.
```

## Search data over a certain value

The conditions for the tag value can also be given as follows.

Filtering was performed for those values greater than 12 and less than 15 among the values of TAG_0002.

```bash
Mach> select * from tag where name = 'TAG_0002' and value > 12 and value < 15 and time between to_date('2018-02-01') and to_date('2018-02-05');
NAME                  TIME                            VALUE                      
--------------------------------------------------------------------------------------
TAG_0002              2018-02-03 03:00:00 000:000:000 13                         
TAG_0002              2018-02-04 04:00:00 000:000:000 14                         
[2] row(s) selected.
```

## Display Statistical Information By Specific Tag ID

When we create tag table, virtual table that aggregate simple statistic information by tag table's tag ID is created.

virtual table name is v${tag table name}_stat.

If user uses that table, user can get tag table's statistic information quickly.

Statistical information target column is automatically designated as the third column.

```bash
Mach> CREATE TAG TABLE tag (name VARCHAR(20) PRIMARY KEY, time DATETIME BASETIME, value DOUBLE SUMMARIZED);
Executed successfully.
 
Mach> DESC v$tag_stat;
[ COLUMN ]                             
----------------------------------------------------------------------------------------------------
NAME                                                        NULL?    TYPE                LENGTH       
----------------------------------------------------------------------------------------------------
NAME                                                                 varchar             100                
ROW_COUNT                                                            ulong               20                 
MIN_TIME                                                             datetime            31             
MAX_TIME                                                             datetime            31             
MIN_VALUE                                                            double              17                 
MIN_VALUE_TIME                                                       datetime            31             
MAX_VALUE                                                            double              17                 
MAX_VALUE_TIME                                                       datetime            31             
RECENT_ROW_TIME                                                      datetime            31
```

If there is no SUMMARIZED keyword in the third column, VALUE-related information (MIN_VALUE, MAX_VALUE, MIN_VALUE_TIME, MAX_VALUE_TIME) is not saved.

Statistic information that is colleced is as follow.

|Column name|Information|
|--|--|
|NAME|Tag ID's name|
|ROW_COUNT|Number of Rows|
|MIN_TIME|The smallest basetime column value among the corresponding tag ID rows|
|MAX_TIME|The biggest basetime column value among the corresponding tag ID rows|
|MIN_VALUE|The smallest summarized column value among the corresponding tag ID rows|
|MIN_VALUE_TIME|The basetime column value that is inserted with MIN_VALUE|
|MAX_VALUE|The biggest summarized column value among the corresponding tag ID rows|
|MAX_VALUE_TIME|The basetime column value that is inserted with MAX_VALUE|
|RECENT_ROW_TIME|The basetime column value that is inserted most recently|

Example of select is as follow.

1. When a SUMMARIZED column exists

```bash
Mach> CREATE TAG TABLE tag (name VARCHAR(20) PRIMARY KEY, time DATETIME BASETIME, value DOUBLE SUMMARIZED);
Executed successfully.
  
Mach> INSERT INTO tag VALUES('tag-0', TO_DATE('2021-08-12'), 10);
Mach> INSERT INTO tag VALUES('tag-0', TO_DATE('2021-08-13'), 10);
Mach> INSERT INTO tag VALUES('tag-0', TO_DATE('2021-08-14'), 20);
Mach> INSERT INTO tag VALUES('tag-0', TO_DATE('2021-08-11'), 5);
Mach> INSERT INTO tag VALUES('tag-1', TO_DATE('2022-08-12'), 100);
Mach> INSERT INTO tag VALUES('tag-1', TO_DATE('2022-08-11'), 200);
Mach> INSERT INTO tag VALUES('tag-1', TO_DATE('2022-08-10'), 50);
  
Mach> SELECT * FROM v$tag_stat;
NAME                                                                              ROW_COUNT            MIN_TIME                        MAX_TIME                        MIN_VALUE                 
---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------
MIN_VALUE_TIME                  MAX_VALUE                   MAX_VALUE_TIME                  RECENT_ROW_TIME               
---------------------------------------------------------------------------------------------------------------------------------
tag-0                                                                             4                    2021-08-11 00:00:00 000:000:000 2021-08-14 00:00:00 000:000:000 5                         
2021-08-11 00:00:00 000:000:000 20                          2021-08-14 00:00:00 000:000:000 2021-08-11 00:00:00 000:000:000
tag-1                                                                             3                    2022-08-10 00:00:00 000:000:000 2022-08-12 00:00:00 000:000:000 50                        
2022-08-10 00:00:00 000:000:000 200                         2022-08-11 00:00:00 000:000:000 2022-08-10 00:00:00 000:000:000
[2] row(s) selected.
  
2. When a SUMMARIZED column does not exist
Mach> CREATE TAG TABLE other_tag (name VARCHAR(20) PRIMARY KEY, time DATETIME BASETIME, value DOUBLE);
Executed successfully.
  
Mach> INSERT INTO other_tag VALUES('tag-0', TO_DATE('2021-08-12'), 10);
Mach> INSERT INTO other_tag VALUES('tag-0', TO_DATE('2021-08-13'), 10);
Mach> INSERT INTO other_tag VALUES('tag-0', TO_DATE('2021-08-14'), 20);
Mach> INSERT INTO other_tag VALUES('tag-0', TO_DATE('2021-08-11'), 5);
Mach> INSERT INTO other_tag VALUES('tag-1', TO_DATE('2022-08-12'), 100);
Mach> INSERT INTO other_tag VALUES('tag-1', TO_DATE('2022-08-11'), 200);
Mach> INSERT INTO other_tag VALUES('tag-1', TO_DATE('2022-08-10'), 50);
  
Mach> SELECT * FROM v$other_tag_stat;
NAME                                                                              ROW_COUNT            MIN_TIME                        MAX_TIME                        MIN_VALUE                 
---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------
MIN_VALUE_TIME                  MAX_VALUE                   MAX_VALUE_TIME                  RECENT_ROW_TIME               
---------------------------------------------------------------------------------------------------------------------------------
tag-0                                                                             4                    2021-08-11 00:00:00 000:000:000 2021-08-14 00:00:00 000:000:000 NULL                      
NULL                            NULL                        NULL                            2021-08-11 00:00:00 000:000:000
tag-1                                                                             3                    2022-08-10 00:00:00 000:000:000 2022-08-12 00:00:00 000:000:000 NULL                      
NULL                            NULL                        NULL                            2022-08-10 00:00:00 000:000:000
[2] row(s) selected.
```

## Extraction by Using RESTful API

### Prepare for RESTful API

Specify the values of the properties blow and start the server.

machbase.conf

```
HTTP_ENABLE = 1
HTTP_PORT_NO = 5678
```

RESTful API calling convention 

**SELECT FORM**

```bash
{MWA URL}/machiot-rest-api/datapoints/raw/{TagName}/{Start}/{End}/{Direction}/{Count}/{Offset}/ 
 
TagName    : Tag Name. multiple tag available(Seperated by ',')
Start, End : range, YYYY-MM-DD HH24:MI:SS or YYYY-MM-DD or YYYY-MM-DD HH24:MI:SS,mmm (mmm: millisecond, When omitted start is 000, End is 999, micro and nano is 999)
When using real string, put 'T' between time and date to remove blank.
Direction  : 0(ascending), support in future (time increase)
Count      : LIMIT, whole if 0
Offset     : offset (default = 0)
```

### Sample for Fetching single tag data by using CURL 

Call for machbase installed in 192.168.0.148 as follow, the data can be retrieved from the web.

**Single Tag**

```bash
$ curl -G "http://192.168.0.148:5001/machiot-rest-api/v1/datapoints/raw/TAG_0001/2018-01-01T00:00:00/2018-01-06T00:00:00"
 
{"ErrorCode": 0,
 "ErrorMessage": "",
 "Data": [{"DataType": "DOUBLE",
 "ErrorCode": 0,
 "TagName": "TAG_0001",
 "CalculationMode": "raw",
 "Samples": [{"TimeStamp": "2018-01-01 01:00:00 000:000:000", "Value": 1.0, "Quality": 1},
             {"TimeStamp": "2018-01-02 02:00:00 000:000:000", "Value": 2.0, "Quality": 1},
             {"TimeStamp": "2018-01-03 03:00:00 000:000:000", "Value": 3.0, "Quality": 1},
             {"TimeStamp": "2018-01-04 04:00:00 000:000:000", "Value": 4.0, "Quality": 1},
             {"TimeStamp": "2018-01-05 05:00:00 000:000:000", "Value": 5.0, "Quality": 1}]}]
}
```

### fetching multi tag data by using CURL

Follow is sample for fetching two tag values.

```bash
$ curl -G "http://192.168.0.148:5001/machiot-rest-api/datapoints/raw/TAG_0001,TAG_0002/2018-01-05T00:00:00/2018-02-05T00:00:00"
{"ErrorCode": 0,
 "ErrorMessage": "",
 "Data": [{"DataType": "DOUBLE",
           "ErrorCode": 0,
           "TagName": "TAG_0001,TAG_0002",
           "CalculationMode": "raw",
           "Samples": [{"TimeStamp": "2018-01-05 05:00:00 000:000:000", "Value": 5.0, "Quality": 1},
                       {"TimeStamp": "2018-01-06 06:00:00 000:000:000", "Value": 6.0, "Quality": 1},
                       {"TimeStamp": "2018-01-07 07:00:00 000:000:000", "Value": 7.0, "Quality": 1},
                       {"TimeStamp": "2018-01-08 08:00:00 000:000:000", "Value": 8.0, "Quality": 1},
                       {"TimeStamp": "2018-01-09 09:00:00 000:000:000", "Value": 9.0, "Quality": 1},
                       {"TimeStamp": "2018-01-10 10:00:00 000:000:000", "Value": 10.0, "Quality": 1},
                       {"TimeStamp": "2018-02-01 01:00:00 000:000:000", "Value": 11.0, "Quality": 1},
                       {"TimeStamp": "2018-02-02 02:00:00 000:000:000", "Value": 12.0, "Quality": 1},
                       {"TimeStamp": "2018-02-03 03:00:00 000:000:000", "Value": 13.0, "Quality": 1},
                       {"TimeStamp": "2018-02-04 04:00:00 000:000:000", "Value": 14.0, "Quality": 1}
]}]}
```

## Specifying the serach direction using hints

In general, the tag table can be searched starting with the oldest record. If you want to search from the most recently inserted record, you can use hints to control the search direction.

### Forward Search

default, search by using '/*+ SCAN_FORWARD(table_name) */' hint.

```bash
Mach> SELECT * FROM tag WHERE t_name='TAG_99' LIMIT 10;
T_NAME                T_TIME                          T_VALUE                    
--------------------------------------------------------------------------------------
TAG_99                2017-01-01 00:00:49 500:000:000 0                          
TAG_99                2017-01-01 00:01:39 500:000:000 1                          
TAG_99                2017-01-01 00:02:29 500:000:000 2                          
TAG_99                2017-01-01 00:03:19 500:000:000 3                          
TAG_99                2017-01-01 00:04:09 500:000:000 4                          
TAG_99                2017-01-01 00:04:59 500:000:000 5                          
TAG_99                2017-01-01 00:05:49 500:000:000 6                          
TAG_99                2017-01-01 00:06:39 500:000:000 7                          
TAG_99                2017-01-01 00:07:29 500:000:000 8                          
TAG_99                2017-01-01 00:08:19 500:000:000 9                          
[10] row(s) selected.
Elapsed time: 0.001
 
Mach> SELECT /*+ SCAN_FORWARD(tag) */  * FROM tag WHERE t_name='TAG_99' LIMIT 10;
T_NAME                T_TIME                          T_VALUE                    
--------------------------------------------------------------------------------------
TAG_99                2017-01-01 00:00:49 500:000:000 0                          
TAG_99                2017-01-01 00:01:39 500:000:000 1                          
TAG_99                2017-01-01 00:02:29 500:000:000 2                          
TAG_99                2017-01-01 00:03:19 500:000:000 3                          
TAG_99                2017-01-01 00:04:09 500:000:000 4                          
TAG_99                2017-01-01 00:04:59 500:000:000 5                          
TAG_99                2017-01-01 00:05:49 500:000:000 6                          
TAG_99                2017-01-01 00:06:39 500:000:000 7                          
TAG_99                2017-01-01 00:07:29 500:000:000 8                          
TAG_99                2017-01-01 00:08:19 500:000:000 9                          
[10] row(s) selected.
Elapsed time: 0.001
Mach>
```

### Backward Search

search by using '/*+ SCAN_BACKWARD(table_name) */' hint.

```bash
Mach> SELECT /*+ SCAN_BACKWARD(tag) */ * FROM tag WHERE t_name='TAG_99' LIMIT 10;
T_NAME                T_TIME                          T_VALUE                    
--------------------------------------------------------------------------------------
TAG_99                2017-02-27 20:53:19 500:000:000 9                          
TAG_99                2017-02-27 20:52:29 500:000:000 8                          
TAG_99                2017-02-27 20:51:39 500:000:000 7                          
TAG_99                2017-02-27 20:50:49 500:000:000 6                          
TAG_99                2017-02-27 20:49:59 500:000:000 5                          
TAG_99                2017-02-27 20:49:09 500:000:000 4                          
TAG_99                2017-02-27 20:48:19 500:000:000 3                          
TAG_99                2017-02-27 20:47:29 500:000:000 2                          
TAG_99                2017-02-27 20:46:39 500:000:000 1                          
TAG_99                2017-02-27 20:45:49 500:000:000 0                          
[10] row(s) selected.
Elapsed time: 0.001
Mach>
```

### Setting basic scan direction property

By using [TABLE_SCAN_DIRECTION](/dbms/config-monitor/property#table_scan_direction) property, user can set tag table scan direction when there is no hint in select query.
