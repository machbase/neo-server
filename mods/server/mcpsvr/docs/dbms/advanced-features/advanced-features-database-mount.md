# Database Mount

* MOUNT
* UNMOUNT
* Read data from a mounted database

Continuously storing a large amount of data in order to analyze the data greatly increases its volume, resulting in the following problems:

* Increased disk cost by storing a large volume of data
* Running into equipment disk limits for data analysis

To solve the problem, it is necessary to back up old data and periodically delete it. If you need to read old data later and you perform data recovery to read the backed up database, it will not only take a long time to recover, but it will also take a long time to convert the database to offline, There is a problem that requires separate equipment to continue the service. Machase supports the MOUNT command to solve this problem.

The MOUNT command reads the backed up data while the database is in service and creates a new database independent of the currently running database. One server can add multiple backed-up databases to retrieve data at the same time, but the mounted database is read-only and can not add or delete data.

The Database MOUNT command allows you to read both the backup data and the main database contents simultaneously. Therefore, the mounted database can retrieve data in the same way as the existing data retrieval method.

To execute the MOUNT instruction, the following conditions must be met.
* The backup database version and the metadata version must be compatible.
* You can not create tables, create/delete indexes, or add/delete data to a mounted backup database.

Information about mounted databases can be obtained from the V$STORAGE_MOUNT_DATABASES meta table.

## Mount 

To run the mount command, the backup database pathname and the name of the database to be mounted must be entered.
The backup database path sets the location of the directory executed by the backup command. The name of the database to be mounted must be given a separate name to distinguish it from the active database.
The backup database pathname can be an absolute pathname (a pathname beginning with the "/" character), or a relative pathname based on $MACHBASE_HOME/dbs with the same rules as the backup command.

Syntax:

```sql
MOUNT DATABASE 'backup_database_path' TO mount_name;
```

Example:

```sql
MOUNT DATABASE '/home/machbase/backup' TO mountdb;
```

## Unmount 

If the mounded database data no longer needs to be read, use the UNMOUNT command to release the mounted state.

Syntax:

```sql
UNMOUNT DATABASE mount_name;
```

Example:

```sql
UNMOUNT DATABASE mountdb;
```

## Reading Data from Mounted Database

When retrieving data from a mounted database, use the same SQL statement as before.
Only the SYS user can read the mounted data. To specify the mounted database table in an SQL statement, the mount_name and user_name must be set connected to a "." character.

Syntax:

```sql
SELECT column_name FROM mount_name.user_name.table_name;
```

Example:

```sql
SELECT * FROM mountdb.sys.backuptable;
```
