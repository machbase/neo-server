# Backup and Recovery

Learn how to protect your Machbase data with proper backup strategies and recovery procedures.

## Backup Overview

### Backup Types

| Type | Method | Downtime | Recovery Point |
|------|--------|----------|----------------|
| **Full Backup** | machadmin -b | Offline | Complete |
| **Online Backup** | Database mount | None | Point-in-time |
| **Export Backup** | CSV export | None | Table-level |

## Full Database Backup

### Offline Backup (Recommended)

```bash
# 1. Stop server
machadmin -s

# 2. Backup database
machadmin -b /backup/machbase_backup_20251010

# 3. Start server
machadmin -u
```

**Advantages**:
- Complete consistency
- Fastest backup
- Smallest backup size

**Disadvantages**:
- Requires downtime
- Full server shutdown

### Backup Script

```bash
#!/bin/bash
# daily_backup.sh

BACKUP_DIR="/backup/machbase"
DATE=$(date +%Y%m%d)
BACKUP_PATH="$BACKUP_DIR/backup_$DATE"

# Create backup directory
mkdir -p $BACKUP_DIR

# Stop server
machadmin -s

# Backup
machadmin -b $BACKUP_PATH

# Start server
machadmin -u

# Verify backup
if [ -d "$BACKUP_PATH" ]; then
    echo "Backup successful: $BACKUP_PATH"
    # Compress backup
    tar -czf ${BACKUP_PATH}.tar.gz -C $BACKUP_DIR backup_$DATE
    rm -rf $BACKUP_PATH
else
    echo "Backup failed!" | mail -s "Backup Error" admin@company.com
fi

# Remove old backups (keep last 7 days)
find $BACKUP_DIR -name "backup_*.tar.gz" -mtime +7 -delete
```

### Automated Backup (cron)

```bash
# Edit crontab
crontab -e

# Add daily backup at 2 AM
0 2 * * * /opt/scripts/daily_backup.sh >> /var/log/machbase_backup.log 2>&1
```

## Database Restore

### Full Restore

```bash
# 1. Stop server (if running)
machadmin -s

# 2. Remove existing database
rm -rf $MACHBASE_HOME/dbs/*

# 3. Restore from backup
machadmin -r /backup/machbase_backup_20251010

# 4. Start server
machadmin -u

# 5. Verify
machsql -f - <<EOF
SHOW TABLES;
SELECT COUNT(*) FROM sensors;
EOF
```

### Restore from Compressed Backup

```bash
# Extract backup
tar -xzf /backup/backup_20251010.tar.gz -C /tmp

# Restore
machadmin -s
rm -rf $MACHBASE_HOME/dbs/*
machadmin -r /tmp/backup_20251010
machadmin -u

# Cleanup
rm -rf /tmp/backup_20251010
```

## Online Backup (Database Mount)

### Create Online Backup

```bash
# 1. Mount database (creates consistent snapshot)
machadmin -mount /backup/mount_point

# 2. Copy database files
cp -r $MACHBASE_HOME/dbs /backup/online_backup_20251010

# 3. Unmount
machadmin -unmount /backup/mount_point
```

**Advantages**:
- No downtime
- Server keeps running

**Disadvantages**:
- Requires disk space
- More complex

## Table-Level Backup (CSV Export)

### Export Table to CSV

```bash
# Export single table
machsql -s localhost -u SYS -p MANAGER -f - -o sensors_backup.csv -r csv <<EOF
SELECT * FROM sensors;
EOF

# Export with time range
machsql -s localhost -u SYS -p MANAGER -f - -o sensors_recent.csv -r csv <<EOF
SELECT * FROM sensors DURATION 7 DAY;
EOF
```

### Export Script

```bash
#!/bin/bash
# export_tables.sh

BACKUP_DIR="/backup/csv"
DATE=$(date +%Y%m%d)

mkdir -p $BACKUP_DIR

# Export tables
for table in sensors logs devices; do
    echo "Exporting $table..."
    machsql -i -f - -o "$BACKUP_DIR/${table}_${DATE}.csv" -r csv <<EOF
SELECT * FROM $table;
EOF
done

# Compress exports
tar -czf $BACKUP_DIR/csv_backup_${DATE}.tar.gz -C $BACKUP_DIR *_${DATE}.csv
rm $BACKUP_DIR/*_${DATE}.csv

echo "CSV backup complete: $BACKUP_DIR/csv_backup_${DATE}.tar.gz"
```

### Restore from CSV

```bash
# Import CSV data
machloader -t sensors -d csv -i sensors_backup.csv
```

## Incremental Backup Strategy

### Daily Incremental + Weekly Full

```bash
#!/bin/bash
# incremental_backup.sh

BACKUP_DIR="/backup/machbase"
DAY_OF_WEEK=$(date +%u)  # 1=Monday, 7=Sunday

if [ $DAY_OF_WEEK -eq 7 ]; then
    # Sunday: Full backup
    echo "Performing full backup..."
    machadmin -s
    machadmin -b $BACKUP_DIR/full_$(date +%Y%m%d)
    machadmin -u
else
    # Weekday: CSV export
    echo "Performing incremental backup..."
    machsql -i -f - -o $BACKUP_DIR/incremental_$(date +%Y%m%d).csv -r csv <<EOF
    SELECT * FROM sensors WHERE _arrival_time >= SYSDATE - 1;
    EOF
fi
```

