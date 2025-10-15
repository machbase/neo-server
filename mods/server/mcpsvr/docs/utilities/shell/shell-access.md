# Machbase Neo Shell Access Guide

## Remote Access via Web

Click the Shell tab to run the interactive shell on the web.

## Remote Access via SSH

SSH (Secure Shell) is a protocol used to securely log onto remote systems. It can use a password for authentication, but it also supports a more secure method called public key authentication.

machbase-neo provides an SSH interface for remote operation and administration. Users can access the SQL interpreter by using the SSH command as shown below.

**Connection Details:**
- User: SYS
- Default password: manager
- Default port: 5652

```sh
$ ssh -p 5652 sys@127.0.0.1
sys@127.0.0.1's password: manager↵
```

Then after `machbase-neo» ` prompt, users can query with SQL statements.

```
machbase-neo» select * from example;
┌─────────┬──────────┬─────────────────────────┬───────────┐
│ ROWNUM  │ NAME     │ TIME(UTC)               │ VALUE     │
├─────────┼──────────┼─────────────────────────┼───────────┤
│       1 │ wave.sin │ 2023-01-31 03:58:02.751 │ 0.913716  │
│       2 │ wave.cos │ 2023-01-31 03:58:02.751 │ 0.406354  │
                        ...omit...
│      13 │ wave.sin │ 2023-01-31 03:58:05.751 │ 0.668819  │
│      14 │ wave.cos │ 2023-01-31 03:58:05.751 │ -0.743425 │
└─────────┴──────────┴─────────────────────────┴───────────┘
```

## SSH without Password

### Key Pair Setup Process

1. **Generate a key pair** - The first step is to generate a new key pair on the local machine (the machine you will log in from). This is done using the `ssh-keygen` command.

   > You can skip this step, if you have already a key pair.

   ```bash
   ssh-keygen -t rsa
   ```

   This command will create two files in the .ssh directory in your home directory: `id_rsa` (private key) and `id_rsa.pub` (public key).

2. **Copy the public key to the remote machine** - The next step is to copy the public key to the remote machine.

   To register the public key into the machbase server, follow the steps below.

3. **Log in with the key pair** - Now you can log in to the machbase server using your key pair. The SSH client will automatically use your private key to decrypt a challenge sent by the server, proving your identity.

   ```bash
   ssh -p 5652 sys@127.0.0.1
   ```

   If everything is set up correctly, you should be logged in to the machbase-neo without being asked for a password.

### Register SSH Key from Web UI

1. Select "SSH Keys" menu from the left bottom menu

   > Since Machbase Neo v8.0.20

2. To add a new SSH key, click on the "New SSH Key" button. Paste your public key in the designated field and provide a title. Finally, click on the "Add SSH Key" button to complete the process.

3. Your SSH key has been registered shows on the list.

### Register SSH Key from Shell Command

Adding the public key to the machbase-neo server enables the execution of any `machbase-neo shell` command without the need for a prompt or password entry.

1. **Add your public key to server**

   ```sh
   machbase-neo shell ssh-key add `cat ~/.ssh/id_rsa.pub`
   ```

2. **Get list of registered public keys**

   ```sh
   machbase-neo shell ssh-key list
   ```

   or

   ```
   $ machbase-neo shell ↵
   
   machbase-neo» ssh-key list
   ┌────────┬────────────────────────────┬─────────────────────┬──────────────────────────────────┐
   │ ROWNUM │ NAME                       │ KEY TYPE            │ FINGERPRINT                      │
   ├────────┼────────────────────────────┼─────────────────────┼──────────────────────────────────┤
   │      1 │ myid@laptop.local          │ ssh-rsa             │ 80bdaba07591276d065ca915a6037fde │
   │      2 │ myid@desktop.local         │ ecdsa-sha2-nistp256 │ e300ee460b890ad4c22cd4c1eae03477 │
   └────────┴────────────────────────────┴─────────────────────┴──────────────────────────────────┘
   ```

3. **Remove registered public key**

   ```sh
   machbase-neo» ssh-key del <fingerprint>
   ```

### Connect without Password

```sh
$ ssh -p 5652 sys@127.0.0.1 ↵

Greetings, SYS
machbase-neo v8.0.20-snapshot (8f10fa95 2024-06-19T16:32:09) standard
sys machbase-neo»
```

## Execute Commands via SSH

We can execute any machbase-neo shell command remotely only with `ssh`.

```sh
$ ssh -p 5652 sys@127.0.0.1 'select * from example order by time desc limit 5'↵

 ROWNUM  NAME      TIME(UTC)            VALUE     
──────────────────────────────────────────────────
 1       wave.sin  2023-02-09 11:46:46  0.406479  
 2       wave.cos  2023-02-09 11:46:46  0.913660  
 3       wave.sin  2023-02-09 11:46:45  -0.000281 
 4       wave.cos  2023-02-09 11:46:45  1.000000  
 5       wave.cos  2023-02-09 11:46:44  0.913431  
```

## Security Considerations

While public key authentication is more secure than password authentication, it is important to keep your private key safe. Anyone who gains access to your private key can log in to any system that has your public key.

## Quick Reference

| Method | Description | Requirements |
|--------|-------------|--------------|
| **Web Access** | Interactive shell via web browser | Browser access to web UI |
| **SSH Password** | Standard SSH with password | Default: sys/manager, port 5652 |
| **SSH Key** | Public key authentication | Key pair generation and registration |
| **Remote Commands** | Execute commands via SSH | SSH access with credentials |
| **Key Management** | Add/list/remove SSH keys | Web UI or shell commands |