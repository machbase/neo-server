# machdeployeradmin

You can check the status of the Deployer, or directly issue the Deployer's startup/shutdown/stop commands.

Normally the fastest way to issue the commands is through machcoordinatoradmin, but if not possible, you must do the following.

Only exits in Cluster Edition Package.

## Options and Features

The options for machdeployeradmin are as follows. The functions described in the previous section are omitted.

```
mach@localhost:~$ machdeployeradmin -h
```

|Options|Description|
|--|--|
|-u, --startup | Runs Deployer process|
|-s, --shutdown | Terminates Deployer process|
|-k, --kill | Stops Deployer process|
|-c, --createdb | Creates Deployer meta|
|-d, --destroydb | Deletes Deployer meta|
|-i, --silence | Runs without output|
|-e, --check | Checks to see if Deployer process is running|

## Checking Running Status

Example:

```
mach@localhost:~$ machdeployeradmin -e
-------------------------------------------------------------------------
     Machbase Deployer Administration Tool
     Release Version - e3c0717.develop
     Copyright 2014, MACHBASE Corp. or its subsidiaries
     All Rights Reserved
-------------------------------------------------------------------------
Machbase Deployer is running with pid(29373)!
```
