# User Management

Learn how to create users, grant permissions, and manage database security in Machbase.

## User Management Overview

### Default User

Machbase comes with a default administrator:
- **Username**: SYS
- **Password**: MANAGER
- **Permissions**: Full administrative rights

**Important**: Change the default password immediately!

```sql
ALTER USER SYS IDENTIFIED BY 'NewStr0ng!Password';
```

## Creating Users

### Basic User Creation

```sql
-- Create user
CREATE USER analyst IDENTIFIED BY 'password123';

-- Create with strong password
CREATE USER dataeng IDENTIFIED BY 'Str0ng!P@ss2025';
```

### User Naming Rules

- 1-128 characters
- Letters, numbers, underscore
- Case-insensitive
- Cannot start with number

## Granting Permissions

### Table Permissions

```sql
-- Grant SELECT
GRANT SELECT ON sensors TO analyst;

-- Grant INSERT
GRANT INSERT ON sensors TO dataeng;

-- Grant multiple permissions
GRANT SELECT, INSERT ON sensors TO dataeng;

-- Grant all permissions on table
GRANT ALL ON sensors TO admin_user;
```

### Database-Level Permissions

```sql
-- Grant SELECT on all tables
GRANT SELECT ON DATABASE TO readonly_user;

-- Grant all permissions
GRANT ALL ON DATABASE TO admin_user;
```

### System Permissions

```sql
-- Grant user management permission
GRANT CREATE USER TO admin_user;

-- Grant table creation permission
GRANT CREATE TABLE TO developer;
```

## Revoking Permissions

```sql
-- Revoke specific permission
REVOKE SELECT ON sensors FROM analyst;

-- Revoke multiple permissions
REVOKE INSERT, UPDATE ON sensors FROM dataeng;

-- Revoke all permissions
REVOKE ALL ON sensors FROM old_user;
```

## Managing Users

### View Users

```sql
-- List all users
SHOW USERS;

-- View user permissions
SELECT * FROM SYSTEM_.SYS_USERS_;
```

### Change Password

```sql
-- Change own password
ALTER USER analyst IDENTIFIED BY 'NewPassword2025';

-- SYS can change any user's password
ALTER USER dataeng IDENTIFIED BY 'ResetPassword';
```

### Delete User

```sql
-- Drop user
DROP USER analyst;

-- Drop user with cascade (remove all permissions)
DROP USER analyst CASCADE;
```

## Permission Levels

### Permission Matrix

| Permission | SELECT | INSERT | UPDATE | DELETE | CREATE TABLE | DROP TABLE |
|-----------|--------|--------|--------|--------|--------------|------------|
| **READ_ONLY** | ✓ | | | | | |
| **DATA_WRITER** | ✓ | ✓ | ✓ | ✓ | | |
| **TABLE_ADMIN** | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| **SYS (Admin)** | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |

## Common User Roles

### Read-Only User

```sql
CREATE USER readonly IDENTIFIED BY 'password';
GRANT SELECT ON DATABASE TO readonly;
```

### Data Analyst

```sql
CREATE USER analyst IDENTIFIED BY 'password';
GRANT SELECT ON sensors TO analyst;
GRANT SELECT ON logs TO analyst;
GRANT SELECT ON devices TO analyst;
```

### Application User

```sql
CREATE USER app_user IDENTIFIED BY 'password';
GRANT SELECT, INSERT ON sensors TO app_user;
GRANT SELECT, INSERT ON logs TO app_user;
```

### Administrator

```sql
CREATE USER admin IDENTIFIED BY 'password';
GRANT ALL ON DATABASE TO admin;
GRANT CREATE USER TO admin;
```

## Security Best Practices

### 1. Strong Passwords

```sql
-- Good: Strong password
CREATE USER secure_user IDENTIFIED BY 'Tr0ng!P@ssw0rd#2025';

-- Bad: Weak password
CREATE USER weak_user IDENTIFIED BY 'password';  -- Don't do this!
```

**Password Requirements**:
- Minimum 8 characters
- Mix of upper/lowercase
- Include numbers
- Include special characters

### 2. Principle of Least Privilege

```sql
-- Grant only necessary permissions
CREATE USER report_user IDENTIFIED BY 'password';
GRANT SELECT ON sensors TO report_user;  -- Only SELECT, not INSERT/UPDATE/DELETE
```

### 3. Regular Password Rotation

