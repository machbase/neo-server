# Creating and Managing Log Table

The log table can be simply generated as follows.

Let's create a table called sensor_data and delete it.

Data types compatible with Machbase can be found in the SQL Reference Types.

## Creating Log Table

Create a log table with the 'CREATE TABLE' syntax.

```sql
Mach> CREATE TABLE sensor_data (id VARCHAR(32), val DOUBLE);
Created successfully.
 
Mach> DROP TABLE sensor_data;
Dropped successfully.
```

## Deleting Log Table

Delete log table with 'DROP TABLE' statement.

```sql
-- DROP deletes both data and table.
Mach> DROP TABLE sensor_data;
Dropped successfully.
 
-- TRUNCATE deletes only data and keeps table.
Mach> TRUNCATE TABLE sensor_data;
Truncated successfully.
```
