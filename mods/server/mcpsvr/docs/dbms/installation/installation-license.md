# License Installation

License key installation is usually performed after MACHBASE installation is finished. If you do not install a specific license after installation, you can use MACHBASE with some restrictions. This section describes MACHBASE's license policy, structure, and installation method.

## License File Structure

The MACHBASE license is managed in the license.dat file. Licenses purchased for the product or for testing are displayed in a text file.

```bash
mach@localhost:~$ cat license.dat 
 
\#Company\#ID-ProjectName: test\#0-Machbase 
 \#License Policy: SIZE4DAY 
 \#License Type \(Version 2\): OFFICIAL 
 \#Issue DATE: 20160216 
 \#Expiry DATE: 20160319 
 \#Tag Count Limit: 0
 VBz5h4TC-d3+Bf3Efkpdp/Tx873PpZA-78LRSdrxbPY-xhGf4355/iXaY5/jfnn+Jdpjn+N+ef4l2mOf4355/iXaY5/jfnn+Jdpjn+N+ef4l2mOf4355/iXaY5/jfnn+Jdpjn+N+ef4l2mOf4355/iXaY5/jfnn+Jdpjn+
```

## No License File

The server will run even if there is no license, but there are some limitations. The server can only be used for evaluation purposes, so if you intend to use it formally, you must obtain the license in a legitimate procedure.

If there is no license file, the following functional limitations will exist.

    1. If you enter more than 100 million records through the Append protocol in one session, a warning message is displayed. Append input is then stopped. The input limit state is released only when the server is restarted.

    2. When creating a tablespace, you can not create more than two disk directories. If you use more than one disk, the following warning indicating that the parallel I / O function for high performance data input can not be used will be displayed. 

```bash
CREATE TABLESPACE tbs1 DATADISK disk1 (disk_path="tbs1_disk1"), disk2 (disk_path="tbs1_disk2"), disk3 (disk_path="tbs1_disk3");
[ERR-00867 : Error in adding disk to tablespace. You cannot use multiple disks for tablespace without valid license.]
```

## License Installation

The MACHBASE license must be installed in $MACHBASE_HOME/conf, and the default name is license.dat .

Copyt the lincense file to `$MACHBASE_HOME/conf`

At this time, the name of the license file issued must be changed to **license.dat** and copied. Then, when the server is started, it will determine if the license is appropriate and begin the installation.

Launch **machadmin -t 'licensefile_path'**

The advantage of this method is that it can be easily installed with commands without having to adjust the license file name or location. 

Installing as a query: This is a way to install a license using a query statement while the server is running.

## Verifying License Installation

### License Installed

If the license file is installed, the following is displayed in machbase.trc after the server is started.

```bash
[2016-02-17 14:51:00 P-20913 T-140709874054912][INFO] LICENSE [License Type (Version 2)][OFFICIAL]
[2016-02-17 14:51:00 P-20913 T-140709874054912][INFO] LICENSE [License Policy] [CORE]
[2016-02-17 14:51:00 P-20913 T-140709874054912][INFO] LICENSE [Host ID] [FFFFFFFFFFFFFFF]
[2016-02-17 14:51:00 P-20913 T-140709874054912][INFO] LICENSE [Expiry DATE] [25300318]
[2016-02-17 14:51:00 P-20913 T-140709874054912][INFO] Machbase Logs Signature! : OFFICIAL:CORE:FFFFFFFFFFFFFFF:25300318-3.5.0.826b8f2.official-LINUX-X86-64-release
```

You can also use the machadmin -f command.

### License Not Installed

If the license file is not installed or if an abnormal file is used, the following output is displayed.

```bash
[2016-02-17 14:49:54 P-6620 T-140539052701440][INFO] LICENSE [License Type(Version 2)][Only for evaluation (No license)]
[2016-02-17 14:49:54 P-6620 T-140539052701440][INFO] LICENSE [License Policy] [None]
[2016-02-17 14:49:54 P-6620 T-140539052701440][INFO] LICENSE [Host ID] [Unknown]
[2016-02-17 14:49:54 P-6620 T-140539052701440][INFO] LICENSE [Expiry DATE] [N/A]
```
