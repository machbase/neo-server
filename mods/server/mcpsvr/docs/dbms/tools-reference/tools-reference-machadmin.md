# machadmin

machadmin is used to start up or shut down the Machbase server and to check the creation, deletion, and execution status.

## Option and Features

The options for machadmin are as follows. The functions described in the previous installation section are omitted.

```bash
mach@localhost:~$ machadmin -h
```

| Options| Describe |
|--|--|
|-u, --startup/ --recovery[=simple,complex,reset]|Machbase server startup/recovery mode (default: simple)
|-s, --shutdown | |Machbase server shuts down  normally |
|-c, --createdb |Creates Machbase database |
| -d, --destroydb| Deletes Machbase database |
| -k, --kill| Force quits Machbase server |
| -i, --silence| Runs without output |
| -r, --restore |Recovers database from backup
| -x, --extract| Converts backup files to backup directory |
|-e, --check| Checks Machbase server run status |
|-t, --licinstall| Installs license file |
|-f, --licinfo| Outputs installed license information|

## Recovery Mode

Syntax

```
machadmin -u --recovery=[simple | complex | reset]
```

The recovery mode is as follows:

* simple: If there is no power loss when the server is running, simple recovery mode is run by default. 
* complex: The complex recovery mode takes longer to execute than the simple mode. It is executed by default when restarting after the power is turned off.
* reset: When recovery is not performed in simple or complex mode, all data in all tables are checked to recover the database. In this case, some loss of data may occur.

## Server Normal Shutdown

Example:

```
mach@localhost:~$ machadmin -s
 
-----------------------------------------------------------------
     Machbase Administration Tool
     Release Version - 5.1.9.community
     Copyright 2014, MACHBASE Corp. or its subsidiaries
     All Rights Reserved
-----------------------------------------------------------------
Waiting for the server shut down...
Server shut down successfully.
```

## Create Database

Example:

```
mach@localhost:~$ machadmin -c
-----------------------------------------------------------------
     Machbase Administration Tool
     Release Version - 5.1.9.community
     Copyright 2014, MACHBASE Corp. or its subsidiaries
     All Rights Reserved
-----------------------------------------------------------------
Database created successfully.
```

## Delete Database

Example:

```
mach@localhost:~$ machadmin -d
-----------------------------------------------------------------
     Machbase Administration Tool
     Release Version - 5.1.9.community
     Copyright 2014, MACHBASE Corp. or its subsidiaries
     All Rights Reserved
-----------------------------------------------------------------
Destroy Machbase database- Are you sure?(y/N) y
Database destroyed successfully.
```

## Force to abort Server

Syntax:

```
machadmin -k
```

Example:

```
mach@localhost:~$ machadmin -k
-----------------------------------------------------------------
     Machbase Administration Tool
     Release Version - 5.1.9.community
     Copyright 2014, MACHBASE Corp. or its subsidiaries
     All Rights Reserved
-----------------------------------------------------------------
Waiting for Machbase terminated...
Server terminated successfully.
```

## Run Silent Mode

Removes the message that is output when 'machadmin'  runs.

Syntax:

```
machadmin -i
```

## Database Recovery

Syntax:

```
machadmin -r backup_database_path
```

Example:

```
mach@localhost:~$ machadmin -r 'backup'
-----------------------------------------------------------------
     Machbase Administration Tool
     Release Version - 5.1.9.community
     Copyright 2014, MACHBASE Corp. or its subsidiaries
     All Rights Reserved
-----------------------------------------------------------------
Backed up database restored successfully.
```

## Check server is running

Syntax:

```
machadmin -e
```

Example when server is not running:

```
mach@localhost:~$ machadmin -e
-----------------------------------------------------------------
     Machbase Administration Tool
     Release Version - 5.1.9.community
     Copyright 2014, MACHBASE Corp. or its subsidiaries
     All Rights Reserved
-----------------------------------------------------------------
[ERR] Server is not running.
```

Example when server is running:

```
mach@localhost:~$ machadmin -e
-----------------------------------------------------------------
     Machbase Administration Tool
     Release Version - 5.1.9.community
     Copyright 2014, MACHBASE Corp. or its subsidiaries
     All Rights Reserved
-----------------------------------------------------------------
Machbase server is already running with PID (14098).
```

## Install License File

Syntax:

```
machadmin -t license_file
```

Example:

```
mach@localhost:~$ machadmin -t license.dat
-----------------------------------------------------------------
     Machbase Administration Tool
     Release Version - 5.1.9.community
     Copyright 2014, MACHBASE Corp. or its subsidiaries
     All Rights Reserved
-----------------------------------------------------------------
License installed successfully.
```

## Check License

Example:

```
mach@localhost:~$ machadmin -f
-----------------------------------------------------------------
     Machbase Administration Tool
     Release Version - 5.1.9.community
     Copyright 2014, MACHBASE Corp. or its subsidiaries
     All Rights Reserved
-----------------------------------------------------------------
                   INFORMATION
Install Date                      : 2018-12-20 11:34:43
Company#ID-ProjectName            : machbase
License Policy                    : CORE
License Type(Version 2)           : OFFICIAL
Host ID                           : FFFFFFFFFFFFFFF
Issue Date                        : 2013-03-25
Expiry Date                       : 2037-03-18
Max Data Size For a Day(GB)       : 0
Percentage Of Data Addendum(%)    : 0
Overflow Action                   : 0
Overflow Count to Stop Per Month  : 0
Stop Action                       : 0
Reset Flag                        : 0
-----------------------------------------------------------------
                   STATUS
Usage Of Data(GB)                 : 0.000000
Previous Checked Date             : 2018-12-22
Violation Count                   : 0
Stop Enabled                      : 0
-----------------------------------------------------------------
License information displayed successfully.
```
