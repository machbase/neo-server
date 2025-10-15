# Preparing Linux Environment for Installation

## Check and Set Maximum Number of Files
1. Check the maximum number of Linux files with the following command.
   
```bash
[machbase@localhost ~] ulimit -Sn
1024
```

2. If the result is less than 65535, modify the files below and reboot the server.
   
```bash
[machbase@localhost ~] sudo vi /etc/security/limits.conf
 
 
#<domain>      <type>  <item>         <value>
#
 
*               hard    nofile          65535
*               soft    nofile          65535
 
 
[machbase@localhost ~] sudo vi /etc/systemd/user.conf
 
DefaultLimitNOFILE=65535
```

3. Reboot the server and check the value again.
   
```bash
[machbase@localhost ~] ulimit -Sn
65535
```

## Check and Set Server Time

Because Machbase is a database that deals with time series data, you need to set the time value correctly on the server where Machbase will be installed.

### Setting Time Zone

Since Machbase uses all the data in the local time where the server is located, you need to make sure that the timezone matches the time of the current server.
Make sure it matches the timezone where you are located with the following command: If different, select the correct region from /usr/share/zoneinfo and link.

```bash

[machbase@localhost ~] ls -l /etc/localtime
lrwxrwxrwx 1 root root 32 Sep 27 14:08 /etc/localtime -> ../usr/share/zoneinfo/Asia/Seoul
 
 
# You can check the timezone set through the date command.
[machbase@localhost ~] date
Wed Jan  2 11:12:44 KST 2019
```

### Setting Time

If the current local time is not correct, reset the time using the following command.

```bash
[machbase@localhost ~] sudo date -s '2018/12/25 12:34:56'
```

## Setting Port

The operating system port to be used by Machbase must be reserved so that it cannot be used by other programs.

If you set a reserved port with the command below, the operating system does not allocate the port to other programs, avoiding port conflicts.

```bash
[machbase@localhost ~] sudo echo reserved port range~reserved port range > /proc/sys/net/ipv4/ip_local_reserved_ports
```

The above method is a temporary method. To set it permanently, you need to edit the /etc/sysctl.conf file.

```bash
[machbase@localhost ~] sudo vim /etc/sysctl.conf

# add text below
net.ipv4.ip_local_reserved_ports = reserved port range-reserved port range
```
