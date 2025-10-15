# Installation Guide

This guide covers installation of Machbase Standard Edition on Linux and Windows. For cluster installations, see [Cluster Edition Installation](../../installation/cluster/).

## Choosing Your Installation Method

**Linux Users:**
- **Tarball** (Recommended) - Maximum flexibility, works on all distributions
- **Docker** - Quick setup, isolated environment

**Windows Users:**
- **MSI Installer** - Easiest option with GUI wizard

## Linux Installation

### Method 1: Tarball Installation (Recommended)

#### 1. Create User (Optional but Recommended)

```bash
sudo useradd machbase
sudo passwd machbase
su - machbase
```

#### 2. Download and Extract

```bash
# Download package
wget http://machbase.com/dist/machbase-fog-x.x.x.official-LINUX-X86-64-release.tgz

# Create directory
mkdir machbase_home
mv machbase-fog-x.x.x.official-LINUX-X86-64-release.tgz machbase_home/
cd machbase_home/

# Extract
tar zxf machbase-fog-x.x.x.official-LINUX-X86-64-release.tgz
```

#### 3. Set Environment Variables

Add to your `~/.bashrc`:

```bash
export MACHBASE_HOME=/home/machbase/machbase_home
export PATH=$MACHBASE_HOME/bin:$PATH
export LD_LIBRARY_PATH=$MACHBASE_HOME/lib:$LD_LIBRARY_PATH
```

Apply changes:

```bash
source ~/.bashrc
```

#### 4. Verify Installation

```bash
machadmin --help
```

You should see the Machbase Administration Tool help output.

### Method 2: Docker Installation

```bash
# Pull image
docker pull machbase/machbase

# Run container
docker run -d --name machbase \
  -p 5656:5656 \
  -v machbase_data:/data \
  machbase/machbase

# Connect to container
docker exec -it machbase machsql
```

For detailed Docker instructions, see [Docker Installation](../../installation/docker/).

## Windows Installation

### MSI Installer

#### 1. Download

Download the Windows installer (.msi file) from the Machbase website.

#### 2. Run Installer

- Double-click the .msi file
- Follow the installation wizard
- Choose installation directory (default: `C:\machbase`)
- The installer automatically sets PATH variables

#### 3. Verify Installation

Open Command Prompt and run:

```cmd
machadmin --help
```

For detailed Windows instructions, see [Windows Installation](../../installation/windows/).

## Post-Installation Steps

### 1. Create Database

```bash
machadmin -c
```

Expected output:
```
Database created successfully.
```

### 2. Start Server

```bash
machadmin -u
```

Expected output:
```
Machbase server started successfully.
```

### 3. Verify Server is Running

```bash
machadmin -e
```

Or check the process:

```bash
# Linux
ps -ef | grep machbased

# Windows
tasklist | findstr machbased
```

### 4. Connect to Database

```bash
machsql
```

Default credentials:
- **Username**: SYS
- **Password**: MANAGER

## Directory Structure

After installation, you'll find these directories:

```
machbase_home/
├── bin/           # Executable files (machadmin, machsql, etc.)
├── conf/          # Configuration files
├── dbs/           # Database files (created after machadmin -c)
├── lib/           # Shared libraries
├── trc/           # Log files
├── sample/        # Example files
└── doc/           # Documentation
```

## Configuration (Optional)

### Change Server Port

By default, Machbase uses port 5656. To change:

**Option 1: Environment Variable**

```bash
export MACHBASE_PORT_NO=7878
```

**Option 2: Configuration File**

Edit `$MACHBASE_HOME/conf/machbase.conf`:

```ini
PORT_NO = 7878
```

For all configuration options, see [Configuration Guide](../../configuration/).

## Essential Commands

```bash
# Create database
machadmin -c

# Start server
machadmin -u

# Stop server
machadmin -s

# Check status
machadmin -e

# Destroy database (careful!)
machadmin -d

# Connect via SQL
machsql
```

## License Installation

For production use, you'll need to install a license:

```bash
machadmin -t /path/to/license.dat
```

Verify license:

```bash
machadmin -f
```

Or in machsql:

```sql
SHOW LICENSE;
```

For trial licenses, visit the Machbase website. For detailed license management, see [License Management](../../installation/license/).

## System Requirements

### Minimum Requirements

- **CPU**: x86-64 compatible processor
- **RAM**: 1GB
- **Disk**: 100MB for software + data storage
- **OS**:
  - Linux: kernel 2.6 or later
  - Windows: Windows 7 or later

### Recommended for Production

- **CPU**: 4+ cores
- **RAM**: 8GB+
- **Disk**: SSD for better performance
- **OS**:
  - Linux: RHEL 7+, Ubuntu 16.04+, CentOS 7+
  - Windows: Windows Server 2012+

## Troubleshooting

### Installation Issues

**"Permission denied" errors (Linux)**

```bash
chmod +x $MACHBASE_HOME/bin/*
```

**"Library not found" errors (Linux)**

```bash
ldd $MACHBASE_HOME/bin/machbased
# Install any missing libraries
```

### Server Start Issues

**Port already in use**

```bash
# Check what's using port 5656
netstat -an | grep 5656

# Use different port
export MACHBASE_PORT_NO=7878
```

**Insufficient memory**

Check and adjust in `$MACHBASE_HOME/conf/machbase.conf`:

```ini
MEM_MAX_DB = 2G
```

See [Troubleshooting Guide](../../troubleshooting/) for more solutions.

## Next Steps

Now that Machbase is installed:

1. [**Quick Start**](../quick-start/) - Create your first database and table
2. [**First Steps**](../first-steps/) - Learn basic machsql commands
3. [**Basic Concepts**](../concepts/) - Understand Machbase architecture

## Advanced Installation

For advanced setups:

- [Cluster Edition Installation](../../installation/cluster/)
- [High Availability Setup](../../installation/cluster/)
- [Upgrade Procedures](../../installation/cluster/upgrade/)
