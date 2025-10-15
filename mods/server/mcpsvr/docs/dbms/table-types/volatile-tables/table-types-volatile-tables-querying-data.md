# Volatile Data Extraction

## Data Retrieval

As with other table types, data retrieval can be performed as follows.

```sql
Mach> create volatile table vtable (id integer primary key, name varchar(20));
Created successfully.
Mach> insert into vtable values(1, 'west device');
1 row(s) inserted.
Mach> insert into vtable values(2, 'east device');
1 row(s) inserted.
Mach> insert into vtable values(3, 'north device');
1 row(s) inserted.
Mach> insert into vtable values(4, 'south device');
1 row(s) inserted.
Mach> select * from vtable;
ID          NAME                 
-------------------------------------
1           west device          
2           east device          
3           north device         
4           south device         
[4] row(s) selected.
Mach> select * from vtable where id = 1;
ID          NAME                 
-------------------------------------
1           west device          
[1] row(s) selected.
Mach> select * from vtable where name like 'west%';
ID          NAME                 
-------------------------------------
1           west device          
[1] row(s) selected.
```
