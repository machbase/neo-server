# Machbase Neo Shell Run Guide

`machbase-neo shell run <file>` executes multiple commands in the given file.

## Make a Script File

Make an example script file like below.

- `cat batch.sh`

```sql
#
# comments starts with `#` or `--`
# A statement should be ends with semi-colon `;`
#

-- Count 1
SELECT count(*) FROM EXAMPLE WHERE name = 'wave.cos';

-- Count 2
SELECT count(*) FROM EXAMPLE 
  WHERE name = 'wave.sin'
;
```

## Run the Script File

```sh
machbase-neo shell run batch.sh
```

**Result:**

```
SELECT count(*) FROM EXAMPLE WHERE name = 'wave.cos'
 ROWNUM  COUNT(*)
──────────────────
      1  2175
a row fetched.

SELECT count(*) FROM EXAMPLE WHERE name = 'wave.sin'
 ROWNUM  COUNT(*)
──────────────────
      1  8175
a row fetched.
```

## Run the Script in Interactive Mode

```sh
$ machbase-neo shell

machbase-neo» run ./b.sh;
SELECT count(*) FROM EXAMPLE WHERE name = 'wave.cos'
╭────────┬──────────╮
│ ROWNUM │ COUNT(*) │
├────────┼──────────┤
│      1 │ 2175     │
╰────────┴──────────╯
a row fetched.

SELECT count(*) FROM EXAMPLE WHERE name = 'wave.sin'
╭────────┬──────────╮
│ ROWNUM │ COUNT(*) │
├────────┼──────────┤
│      1 │ 8175     │
╰────────┴──────────╯
a row fetched.
```

## Make an Executable Script

Add shebang(`#!`) as the first line of script file like below.

```sql
#!/usr/bin/env /path/to/machbase-neo shell run

-- Count 1
SELECT count(*) FROM EXAMPLE WHERE name = 'wave.cos';

-- Count 2
SELECT count(*) FROM EXAMPLE WHERE name = 'wave.sin';
```

Then `chmod` allowing executable permission.

```sh
$ chmod +x batch.sh
```

Execute the script.

```sh
$ ./batch.sh

SELECT count(*) FROM EXAMPLE WHERE name = 'wave.cos'
 ROWNUM  COUNT(*)
──────────────────
      1  2175
a row fetched.

SELECT count(*) FROM EXAMPLE WHERE name = 'wave.sin'
 ROWNUM  COUNT(*)
──────────────────
      1  8175
a row fetched.
```

## Quick Reference

| Method | Command | Description |
|--------|---------|-------------|
| **Direct Execution** | `machbase-neo shell run <file>` | Execute SQL script file directly |
| **Interactive Mode** | `run ./script.sh` | Execute script within shell session |
| **Executable Script** | `./script.sh` | Execute script with shebang header |
| **Comments** | `#` or `--` | Add comments to script files |
| **Statement Ending** | `;` | All statements must end with semicolon |