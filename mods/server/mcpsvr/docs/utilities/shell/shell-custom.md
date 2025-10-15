# Machbase Neo Custom Shell Guide

User can customize command line shell and open it in the web ui.

Open a *SHELL* on the web ui or run `machbase-neo shell` on the terminal, and use `shell` command to add/remove custom shell.

In this example, we are going to show how to add a user-defined shell that invokes `/bin/bash` (or `/bin/zsh`) for *nix users and `cmd.exe` for Windows users. You may add any programming language's REPL, other database's command line interface and ssh command that connects to your servers for example.

## Add a Custom Shell

### Register a Custom Shell

1. Select the menu icon from the left most side.

2. And Click `+` icon from the top left pane.

3. Set a preferred "Display name" and provide the absolute path and flags for the "Command" field. For example, to set 'zsh' as the command line on macOS, use the absolute path of your program and click "Save".

**Configuration Options:**
- **Name**: display name. (Any valid text is possible except some reserved words that machbase-neo reserves for the future use)
- **Command**: any executable command in full path with arguments
- **Theme**: terminal color theme

**Custom Command Examples:**
- Windows Cmd.exe: `C:\Windows\System32\cmd.exe`
- Linux bash: `/bin/bash`
- PostgreSQL Client on macOS: `/opt/homebrew/bin/psql postgres`

### Use the Custom Shell

- Open the custom shell on the main editor area.

- Open the custom shell on the console area.

## Command Line Management

The custom shells are manageable with machbase-neo shell command line interface.

### Add New Custom Shell

Use `shell add <name> <command and args>`. You can give a any name and any executable command with arguments, but the default shell name `SHELL` is reserved.

```sh
machbase-neo» shell add bashterm /bin/bash;
added
```

```sh
machbase-neo» shell add terminal /bin/zsh -il;
added
```

```sh
machbase-neo» shell add console C:\Windows\System32\cmd.exe;
added
```

### Show Registered Shell List

```sh
machbase-neo» shell list;
┌────────┬────────────────────────────┬────────────┬──────────────┐
│ ROWNUM │ ID                         │ NAME       │ COMMAND      │
├────────┼────────────────────────────┼────────────┼──────────────┤
│      1 │ 11F4AFFD-2A9B-4FC5-BB20-637│ BASHTERM   │ /bin/bash    │
│      2 │ 11F4AFFD-2A9B-4FC5-BB20-638│ TERMINAL   │ /bin/zsh -il │
└────────┴────────────────────────────┴────────────┴──────────────┘
```

### Delete a Custom Shell

```sh
machbase-neo» shell del 11F4AFFD-2A9B-4FC5-BB20-637;
deleted
```

## Quick Reference

| Method | Command | Description |
|--------|---------|-------------|
| **Web UI Registration** | UI Menu → `+` icon | Register custom shell via web interface |
| **Command Line Add** | `shell add <name> <command>` | Add custom shell via command line |
| **List Shells** | `shell list` | Show all registered custom shells |
| **Delete Shell** | `shell del <id>` | Remove custom shell by ID |
| **Reserved Names** | `SHELL` | Default shell name cannot be used |