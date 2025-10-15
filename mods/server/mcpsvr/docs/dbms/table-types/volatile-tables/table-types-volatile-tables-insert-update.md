# Inserting and Updating Volatile Data

##  Data Insert

The data insert of the volatile table is as follows.

```sql
Mach> create volatile table vtable (id integer, name varchar(20));
Created successfully.
Mach> insert into vtable values(1, 'west device');
1 row(s) inserted.
Mach> insert into vtable values(2, 'east device');
1 row(s) inserted.
Mach> insert into vtable values(3, 'north device');
1 row(s) inserted.
Mach> insert into vtable values(4, 'south device');
1 row(s) inserted.
```

##  Data Append

It is a fast real-time data input API provided by Machbase.
C, C++, C#, Java, Python, PHP, Javascript are available for append.

```sql
Mach> create volatile table vtable (id integer, value double);
```

```c
SQL_APPEND_PARAM sParam[2];
for(int i=0; i<10000; i++)
{
    sParam[0].mInteger  = i;
    sParam[1].mDouble   = i;
    SQLAppendDataV2(stmt, sParam) != SQL_SUCCESS)
}
```

For the Cluster Edition Append, must be done by Leader Broker.

Details are in [SDK](/dbms/sdk) guide.

##  Data Update

When inputting data in a volatile table, data with duplicate primary key values ​​can be updated using the ON DUPLICATE KEY UPDATE clause.

### Update Data Value to be Inserted

If the INSERT statement specifies data to be inserted, but there is other data that matches the primary key value of the insert data, the INSERT statement fails and the corresponding data is not inserted. If there is another data that matches the primary key value of the insertion data, and if you wish to update the corresponding data instead of insertion, a  ON DUPLICATE KEY UPDATE clause can be added.

* If there is no duplicate primary key data, the contents of the data to be inserted are inserted as is.
* If there is duplicate primary key data, the existing data is updated with the contents of the data to be inserted.

The constraints for using this function are as follows.

* The primary key must be specified in the volatile table.
* The value to be inserted must include the primary key value.

```sql
Mach> create volatile table vtable (id integer primary key, direction varchar(10), refcnt integer);
Created successfully.
Mach> insert into vtable values(1, 'west', 0);
1 row(s) inserted.
Mach> insert into vtable values(2, 'east', 0);
1 row(s) inserted.
Mach> select * from vtable;
ID          DIRECTION   REFCNT     
----------------------------------------
1           west       0          
2           east        0          
[2] row(s) selected.
 
Mach> insert into vtable values(1, 'south', 0);
[ERR-01418 : The key already exists in the unique index.]
Mach> insert into vtable values(1, 'south', 0) on duplicate key update;
1 row(s) inserted.
 
Mach> select * from vtable;
ID          DIRECTION   REFCNT     
----------------------------------------
1           south        0          
2           east        0          
[2] row(s) selected.
```

### Specify Data Value to be Updated

Similar to above, but if you need to update to a different column value than the data value to be inserted, it can be specified through the ON DUPLICATE KEY UPDATE SET clause. The data value to be updated can be specified under the SET clause.

* If the primary key duplication data does not exist, the contents of the embedded data are inserted as it is.
* If there is duplicate primary key data, the existing data is updated only with the update data specified in the SET clause.
* **The primary key value can not be specified as the data value to be updated.**
* The values ​​of the columns not specified in the SET clause are not updated.

```sql
Mach> create volatile table vtable (id integer primary key, direction varchar(10), refcnt integer);
Created successfully.
Mach> insert into vtable values(1, 'west', 0);
1 row(s) inserted.
Mach> insert into vtable values(2, 'east', 0);
1 row(s) inserted.
Mach> select * from vtable;
ID          DIRECTION   REFCNT     
----------------------------------------
1           west        0          
2           east        0          
[2] row(s) selected.
 
Mach> insert into vtable values(1, 'west', 0) on duplicate key update set refcnt = 1;
1 row(s) inserted.
 
Mach> select * from vtable;
ID          DIRECTION   REFCNT     
----------------------------------------
1           west        1          
2           east        0          
[2] row(s) selected.
```
