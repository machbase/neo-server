# Machbase Neo Timer Guide

> **Important**: Machbase Neo commands end with a semicolon ( ; )

## Overview
Timer is a feature that defines tasks to be executed at specific times or repeated at set intervals.

## Adding a New Timer

You can register tasks that run according to a specified schedule. A web UI for timer management has been included since version 8.0.20.

### Adding via Web UI
1. Select the timer icon from the left menu bar
2. Click the `+` button in the top left
3. Set Timer ID (name), Timer Spec, and TQL script path
4. Click the "Create" button

### Timer Start/Stop/Delete
- Use toggle button to start/stop timers
- Edit, start, stop, and delete available from detail page

## Timer Schedule Specifications

There are three ways to define timer schedules:

### Examples
```
0 30 * * * *           Every hour at 30 minutes
@every 1h30m           Every 1 hour 30 minutes
@daily                 Daily
```

## CRON Expressions

| Field | Required | Allowed Values | Special Characters |
|-------|----------|----------------|-------------------|
| Seconds | Yes | 0-59 | * / , - |
| Minutes | Yes | 0-59 | * / , - |
| Hours | Yes | 0-23 | * / , - |
| Day | Yes | 1-31 | * / , - ? |
| Month | Yes | 1-12 or JAN-DEC | * / , - |
| Day of Week | Yes | 0-6 or SUN-SAT | * / , - ? |

### Special Characters Description

- **Asterisk `*`**: Matches all values in the field
- **Slash `/`**: Indicates increments of ranges (e.g., 3-59/15 means starting from 3 minutes with 15-minute intervals)
- **Comma `,`**: Separates list items (e.g., "MON,WED,FRI" means Monday, Wednesday, Friday)
- **Hyphen `-`**: Defines ranges (e.g., 9-17 means 9 AM to 5 PM)
- **Question mark `?`**: Used instead of `*` when leaving day or day of week field empty

## Predefined Schedules

| Expression | Description | Equivalent CRON |
|------------|-------------|-----------------|
| @yearly (or @annually) | Once a year, midnight on January 1st | 0 0 0 1 1 * |
| @monthly | Once a month, midnight on 1st of month | 0 0 0 1 * * |
| @weekly | Once a week, midnight between Sat/Sun | 0 0 0 * * 0 |
| @daily (or @midnight) | Once a day, midnight | 0 0 0 * * * |
| @hourly | Once an hour, beginning of hour | 0 0 * * * * |

## Interval Specification

Uses `@every <duration>` format, where duration is in formats like "300ms", "-1.5h", "2h45m".
Valid time units: "ms", "s", "m", "h"

### Examples
```
@every 10h
@every 1h10m30s
```

## Command Line Usage

### Add Timer
```bash
timer add [--autostart] <name> <timer_spec> <tql-path>;
```
- `--autostart`: Auto-start when machbase-neo starts
- `<name>`: Task name
- `<timer_spec>`: Execution schedule
- `<tql-path>`: TQL script to execute as task

### List Timers
```bash
timer list;
```

### Start/Stop Timer
```bash
timer [start | stop] <name>;
```

### Delete Timer
```bash
timer del <name>;
```

## Hello World Example

### 1. Create TQL Script
Create `helloworld.tql` file and save the following code:

```js
CSV(`helloworld,0,0`)
MAPVALUE(1, time('now'))
MAPVALUE(2, random())
INSERT("name", "time", "value", table("example"))
```

### 2. Test Script
Execute the script to verify a single record is inserted into EXAMPLE table:

```sql
select * from example where name = 'helloworld';
```

Expected result:
```
┌────────┬────────────┬─────────────────────────┬────────────────────┐
│ ROWNUM │ NAME       │ TIME(LOCAL)             │ VALUE              │
├────────┼────────────┼─────────────────────────┼────────────────────┤
│      1 │ helloworld │ 2024-06-19 18:20:07.001 │ 0.6132387755535856 │
└────────┴────────────┴─────────────────────────┴────────────────────┘
```

### 3. Register Timer
Register timer from command line:

```bash
timer add helloworld "@every 5s" helloworld.tql;
```

Check "Auto Start" option or manually start with toggle button.

### 4. Verify Results
Confirm new records are inserted every 5 seconds:

```sql
select * from example where name = 'helloworld';
```

Example result:
```
┌────────┬────────────┬─────────────────────────┬─────────────────────┐
│ ROWNUM │ NAME       │ TIME(LOCAL)             │ VALUE               │
├────────┼────────────┼─────────────────────────┼─────────────────────┤
│      1 │ helloworld │ 2024-07-03 09:49:47.002 │ 0.14047743934840562 │
│      2 │ helloworld │ 2024-07-03 09:49:42.002 │ 0.7656153597963373  │
│      3 │ helloworld │ 2024-07-03 09:49:37.002 │ 0.11713331640146182 │
│      4 │ helloworld │ 2024-07-03 09:49:32.002 │ 0.5351642943247759  │
│      5 │ helloworld │ 2024-07-03 09:49:27.001 │ 0.6588127185612987  │
└────────┴────────────┴─────────────────────────┴─────────────────────┘
```

### 5. Create Dashboard
You can create an auto-refreshing dashboard to monitor timer operation.

## Timer Management

### Check Timer Status from Command Line
```bash
timer list;
```

Example result:
```
┌────────────┬───────────┬────────────────┬───────────┬─────────┐
│ NAME       │ SPEC      │ TQL            │ AUTOSTART │ STATE   │
├────────────┼───────────┼────────────────┼───────────┼─────────┤
│ HELLOWORLD │ @every 5s │ helloworld.tql │ false     │ RUNNING │
└────────────┴───────────┴────────────────┴───────────┴─────────┘
```

### Timer Control
```bash
# Start timer
timer start helloworld;

# Stop timer
timer stop helloworld;
```

---

## Quick Reference

| Operation | Command | Example |
|-----------|---------|---------|
| Add Timer | `timer add <name> <spec> <tql-path>;` | `timer add daily_task "@daily" task.tql;` |
| List Timers | `timer list;` | Shows all timers with status |
| Start Timer | `timer start <name>;` | `timer start daily_task;` |
| Stop Timer | `timer stop <name>;` | `timer stop daily_task;` |
| Delete Timer | `timer del <name>;` | `timer del daily_task;` |