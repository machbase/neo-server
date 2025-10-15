# Insert

Similar to other commercial RDBMSs, you can first create the table and enter the data using the INSERT INTO statement.

Machbase provides the 'machsql' tool as an interactive query processor.

## Create Table

```sql
CREATE TABLE table_name ( column1 datatype, column2 datatype, column3 datatype, .... );
```

```sql
CREATE TABLE sensor_data ( id VARCHAR(32), val DOUBLE );
```

## Data Insertion

```sql
INSERT INTO table_name VALUES (value1, value2, value3, ...);
```

```sql
INSERT INTO sensor_data VALUES('sensor1', 10.1); INSERT INTO sensor_data VALUES('sensor2', 20.2); INSERT INTO sensor_data VALUES('sensor3', 30.3);
```

## Confirm Data Insertion

```sql
SELECT column1, column2, ... FROM table_name;
```

```sql
SELECT * FROM sensor_data;
```

## Entire Process

Below is an example using machsql.

```sql
Mach> CREATE TABLE sensor_data (id VARCHAR(32), val DOUBLE);
 Created successfully.
Mach> INSERT INTO sensor_data VALUES('sensor1', 10.1);
 1 row(s) inserted.
Mach> INSERT INTO sensor_data VALUES('sensor2', 20.2);
 1 row(s) inserted.
Mach> INSERT INTO sensor_data VALUES('sensor3', 30.3);
 1 row(s) inserted.
Mach> SELECT * FROM sensor_data;
ID VAL
-----------------------------------------------------------------
sensor3 30.3
sensor2 20.2
sensor1 10.1
[3] row(s) selected.
```
