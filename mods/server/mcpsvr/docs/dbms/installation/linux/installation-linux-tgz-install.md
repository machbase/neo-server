# Tarball Installation

## Create User

Create a Linux user 'machbase' for installing and using  Machbase.

```bash
sudo useradd machbase
```

After setting the password,  log in as 'machbase' account.

## Package Installation

Create a directory called 'macbase_home' and download and install the package from the Machbase download site.

```bash
[machbase@localhost ~]$ wget http://machbase.com/dist/machbase-fog-x.x.x.official-LINUX-X86-64-release.tgz
[machbase@localhost ~]$ mkdir machbase_home
[machbase@localhost ~]$ mv machbase-fog-x.x.x.official-LINUX-X86-64-release.tgz machbase_home/
[machbase@localhost ~]$ cd machbase_home/
[machbase@localhost machbase_home]$ tar zxf machbase-fog-x.x.x.official-LINUX-X86-64-release.tgz
 
[machbase@loclahost machbase_home]$ ls -l
drwxrwxr-x  5 machbase machbase        64 Oct 30 16:10 3rd-party
drwxrwxr-x  2 machbase machbase      4096 Oct 30 16:10 bin
drwxrwxr-x  2 machbase machbase       306 Jan  2 11:36 conf
drwxrwxr-x  2 machbase machbase       136 Jan  2 11:37 dbs
drwxrwxr-x  3 machbase machbase        22 Oct 30 16:10 doc
drwxrwxr-x  2 machbase machbase        96 Oct 30 16:10 include
drwxrwxr-x  2 machbase machbase        29 Oct 30 16:10 install
drwxrwxr-x  2 machbase machbase       283 Oct 30 16:10 lib
-rw-rw-r--  1 machbase machbase 139888377 Dec 20 11:33 machbase-fog-x.x.x.official-LINUX-X86-64-release.tgz
drwxrwxr-x  2 machbase machbase        22 Dec 21 15:43 msg
 
drwxrwxr-x  2 machbase machbase         6 Oct 30 16:10 package
drwxrwxr-x 12 machbase machbase       140 Oct 30 16:10 sample
drwxrwxr-x  2 machbase machbase      4096 Jan  2 09:37 trc
drwxrwxr-x 10 machbase machbase       160 Oct 30 16:10 tutorials
drwxrwxr-x  3 machbase machbase        19 Oct 30 16:10 webadmin
 
[machbase@loclahost machbase_home]$
```

The directory descriptions installed are as follows.

|Direcotry|Description|
|--|--|
|bin|Executable files|
|conf|Configuration files|
|dbs|Data storage space|
|doc|License files|
|include|Various header files for the CLI program|
|install|mk files for Makefile|
|lib|Various libraries|
|msg|Machbase server error messages|
|package|(Cluster edition) Path to save the added package|
|sample|Various example files|
|trc|Machbase server logs and trace contents|
|webadmin|MWA Web Server Files|
|3rd-party|Grafana plug-in files|

## Set Environment Variable

Add Machbase-related environment variables to your .bashrc file.

```bash

export MACHBASE_HOME=/home/machbase/machbase_home
export PATH=$MACHBASE_HOME/bin:$PATH
export LD_LIBRARY_PATH=$MACHBASE_HOME/lib:$LD_LIBRARY_PATH
 
# Apply the changes with the following command.
source .bashrc
```

## Set Machbase Property

The $MACHBASE_HOME/conf directory contains the file macbase.conf.sample.

```bash

[machbase@localhost ~]$ cd $MACHBASE_HOME/conf
[machbase@localhost conf]$ ls -l
-rw-rw-r-- 1 machbase machbase   106 Oct 30 16:10 machtag.sql.sample
-rw-rw-r-- 1 machbase machbase 17556 Oct 30 16:10 machbase.conf.sample
 
[machbase@localhost conf]$
```

You can also change the Machbase connection port number using the Linux environment variable. Below is an example of switching to a different port number (7878)  other than the default value (5656).

```bash
export MACHBASE_PORT_NO=7878
```

## Machbase Simple Usage

### Create Database

To create the database, use the machadmin utility. You can see the command with the --help option.