```bash
# Quarterly password change policy
*/
ALTER USER analyst IDENTIFIED BY 'NewPasswordQ42025';
```

### 4. Remove Inactive Users

```sql
-- Regularly audit and remove
DROP USER inactive_user;
```

### 5. Separate Application Users

```sql
-- Don't use SYS for applications
-- Create dedicated app users
CREATE USER sensor_app IDENTIFIED BY 'password';
GRANT SELECT, INSERT ON sensors TO sensor_app;
```

## Connection Examples

### Connect as Specific User

```bash
# machsql
machsql -s localhost -u analyst -p password

# JDBC
jdbc:machbase://localhost:5656/MACHBASE?user=analyst&password=password

# Python
conn = machbase.connect('127.0.0.1', 5656, 'analyst', 'password')
```

## Auditing

### Monitor User Activity

```sql
-- Check active sessions
SHOW STATEMENTS;

-- View connection history (check logs)
-- $MACHBASE_HOME/trc/machbase.log
```

### Log Analysis

```bash
# View recent logins
grep "LOGIN" $MACHBASE_HOME/trc/machbase.log | tail -20

# Check failed login attempts
grep "LOGIN FAILED" $MACHBASE_HOME/trc/machbase.log
```

## Troubleshooting

### Login Failed

```bash
# Check username exists
machsql -u SYS -p MANAGER -f - <<EOF
SHOW USERS;
EOF

# Reset password
machsql -u SYS -p MANAGER -f - <<EOF
ALTER USER analyst IDENTIFIED BY 'newpassword';
EOF
```

### Permission Denied

```sql
-- Check user permissions
-- (Connect as SYS)
SELECT * FROM SYSTEM_.SYS_USERS_ WHERE name = 'ANALYST';

-- Grant missing permission
GRANT SELECT ON tablename TO analyst;
```

### User Already Exists

```sql
-- Drop and recreate
DROP USER analyst;
CREATE USER analyst IDENTIFIED BY 'password';
```

## Complete Examples

### Example 1: Analytics Team

```sql
-- Create analysts
CREATE USER analyst1 IDENTIFIED BY 'Pass#2025!A1';
CREATE USER analyst2 IDENTIFIED BY 'Pass#2025!A2';

-- Grant read-only access
GRANT SELECT ON sensors TO analyst1;
GRANT SELECT ON sensors TO analyst2;
GRANT SELECT ON logs TO analyst1;
GRANT SELECT ON logs TO analyst2;
GRANT SELECT ON devices TO analyst1;
GRANT SELECT ON devices TO analyst2;
```

### Example 2: Application Users

```sql
-- Sensor data collector
CREATE USER sensor_app IDENTIFIED BY 'SensApp#2025!';
GRANT INSERT ON sensors TO sensor_app;

-- Log collector
CREATE USER log_app IDENTIFIED BY 'LogApp#2025!';
GRANT INSERT ON logs TO log_app;

-- Dashboard application
CREATE USER dashboard IDENTIFIED BY 'Dash#2025!';
GRANT SELECT ON sensors TO dashboard;
GRANT SELECT ON logs TO dashboard;
GRANT SELECT ON devices TO dashboard;
```

### Example 3: External Partner

```sql
-- Limited access for partner
CREATE USER partner IDENTIFIED BY 'Partner#2025!';

-- Only specific table and time range
GRANT SELECT ON public_sensors TO partner;

-- Restrict via application logic (not SQL)
-- Application enforces: DURATION 7 DAY only
```

## User Management Script

```bash
#!/bin/bash
# user_management.sh

# Create new user
create_user() {
    local username=$1
    local password=$2

    machsql -u SYS -p MANAGER -f - <<EOF
CREATE USER $username IDENTIFIED BY '$password';
EOF
}

# Grant permissions
grant_select() {
    local username=$1
    local table=$2

    machsql -u SYS -p MANAGER -f - <<EOF
GRANT SELECT ON $table TO $username;
EOF
}

# Usage
create_user "newanalyst" "SecurePass#2025"
grant_select "newanalyst" "sensors"
grant_select "newanalyst" "logs"
```

## Next Steps

- **Backup & Recovery**: [Backup & Recovery](../backup-recovery/) - Protect user data
- **Connecting**: [Connecting](../connecting/) - Connection methods for users
- **Security**: [Troubleshooting](../../troubleshooting/) - Security best practices

---

Proper user management ensures secure and controlled access to your Machbase data!
