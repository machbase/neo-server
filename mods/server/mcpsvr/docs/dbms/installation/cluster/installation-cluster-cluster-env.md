# Preparing for Cluster Edition Installation

## Confirm and Change File LIMIT

To increase the maximum number of files that can be opened, do the following.

1. Modify the file /etc/security/limits.conf

```bash
sudo vi /etc/security/limits.conf
*       hard   nofile      65535
*       soft   nofile      65535
```

2. Reboot.

```bash
sudo reboot
# or
sudo shutdown -r now
```

3. Check the results. If the output is 65535, it has been successfully changed.

```bash
unlimit -Sn
```

## Server Time Synchronization

You must synchronize the server time between each host. If they are already synchronized, check for confirmation.

1. Synchronize the time on all servers with the time server. 

```bash
# Synchronize with the following command.                      
/usr/bin/rdate -s time.bora.net && /sbin/clock -w
```

2. If the time server can not be used, modify it with a direct command.

```bash
# Modify with the following command.                                 
date -s "2017-10-31 11:15:30"
```

3. Check the modified time

```bash
# Check with the following command.                                 
date
```

## Change Network Kernel Parameters

1. Check the current set value.

```bash
Check with the following command.                                 
sysctl -a | egrep 'mem_(max|default)|tcp_.*mem'
```

2. Change the setting value with the following command (for 64GB Memory).

```bash
sysctl -w net.core.rmem_default = "33554432"     # 32MB
sysctl -w net.core.wmem_default = "33554432"
sysctl -w net.core.rmem_max     = "268435456"    # 256MB
sysctl -w net.core.wmem_max     = "268435456"  
sysctl -w net.ipv4.tcp_rmem     = "262144 33554432 268435456"
sysctl -w net.ipv4.tcp_wmem     = "262144 33554432 268435456"
 
# 8388608 Page * 4KB = 32GB
sysctl -w net.ipv4.tcp_mem      = "8388608 8388608 8388608"
```

3. To keep the changes, add them to the /etc/sysctl.conf file and restart the host OS.

```bash
# Modify the file /etc/sysctl.conf.
net.core.rmem_default = "33554432"
net.core.wmem_default = "33554432"
net.core.rmem_max     = "268435456"
net.core.wmem_max     = "268435456"
net.ipv4.tcp_rmem     = "262144 33554432 268435456"
net.ipv4.tcp_wmem     = "262144 33554432 268435456"
net.ipv4.tcp_mem      = "8388608 8388608 8388608"
```

## Create User

1. Create a Linux OS user 'machbase' for Machbase installation. The user account directory is created as: /home/machbase.

```bash
$ sudo useradd machbase --home-dir "/home/machbase"
```

2. Set password (machbase)

```bash
sudo passwd machbase
```