```bash
[machbase@localhost machbase_home]$ machadmin --help
-----------------------------------------------------------------
     Machbase Administration Tool
     Release Version - x.x.x.official
     Copyright 2014, MACHBASE Corp. or its subsidiaries
     All Rights Reserved
-----------------------------------------------------------------
<< available option lists >>
  -u, --startup                         Startup Machbase server.
      --recovery[=simple,complex,reset] Recovery mode. (default: simple)
  -s, --shutdown                        Shutdown Machbase server.
  -c, --createdb                        Create Machbase database.
  -d, --destroydb                       Destroy Machbase database.
  -k, --kill                            Terminate Machbase server.
  -i, --silence                         Produce less output.
  -r, --restore                         Restore Machbase database.
  -x, --extract                         Extract BackupFile to BackupDirectory.
  -w, --viewimage                       Display information of BackupImageFile.
  -e, --check                           Check whether Machbase Server is running.
  -t, --licinstall                      Install the license file.
  -f, --licinfo                         Display information of installed license file.
 
[machbase@localhost machbase_home]$
```

Create a database with the -c option.

```bash

[machbase@localhost machbase_home]$ machadmin -c
-----------------------------------------------------------------
     Machbase Administration Tool
     Release Version - x.x.x.official
     Copyright 2014, MACHBASE Corp. or its subsidiaries
     All Rights Reserved
-----------------------------------------------------------------
Database created successfully.
[machbase@localhost machbase_home]$
```

### Launch Machbase Server

Run the Machbase server with the -u option.

```bash

[machbase@localhost machbase_home]$ machadmin -u
-----------------------------------------------------------------
     Machbase Administration Tool
     Release Version - x.x.x.official
     Copyright 2014, MACHBASE Corp. or its subsidiaries
     All Rights Reserved
-----------------------------------------------------------------
Waiting for Machbase server start.
Machbase server started successfully.
[machbase@localhost machbase_home]$
```

You can see that the server daemon, machbased, is running through the ps command as shown below.

```bash
[machbase@localhost machbase_home]$  ps -ef |grep machbased
machbase 11178     1  2 11:25 ?        00:00:01 /home/machbase/machbase_home/bin/machbased -s --recovery=simple
machbase 11276  9867  0 11:26 pts/1    00:00:00 grep --color=auto machbased
[machbase@localhost machbase_home]$
```

### Machbase Server Connection

Connect to the Machbase server using an access utility called machsql.

The administrator account SYS is ready and the password is set to MANAGER.

```bash
[machbase@localhost machbase_home]$  machsql
=================================================================
     Machbase Client Query Utility
     Release Version x.x.x.official
     Copyright 2014 MACHBASE Corporation or its subsidiaries.
     All Rights Reserved.
=================================================================
Machbase server address (Default:127.0.0.1) :
Machbase user ID  (Default:SYS)
Machbase User Password :
MACHBASE_CONNECT_MODE=INET, PORT=5656
Type 'help' to display a list of available commands.
Mach>
```

Let's create a simple table and input / output data.

```sql
create table hello( id integer );
insert into hello values( 1 );
insert into hello values( 2 );
select * from hello;
select _arrival_time, * from hello;
```

```sql
Mach> create table hello( id integer );
Created successfully.
Elapsed time: 0.054
Mach> insert into hello values( 1 );
1 row(s) inserted.
Elapsed time: 0.000
Mach> insert into hello values( 2 );
1 row(s) inserted.
Elapsed time: 0.000
Mach> select * from hello;
ID
--------------
2
1
[2] row(s) selected.
Elapsed time: 0.000
Mach> select _arrival_time, * from hello;
_arrival_time                   ID
-----------------------------------------------
2019-01-02 11:33:00 122:806:804 2
2019-01-02 11:32:57 383:848:361 1
[2] row(s) selected.
Elapsed time: 0.000
Mach>
```

The above SELECT results show that the most recently input data is displayed first.

Also, it can be seen through the _arrival_time column that the input time of the record is set to the nanosecond.

### Stop Machbase Server

Shut down the Machbase server with the -s option.

```bash
[machbase@localhost machbase_home]$ machadmin -s
-----------------------------------------------------------------
     Machbase Administration Tool
     Release Version - x.x.x.official
     Copyright 2014, MACHBASE Corp. or its subsidiaries
     All Rights Reserved
-----------------------------------------------------------------
Waiting for Machbase server shut down...
Machbase server shut down successfully.
[machbase@localhost machbase_home]$
```

### Delete Database

Delete the database with the -d option.
**Be very careful because all data will be deleted.**

```bash
[machbase@localhost machbase_home]$ machadmin -d
-----------------------------------------------------------------
     Machbase Administration Tool
     Release Version - x.x.x.official
     Copyright 2014, MACHBASE Corp. or its subsidiaries
     All Rights Reserved
-----------------------------------------------------------------
Destroy Machbase database. Are you sure?(y/N) y
Database destoryed successfully.
[machbase@localhost machbase_home]$
```
