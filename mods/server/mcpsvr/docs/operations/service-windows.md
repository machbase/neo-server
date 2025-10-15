# Machbase Neo Windows service

`machbase-neo service` command controls the registration of Windows services. Once the service installation has been done, machbase-neo can start automatically along with Windows boot.

> **Note**  
> These operations requires **Administrator** privilege.

## machbase-neo service install

Register machbase-neo to Windows services.

```cmd
machbase-neo.exe service install --host 0.0.0.0 --data D:\database --file D:\database\files --log-filename D:\database\machbase-neo.log
```

## machbase-neo service remove

Remove machbase-neo from Windows services.

```cmd
machbase-neo.exe service remove
```

## start and stop

Start and stop the service process. It is equivalent action that the service control panel of Windows provides.

```cmd
machbase-neo.exe service start
```

```cmd
machbase-neo.exe service stop
```