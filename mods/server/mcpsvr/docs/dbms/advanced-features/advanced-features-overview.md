# Backup Overview

## The Concept of BACKUP/MOUNT

To ensure the permanence of the database, the data stored in memory is stored on the disk as soon as possible. In case of a general failure such as Process Failure, Restart Recovery makes the database consistent. However, in case of power failure or hardware damage caused by fire, database recovery is impossible. In order to solve this problem, the database backup and recovery function saves data to another disk or hardware periodically in another area and recovers the data using the corresponding data in case of an emergency.

Database backups are divided into two types depending on when they are performed.
* Offline Backup
* Online Backup

First, the Offline Backup function is called Cold Backup as it shuts down the DBMS and copies the database. It is very simple, but it has the disadvantage of the user's service being interrupted. Therefore, it is rarely used during operation and tends to be used only for initial testing or data construction.

Second, Online Backup is called Hot Backup as a function to backup the database when DBMS is running. This function can be performed without interrupting the service, increasing the user's service availability. Most DBMS Backup refers to Online Backup. Unlike other database backups, Machbase, a time-series database, provides time range backup. This allows you to specify the time of the database to be backed up at the time of backup so that only the data at the desired time will be backed up.

```sql
backup database into disk = 'backup';
backup database from to_date('2015-07-14 00:00:00','YYYY-MM-DD HH24:MI:SS') to to_date('2015-07-14 23:59:59 999:999:999','YYYY-MM-DD HH24:MI:SS mmm:uuu:nnn')
                into disk = 'backup_20150714';
```

The backed-up database can be used as an existing database through a recovery process. This recovery method is called Restore. This Restore function deletes the damaged database and restores the backed up database image to the Primary Database. Therefore, when recovering, delete the existing database and restore it using machadmin -r.

```bash
machadmin -r 'backup'
```

The mount / unmount function is an online function that attaches a backed up database to the currently running database.

```sql
mount database 'backup' to mountName;
umount database mountName;
```

## Database Backup

Machbase offers two options for backing up your data. It provides DATABASE backup function which backs up information of running DB and TABLE backup function which can select only necessary tables to back up.
The backup command provided by DB is as follows.

```bash
BACKUP [ DATABASE | TABLE table_name ]  [ time_duration ] INTO DISK = 'path/backup_name';
time_duration = FROM start_time TO end_time
path = 'absolute_path' or  'relative_path'
```

```
-- Directory backup
BACKUP DATABASE INTO DISK = 'backup_dir_name';

-- Time range backup
BACKUP DATABASE FROM TO_DATE('2015-07-14 00:00:00','YYYY-MM-DD HH24:MI:SS')
                     TO TO_DATE('2015-07-14 23:59:59','YYYY-MM-DD HH24:MI:SS')
                     INTO DISK = '/home/machbase/backup_20150714'
```

When performing a DB backup, the backup type, time duration, and path must be entered as options. When backing up DATABASE entirely, type DATABASE for the backup type. To backup only a specific table, enter TABLE, and then enter the name of the table to be backed up. The `time_duration` clause can be set to back up only the data for the required period. In the FROM field, enter the start time of the date you want to back up, and enter the time of the last date in the TO field. In Example 2, the `time_duration` is set to FROM, "July 14, 2014, 0, 0, 0", and TO "July 14, 2015, 23:59:59" meaning only 14 days of data are set to be backed up. If `time_duration` is not specified, the FROM item is set to 'January 1, 1970, 9:00:00,' and the TO item is automatically set to the time to execute the command.

Finally, a storage medium to store needs to be configured to store the results of the backup. If you want to create a backup in a single file, set the creation type to IBFILE, or enter DISK to create it in directory units. Note that you can specify the PATH information to store the product. If you enter a relative path, it will be created in the path specified in the DB_PATH item of the current DB configuration. If you want to store it somewhere other than DB_PATH, you must enter an absolute path starting with '/'.

### Incremental Backup

Incremental backup is a function that backs up only data entered after the previous backup. The target for incremental backup is only data in the log and tag tables, and the lookup table always backs up all data. 
In order to perform an incremental backup, the previously performed incremental backup directory or the entire backup directory is required. 
Incremental backup is performed as follows.

```sql
Mach> BACKUP DATABASE INTO DISK = 'backup1'; /* run full backup */
Executed successfully.
Mach> ...
  
Mach> BACKUP DATABASE AFTER 'backup1' INTO DISK = 'backup2'; /* Running incremental backup on the data inserted after backup1 */
Executed successfully.
Mach> ...
```

