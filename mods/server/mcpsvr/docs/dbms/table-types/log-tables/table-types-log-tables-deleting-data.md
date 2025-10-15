# Deletion of Log Data

The DELETE statement in Machbase can be performed on the log table.

In addition, it is not possible to delete data in an arbitrary position in the middle, and it is possible to erase consecutively from the arbitrary position to the last (oldest log) record. This is a policy that takes advantage of the characteristics of log data. It is a DB format representation of the act of deleting a file in order to secure space when it is entered once.

Below is the type of expression you can use.

##  Syntax

```sql
DELETE FROM table_name;
DELETE FROM table_name OLDEST number ROWS;
DELETE FROM table_name EXCEPT number ROWS;
DELETE FROM table_name EXCEPT number [YEAR | MONTH | WEEK | DAY | HOUR | MINUTE | SECOND];
DELETE FROM table_name BEFORE datetime_expr;
```

##  Example

```sql
-- Delete all data.
mach>DELETE FROM devices;
10 row(s) deleted.
 
-- Delete oldest 5.
mach>DELETE FROM devices OLDEST 5 ROWS;
10 row(s) deleted.
 
-- Delete all except last 5.
mach>DELETE FROM devices EXCEPT 5 ROWS;
15 row(s) deleted.
 
-- Delete all data from before June 1, 2018.
mach>DELETE FROM devices BEFORE TO_DATE('2018-06-01', 'YYYY-MM-DD');
50 row(s) deleted.
```
