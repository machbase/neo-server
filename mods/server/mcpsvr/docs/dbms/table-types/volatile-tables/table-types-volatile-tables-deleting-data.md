# Deleting Volatile Data

##  Delete Data

Volatile tables can delete data using the primary key value condition in the condition clause (WHERE clause).

* **The primary key column must be specified in the volatile table.**
* Only the (Primary key column) = (value) condition is allowed, and can not be used with other conditions.
* You can not use a column other than the primary key column.

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
[4] row(s) inserted.
Mach> delete from vtable where id = 2;
[1] row(s) deleted.
Mach> select * from vtable;
ID          NAME                 
-------------------------------------
1           west device          
3           north device         
4           south device         
[3] row(s) selected.
```