Incremental backup is available for the entire database (at this time, the lookup table becomes a full backup), log table, and tag table. If you want to backup by RESTORE function, you need backup data which is saved before incremental backup.
If you do not want to delete the current data and return to the previous state, you can use the MOUNT function described below.

### Precautions for Incremental Backup

As above, if backup2 is created as an incremental backup based on backup1, if backup1 is lost (due to disk failure, etc.), it cannot be restored using backup2.

For the same reason, if a previous backup is lost after an incremental backup, it cannot be restored using a later backup.

If the backup is performed 3 times as shown below, the previous backup of backup3 becomes backup2 and the previous backup of backup2 becomes backup1.

Therefore, if backup1 is lost, both backup2 and backup3 cannot be used, and if backup2 is lost, it cannot be recovered using backup3.

```sql
Mach> BACKUP DATABASE INTO DISK = 'backup1'; /* run full backup */
Executed successfully.
Mach> ...
  
Mach> BACKUP DATABASE AFTER 'backup1' INTO DISK = 'backup2'; /* Running incremental backup on the data inserted after backup1 */
Executed successfully.
Mach> ...
 
Mach> BACKUP DATABASE AFTER 'backup2' INTO DISK = 'backup3'; /* Running incremental backup on the data inserted after backup2 */
Executed successfully.
Mach> ...
```

## Database Restore

The Database Restore feature is not provided as a syntax, and can be recovered offline using machadmin -r. You must check the following before restoration.

* Has Machbase been shutdown?
* Has the previously created DB been deleted?

```bash
machadmin -r backup_database_path;
```

```sql
backup database into disk = '/home/machbase/backup';
```

```bash
machadmin -k
machadmin -d
machadmin -r /home/machbase/backup;
```

## Database Mount

The following problems arise when periodically backing up a large number of databases and adding data continuously in preparation for a system failure.

* Increased disk cost to store data
* Limitations of the physical disk space of the running machine

In order to solve this problem, periodical deletion is performed by leaving only data necessary for the current service. However, if you need to refer to the past data, you need to restore the backed up database. In case of a very large backup image, recovery time is long and additional equipment is needed. This is because the Restore function can only be performed by deleting the currently running database. To solve this problem, Machbase provides the Database Mount function.

The Database Mount function is an online function that attaches a backed up database to the currently running database. By attaching multiple backup databases to the primary database, the user can refer to multiple backup databases as if they were one database. The mounted database is read-only.

The Mount DATABASE command is a function that prepares the database or table DATA created by Backup in a state that it can be viewed from the currently running database. So, Mounted DATABASE can query the data using the same DB command.

The current Database Mount function restrictions are as follows.

* The backup information must be compatible with the database to be mounted, the DB major number, and the Meta major number.
* When mounting backup data, it is read-only and does not support index creation, data insertion or deletion.
* Information about the currently mounted DATABASE can be found by querying V$STORAGE_MOUNT_DATABASES.
* When incremental backup data is mounted, only the incremental data recorded in the backup data is searched, and it does not mount by following the previously performed incremental data.

### Mount

To execute the mount command, Backup_database_path information and DatabaseName are required. Backup_database_path is the location information of the DB created by Backup command. DatabaseName is the name that can be distinguished when mounting to Database. Backup_database_path is searched based on the directory specified in the DB_PATH set in the environment variable of the DB when the relative path is entered in the same way as when performing the backup.

```sql
MOUNT DATABASE 'backup_database_path' TO mount_name;
MOUNT DATABASE '/home/machbase/backup' TO mountdb;
```

### Unmount

If the mounted database will no longer be used, it can be removed using the unmount command.

```sql
UNMOUNT DATABASE mount_name;
UNMOUNT DATABASE mountdb;
```

### MOUNT DB Data Retrieval

When querying DATA of Backup DB, it can be retrieved by using the same SQL statement when querying the DATA of the DB in operation.

The mounted DB can retrieve data only by the SYS admin user of the DB in operation. To retrieve the data, you must put MountDBName and UserName in front of the TableName to be queried, and use '.' for each delimiter. MountDBName is used to refer to a specific DB among currently mounted DBs, and UserName refers to the information of the user that owns the mounted DB table.

```sql
SELECT column_name FROM mount_name.user_name.table_name;
```

```sql
SELECT * FROM mountdb.sys.backuptable;
```