## Backup Verification

### Verify Backup Integrity

```bash
# Check backup directory exists
ls -lh /backup/machbase_backup_20251010

# Check backup size (should not be too small)
du -sh /backup/machbase_backup_20251010

# Test restore to temporary location
TEST_DIR="/tmp/test_restore"
mkdir -p $TEST_DIR
machadmin -r /backup/machbase_backup_20251010 -d $TEST_DIR
ls -lh $TEST_DIR
rm -rf $TEST_DIR
```

### Verify CSV Export

```bash
# Check CSV file
head -10 sensors_backup.csv
wc -l sensors_backup.csv

# Verify column count
awk -F',' '{print NF}' sensors_backup.csv | sort -u
```

## Disaster Recovery

### Recovery Plan

**Scenario**: Complete server failure

1. **Provision new server**
2. **Install Machbase**
3. **Restore from backup**
4. **Verify data integrity**
5. **Resume operations**

### Step-by-Step Recovery

```bash
# 1. Install Machbase
wget http://machbase.com/dist/machbase-xxx.tgz
tar xzf machbase-xxx.tgz
export MACHBASE_HOME=$(pwd)/machbase_home
export PATH=$MACHBASE_HOME/bin:$PATH

# 2. Restore from backup
machadmin -r /backup/machbase_backup_20251010

# 3. Start server
machadmin -u

# 4. Verify data
machsql -f - <<EOF
SHOW TABLES;
SELECT COUNT(*) FROM sensors;
SELECT MAX(_arrival_time) FROM sensors;
EOF

# 5. Test application connections
```

## Backup Best Practices

### 1. 3-2-1 Rule

- **3** copies of data (production + 2 backups)
- **2** different media types (disk + tape/cloud)
- **1** copy offsite (remote location/cloud)

### 2. Regular Testing

```bash
# Monthly restore test
# Restore to test environment and verify
```

### 3. Automated Backups

```bash
# Cron schedule
0 2 * * * /opt/scripts/daily_backup.sh  # Daily at 2 AM
0 3 * * 0 /opt/scripts/weekly_backup.sh  # Weekly on Sunday
```

### 4. Monitor Backup Success

```bash
# Check backup logs
tail -50 /var/log/machbase_backup.log

# Alert on failure
if ! grep -q "Backup successful" /var/log/machbase_backup.log; then
    echo "Backup failed!" | mail -s "Alert" admin@company.com
fi
```

### 5. Document Procedures

Create runbook with:
- Backup schedule
- Restore procedures
- Contact information
- Recovery time objectives (RTO)
- Recovery point objectives (RPO)

## Backup Storage

### Local Storage

```bash
# Dedicated backup disk
/dev/sdb1 → /backup

# Mount in /etc/fstab
/dev/sdb1  /backup  ext4  defaults  0  2
```

### Remote Storage (rsync)

```bash
# Sync to remote server
rsync -avz /backup/machbase/ backup-server:/backups/machbase/

# In backup script
tar -czf backup_${DATE}.tar.gz backup_${DATE}
rsync -avz backup_${DATE}.tar.gz backup-server:/backups/
```

### Cloud Storage (AWS S3)

```bash
# Upload to S3
aws s3 cp backup_${DATE}.tar.gz s3://my-backup-bucket/machbase/

# In backup script
tar -czf /tmp/backup_${DATE}.tar.gz -C /backup backup_${DATE}
aws s3 cp /tmp/backup_${DATE}.tar.gz s3://my-backup-bucket/machbase/
rm /tmp/backup_${DATE}.tar.gz
```

## Performance Impact

### Backup Performance

| Method | Duration (1TB) | CPU Impact | I/O Impact |
|--------|----------------|------------|------------|
| Full Offline | 30-60 min | None (offline) | High |
| Online Mount | 60-90 min | Low | Medium |
| CSV Export | 120+ min | Medium | High |

### Minimize Impact

```bash
# Use ionice to reduce I/O priority
ionice -c3 machadmin -b /backup/machbase

# Use nice to reduce CPU priority
nice -n 19 machadmin -b /backup/machbase
```

## Recovery Time Objectives

### Typical RTOs

| Scenario | RTO Target | Method |
|----------|------------|--------|
| Single table | < 1 hour | CSV restore |
| Full database | < 4 hours | Full restore |
| Disaster recovery | < 24 hours | Full restore + verification |

## Troubleshooting

**Backup fails with "database in use"**:
```bash
# Ensure server is stopped
machadmin -s
sleep 5
machadmin -e  # Should show "not running"
machadmin -b /backup/path
```

**Restore fails**:
```bash
# Check backup integrity
ls -lh /backup/machbase_backup

# Verify backup directory structure
tree /backup/machbase_backup

# Check disk space
df -h $MACHBASE_HOME
```

**Out of disk space during backup**:
```bash
# Check space before backup
df -h /backup

# Clean old backups
find /backup -name "backup_*" -mtime +7 -delete
```

## Next Steps

- **User Management**: [User Management](../user-management/) - Backup user permissions
- **Monitoring**: [Troubleshooting](../../troubleshooting/) - Monitor backup health
- **Tutorials**: [Getting Started](../../getting-started/) - Setup procedures

---

Implement a solid backup strategy and protect your Machbase data from loss!
