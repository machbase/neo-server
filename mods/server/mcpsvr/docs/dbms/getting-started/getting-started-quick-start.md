# Quick Start

Get Machbase up and running in 5 minutes! This guide will walk you through installation, creating your first table, and running your first queries.

## Prerequisites

- Linux or Windows operating system
- 100MB free disk space
- Terminal access

## Step 1: Install Machbase

### Linux

Download and extract Machbase:

```bash
# Download package (replace x.x.x with actual version)
wget http://machbase.com/dist/machbase-fog-x.x.x.official-LINUX-X86-64-release.tgz

# Create directory and extract
mkdir machbase_home
tar zxf machbase-fog-x.x.x.official-LINUX-X86-64-release.tgz -C machbase_home

# Set environment variables
export MACHBASE_HOME=$(pwd)/machbase_home
export PATH=$MACHBASE_HOME/bin:$PATH
export LD_LIBRARY_PATH=$MACHBASE_HOME/lib:$LD_LIBRARY_PATH
```

### Windows

1. Download the Windows installer (.msi file)
2. Run the installer and follow the wizard
3. The installer will automatically set up environment variables

## Step 2: Create and Start Database

```bash
# Create database
machadmin -c

# Start server
machadmin -u
```

You should see:
```
Database created successfully.
Machbase server started successfully.
```

## Step 3: Connect to Machbase

Launch the interactive SQL client:

```bash
machsql
```

When prompted:
- **Server address**: Press Enter (uses default 127.0.0.1)
- **User ID**: Press Enter (uses default SYS)
- **Password**: Type `MANAGER` and press Enter

You'll see the `Mach>` prompt, ready for commands!

## Step 4: Create Your First Table

Let's create a table to store sensor temperature data:

```sql
CREATE TABLE sensor_data (
    sensor_id VARCHAR(20),
    temperature DOUBLE,
    humidity DOUBLE
);
```

## Step 5: Insert Data

Add some sample sensor readings:

```sql
INSERT INTO sensor_data VALUES ('sensor01', 25.3, 65.2);
INSERT INTO sensor_data VALUES ('sensor01', 25.5, 64.8);
INSERT INTO sensor_data VALUES ('sensor02', 22.1, 70.5);
```

## Step 6: Query Data

Retrieve your data:

```sql
-- Get all records
SELECT * FROM sensor_data;

-- Get records with timestamps
SELECT _arrival_time, * FROM sensor_data;

-- Get average temperature
SELECT AVG(temperature) FROM sensor_data;

-- Get data from last 10 minutes
SELECT * FROM sensor_data DURATION 10 MINUTE;
```

**Note**: The `_arrival_time` column is automatically added to every record with nanosecond precision!

## Understanding the Results

When you run `SELECT * FROM sensor_data`, you'll notice:

1. **Newest data first** - Machbase automatically orders results by most recent
2. **Automatic timestamps** - Every record has an `_arrival_time` column
3. **High precision** - Timestamps are accurate to nanoseconds

Example output:
```
SENSOR_ID            TEMPERATURE  HUMIDITY
------------------------------------------------
sensor02             22.1         70.5
sensor01             25.5         64.8
sensor01             25.3         65.2
[3] row(s) selected.
```

## What Just Happened?

Congratulations! You've just:

✓ Installed Machbase
✓ Created and started a database
✓ Connected using machsql
✓ Created a table
✓ Inserted time-series data
✓ Queried data with automatic timestamping

## Next Steps

Now that you have Machbase running:

1. [**First Steps**](../first-steps/) - Learn more machsql commands
2. [**Basic Concepts**](../concepts/) - Understand table types and when to use them
3. [**Tutorials**](../../tutorials/) - Follow hands-on tutorials for real-world scenarios

## Common Commands

Keep these handy:

```bash
# Start Machbase
machadmin -u

# Stop Machbase
machadmin -s

# Check if running
machadmin -e

# Connect to database
machsql
```

## Troubleshooting

**Server won't start?**
- Check if port 5656 is available: `netstat -an | grep 5656`
- Check logs in `$MACHBASE_HOME/trc/` directory

**Can't connect?**
- Verify server is running: `machadmin -e`
- Check default credentials: username `SYS`, password `MANAGER`

**Need help?**
- See [Troubleshooting Guide](../../troubleshooting/)
- Check [Error Codes](../../troubleshooting/error-codes/)

---

**Ready to dive deeper?** Continue to [First Steps](../first-steps/) to master the machsql command-line interface!
