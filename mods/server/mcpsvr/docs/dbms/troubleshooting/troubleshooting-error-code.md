# Error Code

|code|mssage|
|--|--|
|1|Failed to create file<%s>, errno = %d.|
|2|Failed to truncate file<%s>, errno = %d.|
|3|Failed to duplicate file<%s>, errno = %d.|
|4|Failed to copy file<%s> to file<%s>, errno = %d.|
|5|Failed to rename file<%s> to file<%s>, errno = %d.|
|6|Failed to remove file<%s>, errno = %d.|
|7|Failed to get key file<%s>, errno = %d.|
|8|Failed to create pipe<%s>, errno = %d.|
|9|Failed to statistic file<%s>, errno = %d.|
|10|Failed to open file<%s>, errno = %d.|
|11|Failed to close file<%s>, errno = %d.|
|12|Failed to seek file<%s>, offset:%lld, Whence:%d, errno = %d.|
|13|Failed to read file<%s>, size:%llu, errno = %d.|
|14|Failed to write file<%s>, size:%llu, errno = %d.|
|15|Failed to read file<%s> (offset:%llu, req size:%llu, read size: %llu), errno = %d.|
|16|Failed to write file<%s> (offset:%llu, req size:%llu, read size: %llu), errno = %d.|
|17|Failed to sync file<%s>, errno = %d.|
|18|Failed to lock file<%s>, errno = %d.|
|19|Failed to trylock file<%s>, errno = %d.|
|20|Failed to unlock file<%s>, errno = %d.|
|21|There is no file extension.|
|22|Failed to rename file<%s> to file<%s>, retry count<%d>, msec<%d>, errno = %d.|
|31|Error occurred during snprintf: buffer size<%d>, errno = %d.|
|61|Failed to getenv variable<%s>, errno = %d.|
|62|Failed to setenv variable<%s> to value<%s>, errno = %d.|
|67|Failed to opendir <%s>, errno = %d.|
|68|Failed to closedir, errno = %d.|
|69|Failed to readdir, errno = %d.|
|70|Failed to rewinddir, errno = %d.|
|71|Failed to makedir <%s>, errno = %d.|
|72|Failed to removedir, errno = %d.|
|73|Failed to setcwd, errno = %d.|
|74|Failed to getcwd, errno = %d.|
|75|Failed to gethome, errno = %d.|
|76|Path<%s> is too long, errno = %d.|
|77|Path<%s/%s> is too long, errno = %d.|
|78|Path<%s/%s/%s> is too long, errno = %d.|
|79|The directory does not exist in this path<%s>, errno = %d.|
|80|Failed to call removedir (%s), errno = %d.|
|91|%1$s(): [%2$d: %3$s]|
|92|%1$s(): [%2$d: %3$s]|
|121|Stack create failed, errno = %d.|
|122|Stack push failed, errno = %d.|
|123|Stack pop failed, errno = %d.|
|131|Failed to allocate memory(%lu bytes), errno = %d.|
|132|Memory allocation error (alloc'd: %llu, max: %llu).|
|133|Failed to allocate memory (ID = %d) (Request Size = %llu) : (Current Allocated Size / PROCESS_MAX_SIZE (%llu/%llu)).|
|141|Failed to create memory pool, errno = %d.|
|142|Failed to allocate memory from memory pool, errno = %d.|
|151|Failed to create mutex, errno = %d.|
|152|Failed to destroy mutex, errno = %d.|
|153|Failed to lock mutex, errno = %d.|
|154|Failed to trylock mutex, errno = %d.|
|155|Failed to unlock mutex, errno = %d.|
|161|Failed to create queue, errno = %d.|
|162|Failed to destroy queue, errno = %d.|
|163|Failed to enqueue queue, errno = %d.|
|164|Failed to dequeue queue, errno = %d.|
|171|Failed to create thread_attr, errno = %d.|
|172|Failed to destroy thread_attr, errno = %d.|
|173|Failed to set thread_attr bound, errno = %d.|
|174|Failed to set thread_attr detach, errno = %d.|
|175|Failed to set thread_attr stack size, errno = %d.|
|176|Failed to create thread, errno = %d.|
|177|Failed to detach thread, errno = %d.|
|178|Failed to join thread, errno = %d.|
|179|Failed to get id of thread, errno = %d.|
|191|Failed to create thread condition variable, errno = %d.|
|192|Failed to destroy thread condition variable, errno = %d.|
|193|Failed to call cond_timedwait, errno = %d.|
|194|Failed to call cond_signal, errno = %d.|
|195|Failed to call cond_broadcast, errno = %d.|
|196|Failed to call cond_wait, errno = %d.|
|201|Failed to create rwlock, errno = %d.|
|202|Failed to destroy rwlock, errno = %d.|
|203|Failed to call rwlock_lock_read, errno = %d.|
|204|Failed to call rwlock_trylock_read, errno = %d.|
|205|Failed to call rwlock_lock_write, errno = %d.|
|206|Failed to call rwlock_trylock_write, errno = %d.|
|211|RBTREE buffer<%d> is too small for value<%d>, errno = %d.|
|212|RBTREE cursor op not applicable. errno = %d.|
|213|RBTREE node is already freed, errno = %d.|
|216|Key already exists.|
|221|LZO compress failed, errno = %d.|
|222|LZO decompress failed, errno = %d.|
|231|Failed to get CPU count, errno = %d.|
|232|Configuration file does not exist(%S).|
|251|Tlsf memory manager initialization failed, errno = %d.|
|252|Tlsf memory manager finalization failed, errno = %d.|
|253|Tlsf memory manager allocation(%lld) failed, errno = %d.|
|254|Tlsf memory manager free failed, errno = %d.|
|255|Tlsf memory manager control failed, errno = %d.|
|256|Tlsf memory manager shrink failed, errno = %d.|
|257|Tlsf memory manager getstatistics failed, errno = %d.|
|271|The session is closed.|
|272|The session is canceled.|
|291|The license is invalid or expired.|
|292|The value<%s> does not exist in the license file.|
|293|Failed to get hardware key, errno =%d|
|294|Failed to verify the license, errno = %d|
|300|Invalid date value.(%s)|
|301|Invalid network string.(%s)|
|321|Error in initializing sha1, errno = %d|
|322|Error in updating sha1, errno = %d|
|323|Error in finalizing sha1, errno = %d|
|324|Invalid SHA type.(%d)|
|326|Invalid SHA hex string.(%s)|
|341|Parallel job thread abnormally terminated|
|342|The thread count should be between %d and %d|
|361|Error in setting a log to the buffer of the result file: %s, errno = %d|
|381|Regular expression error: an error occurred at offset %d of (%s).|
|400|This DB file is older than binary (no meta-version table). Check database image and binary.|
|401|Version mismatched. In Executable DB(%d.%d) META(%d.%d) CM(%d.%d) But, In File DB(%d.%d) META(%d.%d) CM(%d.%d)|
|420|Error in getting system information by the sysinfo, errno = %d|
|421|Error in getting stack information by the pmuSysSetStackSize, errno = %d|
|422|Error in setting stack information by the pmuSysSetStackSize, errno = %d|
|431|mmap (size<%u>) error, errno = %d|
|432|unmap (address<%p>, size<%u>) error, errno = %d|
|451|Failed to set the CPU affinity [%u, %u), errno = %d|
|452|The IDs of CPUs should be between [0, %u), but [%u, %u) given.|
|453|Maximum abs value of CPU_AFFINITY_COUNT(%d) should be less than CPU count(%u).|
|461|Failed to get the number of CPUs in sysconf, errno = %d|
|471|Failed to initialize a heap.|
|472|Heap push failed, errno = %d|
|491|Error in json dump.|
|492|Error in json load.|
|493|json object error: %s|
|494|Error in json-array.|
|495|Error in json-string (%s).|
|496|Error in json-integer (%lld).|
|497|Error in json-real (%lf).|
|498|Error in json copy.|
|499|Error in json pack.|
|500|Error in json unpack.|
|501|No data matches for the json path (%s)|
|502|Json path is too long.|
|503|Error json object set (%s).|
|504|Error json array append.|
|505|Error encode base64.|
|506|Error decode base64.|
|600|Invalid property value: %s.|
|601|Failed to convert %s to UTF8. (%s, errno=%d)|
|602|Buffer size is not enough for code conversion. (%d > %d)|
|701|Geohash invalid precision (%u)|
|702|Geohash invalid length|
|703|Geohash invalid direction|
|1000|File<%s> is invalid.|
|1001|Invalid object storage id, errno = %d.|
|1002|Object storage<%d> already freed, errno = %d.|
|1003|Group storage dir<%s> already exists, errno = %d.|
|1004|Object filename<%s> is invalid, errno = %d.|
|1005|Disk file<%s> is in use, errno = %d.|
|1006|Functionality is not supported yet.|
|1007|There is no available disk space for writing <%lld>bytes to the file<%s>, errno = %d.|
|1008|Error in the duplicating file<%s>, errno = %d.|
|1009|Error in the read file size.(<io: %u>, <disk: %u>)|
|1010|Used media space is reached to threshold. (%4.1lf%% cap < %4.1lf%% used)|
|1011|Error in the write file size.(<write: %u>, <written: %u>)|
|1031|The database in <%s> has already been mounted.|
|1032|The database in <%s> is not mounted.|
|1033|The mount operation of database in <%s> is not completed.|
|1034|The mounted database<%s> is busy.|
|1035|The database creation is not complete. Destroy it and create a new one.|
|1036|The database creation is not complete. Destroy it and create a new one.|
|1037|The mount database<%s> is not backed up from the primary database|
|1038|Cannot find MountDB with <TBSID: %lld>.|
|1039|Mount DB<%s>'s state is invalid.|
|1101|Error in reading column partition cache block. Reading block of RID<%lld> in the column partition<%lld> failed, errno = %d.|
|1102|Invalid cache object.|
|1103|Error occurred in checkpoint thread. Processing abnormal shutdown.|
|1104|Error in waiting to read a page.|
|1105|Error in clear thread of the page cache.|
|1106|It<%llu> is smaller than the max size value of the page cache currently set<%llu>.|
|1107|It<%llu> is impossible to set a value larger than the memory size set in the current process<%llu>.|
|1108|Invalid page id in column partition. Page id<%d> is greater than the page max id<%d>.|
|1201|Duplicated table id<%llu> in SYS_STORAGE_TABLES, errno = %d.|
|1202|Duplicated table id<%llu>, column id<%u> in the SYS_STORAGE_COLUMNS, errno = %d.|
|1203|Table id<%lld> does not exist in SYS_STORAGE_TABLES, errno = %d.|
|1204|Duplicated (table id<%llu>, index id<%llu>) in SYS_STORAGE_INDEXES, errno = %d.|
|1205|Duplicated (table id<%llu>, index id<%llu>, column id<%u>) in SYS_STORAGE_INDEXES_COLUMNS, errno = %d.|
|1206|Index id<%llu> of table id<%llu> dose not exist in SYS_STORAGE_INDEXES, errno = %d.|
|1207|Available recovery modes: simple, complex, reset|
|1301|Partition range does not exist. Partition id is less than <%lld> in the table(id<%lld>) with partitions between <%lld> and <%lld>.|
|1302|Invalid record range. No such record whose id is less than <%llu> in the table(id<%llu>) with records between <%llu> and <%llu>.|
|1303|Maximum number of columns in a table is %d.|
|1304|Invalid column ID (<%d>).|
|1305|Invalid table ID (<%llu>).|
|1306|Table has been dropped.|
|1307|Table structure was modified.|
|1308|Invalid fixed column size. Invalid value size(<%u>) for the fixed column.|
|1309|Invalid varying column size. Value size(<%u>) for the variable column is greater than the max size (<%u>).|
|1310|Table flush thread terminated abnormally.|
|1311|Table column partition prepare thread terminated abnormally.|
|1312|Failed to read the head of the table column partition file (<%s>).|
|1313|Failed to read the table column partition file (<%s>).|
|1314|Index build thread terminated abnormally.|
|1315|Invalid table type<%d>.|
|1316|Column size<%u> is too big.|
|1317|Value of the time column(<%lld>) is less than the last time value(<%lld>).|
|1318|The size of VARCHAR column must be less than (<%llu>).|
|1319|The size of column value must be less than (<%u>).|
|1320|There is an index on the column(<%u>) of the table(<%llu>)|
|1321|This feature is not supported on this table type.|
|1322|The new column size(<%u>) should be greater than the old one(<%u>)|
|1323|The table(%llu) reached max column count limit (%u) already.|
|1324|An error occurred adjusting end rid of the table<%llu> column partition(<%llu>), errno = %d.|
|1325|The end RID<%lld> of the column<%d> is less than the end RID<%llu> of the table<%llu>|
|1330|The column with ID<%hu> does not exist in the table with ID<%llu>|
|1331|Table checkpoint thread terminated abnormally.|
|1332|Partition ID <%llu> of the table(id<%llu>) does not exist between <%llu> and <%llu>.|
|1333|The table<%llu> in the backup database<%s> has been mounted already.|
|1334|The table is busy with mounting.|
|1335|The mounted table is busy with unmounting.|
|1336|The mounted table is invalid.|
|1337|The mounted table is busy.|
|1338|The table is not mounted.|
|1339|The table<%llu> of the backup tablespace<%s> is different from the table in main database.|
|1340|The table<%llu> of the backup tablespace<%s> is dropped from the main database.|
|1341|There is a mounted table in the table<%llu>.|
|1342|The mount table<end_rid:%llu> has more furture data than the base table<end_rid:%llu.>|
|1343|Cannot update columns with indexes in VOLATILE / LOOKUP table.|
|1344|The memory size<%llu bytes> of VOLATILE / LOOKUP tables exceeds <%llu bytes>.|
|1345|The value of the column<%u> must not be NULL|
|1346|Current Allocate Memory / PROCESS_MAX_SIZE (%llu/%llu), increase PROCESS_MAX_SIZE property and restart.|
|1401|Invalid index type. Index type<%d> does not exist.|
|1402|Index id(<%llu>) does not exist in table id <%llu>.|
|1403|Index has invalid column count(<%d>).|
|1404|Index has invalid key value count(<%d>).|
|1405|Index has invalid key value size(<%d>).|
|1406|Index column file(<%s>) is invalid.|
|1407|Failed to read the head of the index column partition file(<%s>).|
|1408|Failed to read the index column partition file(<%s>).|
|1409|Type of the column for the index is invalid.|
|1410|Index flush thread terminated abnormally.|
|1411|Index build thread terminated abnormally.|
|1412|The keyword size<%d> should be less than the max size<%d>.|
|1413|The word bit count(%d) is over than %d in the partition<%lld> of the index <%lld>|
|1414|Invalid key count <%u> is not equal to the count <%u> in partition <%lld> of index <%lld>.|
|1415|The level<%u> of the index is bigger than the max level<%u>|
|1416|The partition size<%u> of level<%u> is bigger than the max level<%u>|
|1417|The index has been dropped.|
|1418|The key already exists in the unique index.|
|1419|The primary index is already created on the table.|
|1420|The number<%llu> of key values is different from the number<%llu> of bitvectors.|
|1421|The partition file<%llu> on the level<%u> of the index<%llu> is invalid.(KPC:%u, BPC:%u)|
|1422|NULL value is not allowed for the primary index column|
|1423|TAG cache exhausted, increase TAG_CACHE_MAX_MEMORY_SIZE(%llu)|
|1424|Could not allocate TAG cache: (Table,part=%llu,%llu) offset/size=%llu/%llu|
|1425|Failed to allocate index memory (Current Allocated Size / Threshold size (%llu/%llu)).|
|1426|Not ready to build keyvalue index (Current Count / Target Count (%llu/%llu) in File).|
|1501|Invalid page id in cpfile. Page id<%d> for the column partition file<%s> is greater than the page max id.|
|1502|Error in reading page<%d> in the column partition file<%s>. Page timestamps <head:%lld, tail:%lld> are invalid.|
|1503|Error in reading page<%d> in the column partition file<%s>. Page checksum <write:%#X, read:%#X> are invalid|
|1504|The size<%u> of the column partition file<%s> is too small. It is supposed to be greater than the size<%u>|
|1505|The offset<%u> and size<%u> of the update value for the page<id:%u, offset:%u, size:%u> in the column partition file<%s> is invalid|
|1506|The checksum<write:%#X, read:%#X> of the head of the column partition file<%s> is invalid.|
|1551|Error in getting the fd of the file<%s> from the fd cache.|
|1601|Ager thread terminated abnormally.|
|1631|There is no root dir<%s> for the database backup.|
|1632|The database is not destroyed.|
|1633|Failed to write data<%u> of the backup stat file<%s>.|
|1634|Failed to read data<%u> of the backup stat file<%s>.|
|1635|The backup statfile<%s> is invalid(CRC<H:%u, B:%u, T:%u).|
|1636|The backup <%s> is not completed.|
|1637|The backup <%s> has already exist.|
|1638|The end rid<%llu> of the table<%llu> in the restored database is invalid.|
|1639|The name<%s> of backup is too long, errno = %d.|
|1640|The backup file<%s> already exists.|
|1641|The backup file<%s> has the invalid magic string<%s>.|
|1642|The header of backup file<%s> has the invalid crc32<%u>.|
|1643|Length<%u> of backup file<%s> is too long.|
|1644|The page size <%u> of backup file<%s> is invalid.|
|1645|The file size <%llu> of the head is different from the size<%llu> on the disk.|
|1646|The backup file is invalid since the backup is not completed.|
|1647|Incremental backup should be preceded by the last backup.|
|1648|Backup targets are different from that of previous target.|
|1701|The tablespace<%s> is still referenced by other objects such as tables and indexes.|
|1702|The tablespace<%s> does not exist in the database.|
|1703|The SYSTEM_TABLESPACE cannot be dropped.|
|1704|Tablespace already exists. <%s>|
|1705|The dir<%s> for the tablespace<%s> of datadisk<%s> already exists.|
|1706|Disk<%s> does not exist in the tablespace<%s>.|
|1707|The parallel I/O of a disk should be between %d and %d.|
|1708|Failed to read <%ld> bytes from the file<%s>, errno = %d.|
|1709|The page<offset:%u, size:%u> of the file<%s> is invalid because it has the invalid timestamp<head:%lld, tail:%lld>|
|1710|The page<offset:%u, size:%u> of the file<%s> is invalid because it has the invalid crc<memory:%u, disk:%u>|
|1711|Failed to create directory<%s> for virtual disk.|
|1712|Failed to allocate memory for directory to be removed.|
|1801|Error in waiting to read value: value offset<%lld>, value size<%u>, and file<%s>|
|1821|The image in the DWFile<%s> is invalid.|
|1841|The operation is aborted by ART.|
|1851|Variable length columns are not allowed in tag table.|
|1852|Another deletion is in progress for table <%llX>.|
|1853|Cannot create append file for Key-Value table <%llX>, errno = %d.|
|1854|Cannot sync append file for Key-Value table <%llX>, errno = %d.|
|1855|Cannot close append file for Key-Value table <%llX>, errno = %d.|
|1856|Cannot create data file <%llX> for Key-Value table <%llX>, errno = %d.|
|1857|Cannot open data file <%llX> for Key-Value table <%llX>, errno = %d.|
|1858|Cannot read data file <%llX> for Key-Value table <%llX>, errno = %d.|
|1859|Cannot write data file <%llX> for Key-Value table <%llX>, errno = %d.|
|1860|Data file <%llX> is corrupted for Key-Value table <%llX>.|
|1861|Cannot create index file <%llX> for Key-Value table <%llX>, errno = %d.|
|1862|Cannot open index file <%llX> for Key-Value table <%llX>, errno = %d.|
|1863|Cannot read <.%s> file <%llX> for Key-Value table <%llX>, errno = %d.|
|1864|Cannot write <.%s> file <%llX> for Key-Value table <%llX>, errno = %d.|
|1865|Index file <%llX> is corrupted for Key-Value table <%llX>.|
|1866|Cannot perform I/O for Key-Value table <%llX>.|
|1867|Invalid path to append file for Key-Value table <%llX>, errno = %d.|
|1868|Cannot open append file for Key-Value table <%llX>, errno = %d.|
|1869|RID-based SELECT is not allowed without datafile, Table<%llX>/RID<%llu>.|
|1870|Cannot open append file for mounted Key-Value table <%llX>, errno = %d.|
|1871|Cannot read append file for mounted Key-Value table <%llX>, errno = %d.|
|1872|Another backup is in progress for table <%llX>.|
|1873|No index-file <%llx> for Key-Value table Table<%llX>.|
|1874|Cannot open file <%llX> for Key-Value table <%llX> path<%s>, errno = %d.|
|1875|Cannot find unpurged node for Key-Value table <%llX>.|
|1876|Fail to %s decompress file <%llX> for key-value table<%llx>, error = %d|
|1877|Tag stat for id[%llu] is not found.|
|1878|Cannot write stat file for Key-Value table <%llX> path<%s> errno = %d.|
|1879|Cannot read stat file for Key-Value table <%llX> path<%s> errno = %d.|
|1880|Cannot open stat file for Key-Value table <%llX> path<%s>, errno = %d.|
|1881|Stat File Invalid TableID[%llu], TablePath[%s].|
|1882|No kvindex-file <%llx> for Key-Value table Table<%llX>.|
|1883|Value of the time column(<%lld>) must be greater than or equal to <%lld>.|
|1884|keyvalue table<%llx> thread for [%s] stopped.|
|1885|Data row value is corrupted: required RID<%llu>, value RID<%llu>.|
|1886|%s varchar data is corrupted: required VRID<%u>, value VRID<%u>.|
|1900|Snapshot ID <%s> is invalid.|
|1901|Cannot snapshot with no table.|
|1902|Snapshot timed out.|
|1903|Snapshot ID <%s> does not exist.|
|1904|Snapshot ID <%s> already exists.|
|1910|Cannot freeze with no table.|
|1911|Snapshot already frozen.|
|1951|Failed to call function <%s>, errno=%d|
|1952|Table (0x%llx) resource busy (%s).|
|2000|Memory allocation error, Error code = %d|
|2001|Error in opening meta.|
|2002|Error in executing meta.|
|2003|Error in closing meta.|
|2004|Error in creating hash. (errno=%d)|
|2005|Error in allocating memory.|
|2006|Error in adding hash. (errno=%d)|
|2007|Error in fetching meta.|
|2008|Error in traversing hash.|
|2009|Insufficient parser memory.|
|2010|Syntax error: near token (%s).|
|2011|Unrecognized token (%s).|
|2012|Single row error. Single-row subquery returns more than one row. (NOT USED)|
|2013|A GROUP BY clause is required before HAVING.|
|2014|Column name is duplicated: (%s).|
|2015|Invalid column type: (%s).|
|2016|Table property (%s) does not exist.|
|2017|Error in converting table property. Cannot convert string (%s) to integer.|
|2018|Table property value is out of range: (%s).|
|2019|Column size should be specified on a variable column type.|
|2020|Invalid size specified. Cannot specify type size to (%s).|
|2021|Cannot create bitmap index on data type (%s)|
|2022|Cannot create keyword index on data type (%s)|
|2023|snprintf function error (%d).|
|2024|Table %s already exists.|
|2025|Table %s does not exist.|
|2026|The number of insert values and that of columns are mismatched.|
|2027|Error in table insert column integer conversion. Insert value conversion to integer error (%s).|
|2028|Error in table insert column double conversion. Insert value conversion to double error (%s)|
|2029|Error in table insert column time format. Insert _arrival_time value conversion error.|
|2030|Column name (%s) does not exist.|
|2031|Resource busy (%s).|
|2032|Type conversion error: error occurred while comparing the values of type (%s) and type (%s).|
|2033|Cannot concatenate non varchar types.|
|2034|Invalid format of time expression.|
|2035|Function [%s] does not exist.|
|2036|Function [%s] argument is mismatched.|
|2037|Function [%s] argument data type is mismatched.|
|2038|Table [%s] does not exist.|
|2039|No table specified in the target list.|
|2040|Invalid time range.|
|2041|Time value must be positive.|
|2042|Expression cannot have a NULL value.|
|2043|Group function is not allowed here.|
|2044|Not a GROUP BY expression.|
|2045|Type is not supported(typecode is %u). Internal error.|
|2046|String buffer is not enough.|
|2047|Lock buffer is not enough. Table counts are too many.|
|2048|Bind parameter count is overflowed. (max=%u)|
|2049|Cannot apply bind parameter.|
|2050|Bind data from client is corrupted.|
|2051|Bind data type unknown (typecode is %u).|
|2052|Cannot insert data into this table (%s).|
|2053|Failed to convert type (%s) to type (%s).|
|2054|Aggregation error on function usage (NOT USED)|
|2055|Invalid insert value.|
|2056|Column name (%s) not found.|
|2057|Only literal type can be used in SEARCH keyword.|
|2058|Index %s already exists|
|2059|Index %s does not exist|
|2060|Composite index is not supported.|
|2061|Cannot divide a value by zero.|
|2062|Cannot calculate date type.|
|2063|Invalid search type. Search type must be VARCHAR.|
|2064|Invalid time format. (format: \"year/mon/day hour:min:sec\")|
|2065|Index property (%s) dose not exist.|
|2066|Invalid index property value: (%s).|
|2067|Error in TO_ADDR4 function aggregate. Argument type to TO_ADDR4 function must be an integer.|
|2068|Invalid IPv4 address format (%s).|
|2069|%s index can only be created for %s table.|
|2070|Search predicate needs keyword index.|
|2071|Only one index is allowed for a single column.|
|2072|Cannot delete data from this table (%s).|
|2073|Invalid DELETE condition. %s|
|2074|Invalid delete time range. BEFORE time range should be older than present.|
|2075|Table(%s) record does not exist in meta database.|
|2076|Table(%lld) record does not exist in meta database.|
|2077|Invalid %s size. %s type size cannot be more than %d.|
|2078|Invalid statement type. Statement type(%d) is unsupported.|
|2079|The number of function (%s) arguments is not matched.|
|2080|User (%s) does not exist.|
|2081|Invalid username/password.|
|2082|User (%s) already exists.|
|2083|You cannot drop yourself(%s).|
|2084|User drop error. This user's tables still exist. Drop those tables first.|
|2085|The user(%s) does not have alter privileges.|
|2086|The user(%s) does not have connect privileges.|
|2087|The user does not have access privileges on table(%s.%s).|
|2088|Error in altering table. Only the LOG table can be altered.|
|2089|Error in altering table. Column name(%s) already exists.|
|2090|Error in altering table. Only varchar type can be modified.|
|2091|Error in altering table. Varchar length should be greater than previous value length|
|2092|Error in altering table. Column (%s) cannot be dropped.|
|2093|Error in altering table. Column (%s) having index cannot be dropped.|
|2094|Error in altering table. Column (%s) already exists.|
|2095|Error in truncating table. Only the LOG table can be truncated.|
|2096|Error in truncating table. Table %s does not exist.|
|2097|Error in altering table. The table must have at least one column.|
|2098|Error in joining tables. Only equi-join is allowed.|
|2099|Error in joining tables. The OR condition for a join predicate is not allowed.|
|2100|Error in joining tables. The join predicate cannot use functions.|
|2101|Error in joining tables. Cannot join without join predicate.|
|2102|Collector (%s.%s) does not exist.|
|2103|The collector (%s.%s) already exists.|
|2104|The template file (%s) does not exist.|
|2105|The template format (%s : %s : %d) is invalid.|
|2106|The collector (%s.%s) is already running.|
|2107|The collector (%s.%s) is not running.|
|2108|Error in loading collector template. The value (%s) is invalid.|
|2109|Cannot join two or more LOG tables.|
|2110|Search condition argument is too short. It needs more than (%d) characters.|
|2111|Invalid option.|
|2112|Cannot use DISTINCT with GROUP BY clause.|
|2113|Cannot use DISTINCT with aggregation function.|
|2114|DISTINCT clause is not allowed here.|
|2115|Internal column cannot be modified.|
|2116|Search predicate must use an index.|
|2117|DDL on table (%s) is forbidden.|
|2118|Lock object was already initialized. (Do not use select and append simultaneously in single session.)|
|2119|This functionality has not been implemented.|
|2120|Invalid session ID (%s).|
|2121|No privileges to kill the session.|
|2122|No privileges to cancel the session.|
|2123|Table id (%lld) does not exist in meta database.|
|2124|Column id (%llu) does not exist in table (%llu).|
|2125|Error in converting string (%s) to datetime with heuristic method. Check the default date string format in this session.|
|2126|ORDER BY clause is not allowed in a subquery|
|2127|Only integer constants must be used for ORDER BY column position.|
|2128|ORDER BY column position %d is out of range - should be between 1 and %d.|
|2129|GROUP BY terms must be integer constants|
|2130|A GROUP BY clause is required before HAVING|
|2131|Single row error. Single-row subquery returns more than one row.|
|2132|Cannot use subquery on HAVING, ORDER BY and GROUP BY clauses.|
|2133|Invalid subquery.|
|2134|Too many REGEXP in WHERE clause. No more than %d REGEXP in WHERE clause.|
|2135|WHERE clause has to return a boolean result.|
|2136|Invalid tablespace type.|
|2137|There are too many disks<%ud> for tablespace %s.|
|2138|The PARALLEL_IO value<%d> for the disk<%s> must be higher than <%d>.|
|2139|MINMAX CACHE is not allowed for VARCHAR column(%s).|
|2140|This type of tables do not support the tablespace functionality.|
|2141|Type comparison error.|
|2142|Cannot use lob type in the GROUP BY clause.|
|2143|Cannot use lob type in the ORDER BY clause.|
|2144|Outerjoin permits only 2 tables.|
|2145|The string cannot be converted to number value.(%s)|
|2146|Cannot join tables with timeseries function.|
|2147|Cannot use inline view with timeseries function.|
|2148|Invalid IPv6 address format.(%s)|
|2149|Error in executing CONTAINS. Cannot convert from type(%d) to type(%d).|
|2150|Network type error. Network Mask length does not match with the column's length.(mask=%s, column=%s)|
|2151|Error in adding disk to tablespace. You cannot use multiple disks for tablespace without valid license.|
|2152|Error in setting column property. You should specify a positive value of column property PARTITION_PAGE_COUNT as well as PAGE_VALUE_COUNT.|
|2153|Wrong column property name(%s). You should specify a valid property name.|
|2154|Select set operator parsing error.|
|2155|Only UNION ALL set operator is supported.|
|2156|Set operator column types are mismatched on (%d)th column.|
|2157|Internal error on validating query|
|2158|Error in evaluating data type. You must specify a valid data type.|
|2159|FROM DATETIME' must be earlier than 'TO DATETIME'.|
|2160|Error in doing unmount table(%s). You can unmount only mounted tables.|
|2161|Error in executing DDL. You cannot execute DDL with mounted DB. (*NOT USED*)|
|2162|Error in doing unmount DB. You cannot umount database which is not mounted.|
|2163|Invalid directory path (%s). You should specify a valid path.|
|2164|Invalid index property. Property (%s) for index cannot be altered.|
|2165|Function (%s) is not allowed here.|
|2166|Cannot use ORDER BY clause with aggregation function.|
|2167|GROUP_CONCAT function error. Separator should be a string constant.|
|2168|Operator argument count or type is mismatched.|
|2169|Invalid column property value: (%s)|
|2170|Every specified table or inline view in FROM clause must have its own alias.|
|2171|VOLATILE / LOOKUP table cannot have more than one primary key.|
|2172|Primary key is allowed only for VOLATILE / LOOKUP table.|
|2173|Cannot create columns with data type (%s) in VOLATILE / LOOKUP table.|
|2174|The index already exists in the column(%s).|
|2175|SET clause must be written as a list of 'column = value' expression.|
|2176|Cannot update primary key column in SET clause.|
|2177|ON DUPLICATE UPDATE clause is allowed only in LOOKUP / VOLATILE table.|
|2178|Error in updating table. Column name (%s) does not exist in this table.|
|2179|INSERT on a %s table without primary key value cannot be proceeded.|
|2180|Primary key is mandatory for UPDATE.|
|2181|Invalid index name starting with (%s) which is the same as primary key index.|
|2182|You cannot drop the primary key index (%s).|
|2183|Append mode for table (%s) is not supported.|
|2184|Specified property value is invalid in %s table.|
|2185|Invalid database name. This database name is already used for mount.|
|2186|Invalid database name.|
|2187|Error in unmounting database. Some tables in mounted database are accessed by other transactions|
|2188|The database is not mounted.|
|2189|Error in deleting rows. Only rows in VOLATILE / LOOKUP table can be deleted.|
|2190|Invalid UPDATE/DELETE condition. Specify it as (primary key column) = (value)|
|2191|WHERE clause in DELETE statement is not supported yet.|
|2192|Index type for keyword index only supports keyword bitmap or keyword LSM.|
|2193|Error in creating collector. The regular expression file (%s) does not exist.|
|2194|Error in creating collector. The regular expression file path has not specified.|
|2195|Buffer size insufficient.|
|2196|Error in loading data. File (%s) does not exist.|
|2197|Error in loading data with automatic mode. The table (%s) already exists.|
|2198|Error in loading data. The table (%s) doesn't exists.|
|2199|CSV parsing error occurred in %d line: [%s]|
|2200|[%s] is not valid string terminator or encloser.|
|2201|The automatic loading mode is invalid.|
|2202|The automatic column detection has been failed. No data or invalid headers.|
|2203|Invalid encoding.|
|2204|Failed to convert %s to UTF8.|
|2205|A modulo operator can be applied only for integer types.|
|2206|Tablespace name cannot be specified in non automode|
|2207|Error in saving table into file (%s). File already exists.|
|2208|Expression argument type is mismatched.|
|2209|Error in connecting to collector manager. The collector manager (%s:%d) doesn't exists.|
|2210|Error in executing CREATE command. The collector manager (%s) returns error.|
|2211|Error in executing DROP command. The collector manager (%s) returns error.|
|2212|Error in running collector manager. The collector manager (%s) is already running.|
|2213|Error in stopping collector manager. The collector manager (%s) is not running.|
|2214|Error in starting collector manager. The collector manager (%s) is already started.|
|2215|Error in starting command execution. The collector manager (%s) returns error.|
|2216|Error in executing collector CREATE command. The collector (%s.%s) returns error.|
|2217|Error in receiving meta data from collector manager (%s-%s:%d).|
|2218|Collector manager (%s) does not exist.|
|2219|Error in creating collector manager. The collector manager (%s) already exists.|
|2220|Error in executing command. The collector manager (%s) returns error.|
|2221|Manager name is not specified.|
|2222|Error in read protocol. Send %s protocol, but received %d protocol.|
|2223|Unable to establish connection with collectormanager (%s).|
|2224|Manager name is not specified.|
|2225|Invalid set column unit.|
|2226|Invalid character ('%c').|
|2227|The collector source (%s.%s) already exists.|
|2228|Invalid procedure (%s).|
|2229|Invalid argument value for function (%s).|
|2230|Wrong number of arguments in call to '%s'.|
|2231|strcpy function error (%d).|
|2232|Calculation argument type (%s), (%s) error.|
|2233|Error occurred at column (%u): (%s)|
|2234|Set operator columns counts are mismatched by %d and %d.|
|2235|SERIES BY clause is not allowed here.|
|2236|For a table list in FROM clause, The number of tables should be less than 32.|
|2237|The index <%s> is not an index for the table <%s>.|
|2238|This type of join is not allowed.|
|2239|Invalid use of aggregation function.|
|2240|Cannot fetch column with type (%s).|
|2241|Join between LOG table and fixed table is not supported in Cluster Edition.|
|2242|Only equal predicate for joining LOG tables is available in Cluster Edition.|
|2243|DELETE statement with the number of rows is not supported in Cluster Edition.|
|2244|Allocate collector columns meta failure.|
|2245|Allocate collector column target failure.|
|2246|Identifier %.*s is too long.|
|2247|DATETIME earlier than 1970-01-01 00:00:00 (UTC) is not valid.|
|2248|Insufficient column definitions.|
|2249|Invalid DELETE condition.|
|2250|You cannot execute DDL on compoment table/index of TAGDATA table explictly.|
|2251|You cannot define columns with duplicate flag (%s) in TAGDATA table.|
|2252|Invalid column type (%s) for flag (%s) in TAGDATA table.|
|2253|Mandatory column definition (PRIMARY KEY / BASETIME) is missing.|
|2254|Column flag (%s) is only allowed for TAG table.|
|2255|Primary key of TAGDATA table is not defined in metadata.|
|2256|Metadata column definition is allowed only in TAGDATA table.|
|2257|Metadata insertion is allowed only in TAGDATA table.|
|2258|Metadata key (%.*s) for the TAG table has already been inserted.|
|2259|Metadata of TAGDATA table is not found. (Key = %s)|
|2260|Failed to allocate new metadata of TAGDATA table (Current Size=%llu).|
|2261|You cannot insert metadata into TAGDATA table with ON DUPLICATE KEY UPDATE clause.|
|2262|Direct DML on component tables of TAGDATA table is not allowed.|
|2263|You can create only one TAGDATA table.|
|2264|Cannot read a column (%s) in ROLLUP query because it is not a ROLLUP column.|
|2265|Reading TAGDATA table without primary key condition is not allowed.|
|2266|You cannot delete raw data of TAGDATA table with WHERE condition.|
|2267|Primary key in TAGDATA table should be compared by '=' or 'IN' operation.|
|2268|Primary key in TAGDATA table should be compared with constant value.|
|2269|Outerjoin on TAGDATA table is not allowed.|
|2270|TAGDATA table's name should be 'TAG'.|
|2271|You must insert key value of TAGDATA table as constant.|
|2272|Index id (%llu) does not exist in meta database.|
|2273|The user does not have privileges on TAGDATA DDL.|
|2274|Failed to free new metadata of TAGDATA table.|
|2275|The INSERT SELECT statement to the TAGDATA table is not allowed in enterprise edition.|
|2276|Component table (%s) of TAGDATA table already exists.|
|2277|Table or index name that starts with '_TAG' is reserved.|
|2278|UPDATE statement is not allowed for %s.|
|2279|Invalid tag name insertion to TAGDATA table (name = '%s').|
|2280|Invalid tag name insertion due to wrong bind variable.|
|2281|DURATION clause is not applicable on %s.|
|2282|The DELETE statement for table '%s' is already been executed.|
|2283|IN subquery on TAGDATA table is not allowed.|
|2284|Internal NULL value exists in the condition expression.|
|2285|Internal error: %s.|
|2286|Memory allocation failed while creating TAGDATA table. You may need to decrease TAG_DATA_PART_SIZE in machbase.conf.|
|2287|TAGDATA name value (%s) is too long.|
|2288|Aggregate function is expected at (%.*s).|
|2289|Non-constant expression is not allowed for PIVOT values.|
|2290|%s cannot be altered.|
|2291|Table name 'TAG' must be used for TAGDATA table.|
|2292|The order of columns in TAGDATA table must be (PRIMARY, BASETIME, SUMMARIZED, other columns, .. ).|
|2293|Column meta for bind param[%d] is not available.|
|2294|JOIN more than one TAGDATA table is not supported.|
|2295|Invalid ordinal number ID_COLUMN (%lld) and TIME_COLUMN (%lld).|
|2296|Interpolation requires only one BETWEEN expression.|
|2297|BETWEEN has invalid expression (%s).|
|2298|FREQUENCE must be a factor of INTERPOLATION_INTERVAL (%lld).|
|2299|Only BETWEEN condition is supported.|
|2300|Interpolation column is missing. (%s)|
|2301|Some properties are missing for interpolation.|
|2302|Invalid interpolation interval property: %lld.|
|2303|You must use higher ROLLUP unit.|
|2304|Interpolation is not applicable on (%s).|
|2305|JOIN is not applicable for interpolation.|
|2306|Interpolation interval value(%lld) should be less than checkpoint interval value(%lld).|
|2307|Interpolation interval value(%lld) should be less than ROLLUP unit (%s).|
|2308|Checkpoint interval value(%lld) should be less than ROLLUP unit (%s).|
|2309|Checkpoint interval value(%lld) should be divide by interpolation value(%lld).|
|2310|Invalid interpolation checkpoint property: %lld.|
|2311|%s cannot be used in interpolation query.|
|2312|Rollup delete can only be done on the Interpolation Tag table.|
|2313|Unable to execute ROLLUP DELETE with the given range.|
|2314|Regular duration backup does not support a backup of the TAG table (Try incremental backup which permits the action on the TAG table).|
|2315|Snapshot is not supported.|
|2316|Invalid expression in DURATION clause: %.*s|
|2317|Function execution failed: %s (errno=%d)|
|2318|Can not connect Lookup Node|
|2319|Error on Lookup Node|
|2320|Lookup Node is not ready|
|2321|Data is not found in Lookup Node.|
|2322|Mandatory column definition (PRIMARY KEY) is missing.|
|2323|Table REFRESH is not applicable on %s.|
|2324|Cannot delete tagmeta. there exist data with deleted_tag key.|
|2325|Integer %s type overflow.|
|2326|Backup/Mount is not supported.|
|2327|Pivot is not supported in rollup query.|
|2328|Cannot insert a new tag since the number of tags has exceeded MAX_TAG_COUNT(%lld).|
|2329|Cannot insert a new tag since the number of tags has exceeded TAG_COUNT_LIMIT(%lld).|
|2330|Mandatory column definition (ULONG / DATETIME) is missing.|
|2331|RANGE expression is not applicable on the table (%s).|
|2332|Unable to create an index on the column (%s).|
|2333|This column(%s) can't be more than %d.|
|2334|Tag Index is not yet supported.|
|2335|Failed to delete all on this table. It is recommended to use EXEC TABLE_REFRESH(%s).|
|2336|CASCADE option is not applicable on %s.|
|2337|Unable to define more than one column attribute (%s).|
|2339|The type of %s column (%s) is different from that of VALUE column (%s).|
|2340|SUMMARIZED column does not exist for %s.|
|2341|SUMMARIZED value is greater than UPPER LIMIT.|
|2342|SUMMARIZED value is less than LOWER LIMIT.|
|2343|LOWER LIMIT must not be greater than UPPER LIMIT.|
|2344|Not numeric type. (%s)|
|2345|Column flag (%s) is only allowed for TAGMETA table.|
|2346|Column type (%s) is not allowed for default value.|
|2347|SYSDATE is only allowed for default value.|
|2348|Alter table set %s not support on cluster.|
|2349|Bind variable is not supported for new tag.|
|2350|The function (%s) requires OVER clause.|
|2351|Window function is allowed only in SELECT list.|
|2352|OVER clause is not applicable on (%s).|
|2353|Invalid data type (%s) in OVER clause.|
|2354|Constant is not allowed in OVER clause.|
|2355|Type (%s) is not supported.|
|2356|Origin must be the first day of the month.|
|2600|SEQUENCE property is not applicable in the table.|
|2601|Invalid function in a SEQUENCE column. NEXTVAL must be used.|
|2602|NEXTVAL is applicable only in INSERT statement.|
|2603|NEXTVAL is applicable only in SEQUENCE columns.|
|2604|Sequence column must be LONG type.|
|2651|Dependent ROLLUP table exists.|
|2652|Not a ROLLUP table. (%s)|
|2653|Rollup interval must be greater than source rollup interval.|
|2654|ROLLUP (%s) is not found.|
|2655|Rollup interval source rollup interval Must Divide Zero.|
|2656|Rollup interval must positive integer.|
|2657|Rollup interval must be smaller than year.|
|2658|ROLLUP is not enabled for %s.|
|2659|Rollup maximum count is 100.|
|2670|Rollup user ID(%d) is not equal to Source user ID(%d)|
|2671|Invalid type for ROLLUP column (%s).|
|2672|Json path is not specified on %s.|
|2673|Json path is not applicable on %s.|
|2674|ROLLUP query must have a target column.|
|2675|Cannot use more than one ROLLUP column in a ROLLUP query.|
|2676|Not a TAG table.|
|2677|Invalid rollup time unit (%s).|
|2678|WITH ROLLUP requires a SUMMARIZED column.|
|2679|Failed to create ROLLUP by WITH ROLLUP option.|
|2680|ROLLUP (%s) is already started.|
|2681|ROLLUP (%s) is already stopped.|
|2682|ROLLUP extension type is different.|
|2683|Cannot read a column (%s) in ROLLUP query because it is not a ROLLUP time column.|
|2684|Dependent ROLLUP table does not exist.|
|2685|There are no applicable ROLLUP tables.|
|2700|Policy (%s) already exists.|
|2701|Policy (%s) does not exists.|
|2702|Policy (%s) is in use.|
|2703|Table (%s) has no retention policy.|
|2704|Table (%s) already has a retention policy.|
|2705|Retention duration must be longer than 1 day.|
|2706|Retention interval must be longer than 1 hour.|
|2707|Retention is not applicable on the table (%s).|
|2708|Only SYS user can create or drop RETENTION.|
|2800|The stream query is too long. (max=2048)|
|2801|The stream query is not applicable.|
|2802|Cannot get table information.|
|2803|Specified stream query statement has no LOG table.|
|2804|Specified stream query statement has more than 1 LOG tables.|
|2805|The stream query (%s) is not found.|
|2806|The stream query (%s) already exists.|
|2807|The stream query (%s) is already started.|
|2808|The stream query (%s) is already stopped.|
|2809|The stream query (%s) is still running.|
|2810|Cannot execute the stream query (%s).|
|2811|The stream query (%s) is executed automatically by the system.|
|2812|The Frequency clause is not allowed here.|
|2813|Invalid ROLLUP expression. (Token = %s, Unit = %ld)|
|2814|Invalid ROLLUP target. BASETIME column of TAGDATA table is the only target.|
|2815|Different ROLLUP expressions are used in a single SELECT query.|
|2816|Only rollup column with aggregate function can be referenced in ROLLUP SELECT query.|
|2817|ROLLUP expression must be used in SELECT query.|
|2818|Invalid ROLLUP target (%s).|
|2819|ROLLUP thread is running.|
|2820|ROLLUP thread is not running.|
|2821|Another DDL/DELETE/SNAPSHOT is in progress.|
|2822|Invalid expression in ROLLUP query : %.*s|
|2823|Invalid table in ROLLUP query: %s|
|2825|Extended column(%s) cannot be used in ROLLUP query.|
|2826|User (%s) can't revoke from table (%s.%s).|
|2827|User does not have grant privileges.|
|2828|User does not have revoke privileges.|
|2829|Only SYS user can create or drop user.|
|2830|The user does not have (%s) privilege on table(%s.%s).|
|2831|You can't grant UPDATE privilege on Log Table.|
|2832|You can't revoke UPDATE privilege on Log Table.|
|2833|You can only grant SELECT privileges on Mounted database.|
|3000|Statement ID overflow (Limit = %u, Curr = %u).|
|3001|Statement query length is zero.|
|3002|Task pool initialization error.|
|3003|Statement pool initialization error.|
|3004|Queue creation error.|
|3005|Statement allocation error.|
|3006|Unknown meta type error (typecode is %u). Internal error.|
|3007|Insufficient protocol buffer size. Increase it.|
|3008|Invalid protocol state. Check your application again. (Protocol = %s, State = %s)|
|3009|Invalid execute protocol data (%s).|
|3010|Error in fetch protocol: not enough buffer size to execute it. Increase the size.|
|3011|Send error.|
|3012|Memory allocation error.|
|3013|Invalid table name for append table. Table name is omitted.|
|3014|Invalid append protocol data (%s).|
|3015|Endian is not specified for append. Check endian information.|
|3016|Too many columns are specified for append. Cannot append more than %d columns|
|3017|Too large record size for append. Cannot append more than %d bytes per record.|
|3018|It exceeds the specified max. block size (%d). Check append data structure of your applcation.|
|3019|Explain plan error. Use it for SELECT statement only.|
|3020|Explain plan is not allowed in prepared mode.|
|3021|Protocol versions are not matched: Server (%d.%d.%d) Client (%d.%d.%d)|
|3022|Failed to get handle limit from the system.|
|3023|Handle limit(%d) from the system is less than that of property(%d). Tune system handle limit or decrease the property 'HANDLE_LIMIT'|
|3024|Invalid session ID (%llu).|
|3025|Not enough privileges to manipulate the session. (%llu)|
|3026|You should log in with the same user name in the target session. Now (%d) Target(%d)|
|3027|This statement has been canceled.|
|3028|Invalid session property name. Name (%s) does not exist.|
|3029|Error in converting session property (%s). Cannot convert string (%s) to integer.|
|3030|Invalid session property value. Check the session value (%s)|
|3031|Protocol error.|
|3032|Error in getting license meta. Check DB image and binary.|
|3033|Error in opening meta.|
|3034|Error in executing meta.|
|3035|Error in closing meta.|
|3036|The license is expired(%s).|
|3037|The license is invalid or the license file does not exist(%s).|
|3038|License violation detected. You have exceeded your license limit(%u) or expiry date(%s). For more information, contact sales@machbase.com.|
|3039|Session count exceed: (maxcount =%llu).|
|3040|Unable to shutdown since the server is busy.|
|3041|AppendBatch error: %s.|
|3042|Recovery in progress.|
|3043|Array Execute is not applicable for SELECT query.|
|3044|Wrong TIMEZONE string error: %s.|
|3045|Invalid context at %s.|
|3046|Communication module error (rc=%d): [%s].|
|3047|Failed to call function %s (rc=%d)|
|3100|Append Operation is not supported in Cluster Edition|
|3101|Can't convert (%s) to number|
|3102|Can't decode in base64|
|3103|not supported JSON type (%d)|
|3104|Column count for append is not matched with Meta|
|3105|The table type for append is not valid|
|3106|the JSON format is not valid|
|3107|Can't get the meta info for append table|
|3108|Can't read data from or write data into network buffer|
|3109|The URL (%s) is not valid format|
|3110|Timeout in waiting for response|
|3111|Can't connect to proxy server (%s:%d)|
|3112|Tag name in JSON must be string type|
|3113|Tag name (%s) does not exist in meta list|
|3114|The HTTP memory request exceeded the limit of (%lld). Change the property (HTTP_MAX_MEM) and restart server|
|3115|Table name is omitted from the JSON format|
|3116|Wrong Base64 Format Received|
|3117|Only Basic Authorization Accept To Login.|
|3118|There is No Authorization Header.|
|3119|The value of tag default column<%d> must not be NULL|
|3120|The timezone (%s) is invalid.|
|3121|Fail to get json object|
|3122|There is no argument for Rest API|
|3123|Count value is not valid for Rest API|
|3124|Interval value is not valid for Rest API|
|3125|Interval type is not valid for Rest API|
|3126|The requested URL for the REST API is not valid|
|3127|The requested URL for the REST API is not supported|
|3128|TAG_STAT is not available for table %s.|
|3200|Server is not running.|
|3201|Invalid statement state: (%d)|
|3202|Column index is out of range.|
|3203|The length of data exceeded the size of buffer.|
|3204|Append data ip string is null.|
|3205|Append data datetime string(%s) is null.|
|3206|Invalid column type (%d).|
|3207|Invalid statement type (%d).|
|3208|Server thread error: %d - %s|
|3209|statement is busy. (%d)|
|3210|This connection has been already disconnected|
|3211|Database already exists.|
|3212|Database does not exist.|
|3213|Server is running.|
|3214|Failed to open dbs(%s) directory.|
|3215|ALTER SESSION statement is not supported.|
