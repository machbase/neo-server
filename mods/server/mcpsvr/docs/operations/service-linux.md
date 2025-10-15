# Machbase Neo Linux service

Using *systemd* or *supervisord*, you can run and manage machbase-neo process as system service, so that make it to start automatically when the system boot.

## Create start/stop script

**Create neo-start.sh**

```sh
$ vi neo-start.sh
```

```sh
#!/bin/bash 
exec /data/machbase-neo serve --host 0.0.0.0 --log-filename /data/log/machbase-neo.log
```

```sh
$ chmod 755 neo-start.sh
```

**Create neo-stop.sh**

```sh
$ vi neo-stop.sh
```

```sh
#!/bin/bash 
/data/machbase-neo shell shutdown
```

```sh
$ chmod 755 neo-stop.sh
```

## systemd

### Step 1: Create neo.service

```sh
$ cd /etc/systemd/system
$ sudo vi neo.service
```

```ini
[Unit]   
Description=neo service   
StartLimitBurst=10   
StartLimitIntervalSec=10   
  
[Service]   
User=machbase   
LimitNOFILE=65535   
ExecStart=/data/neo-start.sh
ExecStop=/data/neo-stop.sh
ExecStartPre=sleep 2   
WorkingDirectory=/data   
Restart=always   
RestartSec=1   
  
[Install]   
WantedBy=multi-user.target   
```

* Modify the `User` and paths according to your environment.

### Step 2: Activate the service

```sh
$ sudo chmod 755 neo.service
$ sudo systemctl daemon-reload
```

Make the service to auto-start when host machine re-boot.

```sh
$ sudo systemctl enable neo.service
```

### Step 3: Done

After activating the service, you can control it with the following commands:

```sh
$ sudo systemctl start neo.service
$ sudo systemctl status neo.service
$ sudo systemctl stop neo.service
```

## supervisord

### Step 1: Create neo.conf

```sh
$ cd /etc/supervisor/conf.d
$ sudo vi neo.conf
```

```ini
[program:neo]
command=/data/neo-start.sh
priority=10   
autostart=true   
autorestart=true   
environment=HOME=/home/machbase   
stdout_logfile=/data/log/machbase-neo_stdout.log   
stderr_logfile=/data/log/machbase-neo_stderr.log   
user=machbase   
```

* Modify the `user` and paths according to your environment.
* In the above example, the log folder `/data/log` should exist in advance.

### Step 2: Update Supervisord

```sh
$ sudo supervisorctl reread
$ sudo supervisorctl update
```

### Step 3: Done

After activating the service, you can control machbase-neo with the following commands:

```sh
$ sudo supervisorctl start neo
$ sudo supervisorctl status neo
$ sudo supervisorctl stop neo
```

## PM2

### Step 1: Create neo-start.sh

```sh
$ vi neo-start.sh
```

```sh
#!/bin/bash
exec /data/machbase-neo serve --host 0.0.0.0
```

* Logs will be managed by PM2, so the `--log-filename` option is not necessary.

### Step 2: Executable neo-start.sh

```sh
$ chmod 755 neo-start.sh
```

### Step 3: Run machbase-neo using PM2

```sh
$ pm2 start /data/neo-start.sh --name neo --log /data/log/machbase-neo.log
```

Check the status of machbase-neo.

```sh
$ pm2 status neo
```

### Step 4: Make PM2 to auto-start

* You can skip this process if you have already executed it.

To automatically generate and configuration a startup script just type the command (without sudo) `pm2 startup`:

```sh
$ pm2 startup
[PM2] Init System found: systemd
[PM2] To setup the Startup Script, copy/paste the following command:
sudo env PATH=$PATH:/usr/local/bin /usr/local/lib/node_modules/pm2/bin/pm2 startup systemd -u machbase --hp /home/machbase
```

Then copy/paste the displayed command onto the terminal:

```sh
$ sudo env PATH=$PATH:/usr/local/bin /usr/local/lib/node_modules/pm2/bin/pm2 startup systemd -u machbase --hp /home/machbase
```

Now PM2 will automatically restart at boot.

### Step 5: Saving the app list

Once you have started all desired apps, save the app list so it will respawn after reboot:

```sh
$ pm2 save
```

### Step 6: Done

You can control machbase-neo with the following commands:

```sh
$ pm2 start neo
$ pm2 status neo
$ pm2 stop neo
$ pm2 restart neo

$ pm2 logs neo
$ pm2 monit
```