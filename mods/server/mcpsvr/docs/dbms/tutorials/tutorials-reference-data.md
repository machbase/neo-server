# Tutorial 4: Reference Data

Learn how to manage reference and master data using Machbase Lookup tables. This tutorial shows you how to maintain device registries, configuration tables, and dimension data.

## Scenario

You're managing an IoT platform with:
- 1,000+ sensors deployed across multiple facilities
- Device metadata (location, type, owner, installation date)
- Configuration settings
- Need to JOIN with time-series data for enriched queries

**Goal**: Store device registry and configuration data that changes rarely but is frequently read.

## What You'll Learn

- Creating Lookup tables for reference data
- Storing master/dimension data
- JOIN operations with time-series tables
- Managing device metadata
- Maintaining configuration tables

## Prerequisites

- Machbase installed and running
- machsql client connected
- Basic SQL knowledge
- 10 minutes of time

## Step 1: Create Lookup Tables

Lookup tables are disk-based tables optimized for reference data:

```sql
-- Device registry
CREATE LOOKUP TABLE devices (
    device_id VARCHAR(50),
    device_name VARCHAR(100),
    device_type VARCHAR(50),
    location VARCHAR(200),
    facility VARCHAR(100),
    installed_date DATETIME,
    owner VARCHAR(100),
    status VARCHAR(20)
);

-- Device types catalog
CREATE LOOKUP TABLE device_types (
    type_code VARCHAR(50),
    type_name VARCHAR(100),
    manufacturer VARCHAR(100),
    model VARCHAR(100),
    specifications VARCHAR(500)
);

-- Facility information
CREATE LOOKUP TABLE facilities (
    facility_code VARCHAR(50),
    facility_name VARCHAR(100),
    address VARCHAR(200),
    city VARCHAR(100),
    country VARCHAR(50),
    manager VARCHAR(100)
);
```

## Step 2: Insert Reference Data

```sql
-- Add device types
INSERT INTO device_types VALUES (
    'TEMP-001', 'Temperature Sensor', 'Acme Corp', 'TempMaster 3000',
    'Range: -40 to 125C, Accuracy: ±0.5C'
);
INSERT INTO device_types VALUES (
    'HUM-001', 'Humidity Sensor', 'Acme Corp', 'HumidityPro 200',
    'Range: 0-100%, Accuracy: ±2%'
);
INSERT INTO device_types VALUES (
    'PRESS-001', 'Pressure Sensor', 'SensorTech', 'PressSense 500',
    'Range: 0-1000 PSI, Accuracy: ±1%'
);

-- Add facilities
INSERT INTO facilities VALUES (
    'FAC-NY-01', 'New York Warehouse', '123 Industrial Ave', 'New York', 'USA', 'John Smith'
);
INSERT INTO facilities VALUES (
    'FAC-LA-01', 'Los Angeles Distribution Center', '456 Commerce Blvd', 'Los Angeles', 'USA', 'Jane Doe'
);
INSERT INTO facilities VALUES (
    'FAC-CHI-01', 'Chicago Manufacturing Plant', '789 Factory St', 'Chicago', 'USA', 'Bob Johnson'
);

-- Add devices
INSERT INTO devices VALUES (
    'sensor-ny-temp-001', 'NY Warehouse Zone A Temp', 'TEMP-001',
    'Zone A, Row 5, Shelf 3', 'FAC-NY-01',
    TO_DATE('2024-01-15', 'YYYY-MM-DD'), 'Facilities Team', 'ACTIVE'
);
INSERT INTO devices VALUES (
    'sensor-ny-temp-002', 'NY Warehouse Zone B Temp', 'TEMP-001',
    'Zone B, Row 2, Shelf 1', 'FAC-NY-01',
    TO_DATE('2024-01-15', 'YYYY-MM-DD'), 'Facilities Team', 'ACTIVE'
);
INSERT INTO devices VALUES (
    'sensor-la-hum-001', 'LA DC Humidity Monitor', 'HUM-001',
    'Main Floor, Section 3', 'FAC-LA-01',
    TO_DATE('2024-02-20', 'YYYY-MM-DD'), 'Operations', 'ACTIVE'
);
INSERT INTO devices VALUES (
    'sensor-chi-press-001', 'Chicago Line 1 Pressure', 'PRESS-001',
    'Production Line 1, Station 5', 'FAC-CHI-01',
    TO_DATE('2023-11-10', 'YYYY-MM-DD'), 'Manufacturing', 'MAINTENANCE'
);
```

## Step 3: Query Reference Data

```sql
-- Get all active devices
SELECT device_id, device_name, location, facility
FROM devices
WHERE status = 'ACTIVE';

-- Devices by facility
SELECT device_id, device_name, device_type, location
FROM devices
WHERE facility = 'FAC-NY-01'
ORDER BY location;

-- Devices needing maintenance
SELECT device_id, device_name, facility, installed_date
FROM devices
WHERE status = 'MAINTENANCE';

-- Devices older than 1 year
SELECT device_id, device_name, installed_date, facility
FROM devices
WHERE installed_date < NOW - INTERVAL '365' DAY
ORDER BY installed_date;
```

## Step 4: Update Reference Data

Lookup tables support UPDATE and DELETE:

```sql
-- Update device status
UPDATE devices
SET status = 'ACTIVE'
WHERE device_id = 'sensor-chi-press-001';

-- Change device location
UPDATE devices
SET location = 'Zone C, Row 1, Shelf 2'
WHERE device_id = 'sensor-ny-temp-001';

-- Update facility manager
UPDATE facilities
SET manager = 'Sarah Williams'
WHERE facility_code = 'FAC-NY-01';

-- Decommission device
UPDATE devices
SET status = 'DECOMMISSIONED'
WHERE device_id = 'sensor-la-hum-001';
```

## Step 5: JOIN with Time-Series Data

Combine reference data with sensor readings:

```sql
-- First, create sensor data table (from Tutorial 1)
CREATE TAGDATA TABLE sensor_readings (
    sensor_id VARCHAR(50) PRIMARY KEY,
    time DATETIME BASETIME,
    value DOUBLE SUMMARIZED
);

-- Insert sample readings
INSERT INTO sensor_readings VALUES ('sensor-ny-temp-001', NOW, 22.5);
INSERT INTO sensor_readings VALUES ('sensor-ny-temp-002', NOW, 23.1);
INSERT INTO sensor_readings VALUES ('sensor-la-hum-001', NOW, 55.2);

-- JOIN sensor data with device info
SELECT
    sr.sensor_id,
    d.device_name,
    d.location,
    d.facility,
    sr.value,
    sr.time
FROM sensor_readings sr
JOIN devices d ON sr.sensor_id = d.device_id
DURATION 1 HOUR;

-- Get readings with full context
SELECT
    sr.sensor_id,
    d.device_name,
    d.location,
    f.facility_name,
    f.city,
    dt.type_name,
    dt.manufacturer,
    sr.value
FROM sensor_readings sr
JOIN devices d ON sr.sensor_id = d.device_id
JOIN facilities f ON d.facility = f.facility_code
JOIN device_types dt ON d.device_type = dt.type_code
DURATION 1 HOUR;
```

## Step 6: Create Configuration Table

Store application configuration:

```sql
CREATE LOOKUP TABLE system_config (
    config_key VARCHAR(100),
    config_value VARCHAR(500),
    config_type VARCHAR(50),
    description VARCHAR(500),
    updated_at DATETIME,
    updated_by VARCHAR(100)
);

-- Add configuration
INSERT INTO system_config VALUES (
    'alert.temperature.max', '80.0', 'THRESHOLD',
    'Maximum temperature threshold in Celsius',
    NOW, 'admin'
);
INSERT INTO system_config VALUES (
    'alert.humidity.max', '75.0', 'THRESHOLD',
    'Maximum humidity threshold percentage',
    NOW, 'admin'
);
INSERT INTO system_config VALUES (
    'retention.sensor_data.days', '90', 'RETENTION',
    'Days to retain sensor data',
    NOW, 'admin'
);
INSERT INTO system_config VALUES (
    'alert.email', 'ops@company.com', 'CONTACT',
    'Alert notification email address',
    NOW, 'admin'
);

-- Query configuration
SELECT config_key, config_value
FROM system_config
WHERE config_type = 'THRESHOLD';

-- Update configuration
UPDATE system_config
SET config_value = '85.0',
    updated_at = NOW,
    updated_by = 'supervisor'
WHERE config_key = 'alert.temperature.max';
```

## Step 7: Device Lifecycle Management

Track device lifecycle:

```sql
CREATE LOOKUP TABLE device_lifecycle (
    event_id INTEGER,
    device_id VARCHAR(50),
    event_type VARCHAR(50),
    event_date DATETIME,
    performed_by VARCHAR(100),
    notes VARCHAR(500)
);

-- Record lifecycle events
INSERT INTO device_lifecycle VALUES (
    1, 'sensor-ny-temp-001', 'INSTALLED',
    TO_DATE('2024-01-15', 'YYYY-MM-DD'), 'Install Team', 'Initial installation'
);
INSERT INTO device_lifecycle VALUES (
    2, 'sensor-chi-press-001', 'MAINTENANCE',
    TO_DATE('2025-09-15', 'YYYY-MM-DD'), 'Maintenance Team', 'Scheduled maintenance'
);
INSERT INTO device_lifecycle VALUES (
    3, 'sensor-chi-press-001', 'CALIBRATED',
    TO_DATE('2025-09-15', 'YYYY-MM-DD'), 'Maintenance Team', 'Recalibrated after maintenance'
);

-- Device history
SELECT
    d.device_id,
    d.device_name,
    dl.event_type,
    dl.event_date,
    dl.performed_by,
    dl.notes
FROM devices d
JOIN device_lifecycle dl ON d.device_id = dl.device_id
WHERE d.device_id = 'sensor-chi-press-001'
ORDER BY dl.event_date DESC;
```

## Try It Yourself

### Exercise 1: Add New Facility

Add a new facility and devices:

<details>
<summary>Solution</summary>

```sql
-- Add facility
INSERT INTO facilities VALUES (
    'FAC-SEA-01', 'Seattle Tech Center', '999 Innovation Way',
    'Seattle', 'USA', 'Mike Chen'
);

-- Add devices for new facility
INSERT INTO devices VALUES (
    'sensor-sea-temp-001', 'Seattle Server Room Temp', 'TEMP-001',
    'Server Room A, Rack 1', 'FAC-SEA-01',
    NOW, 'IT Team', 'ACTIVE'
);
INSERT INTO devices VALUES (
    'sensor-sea-hum-001', 'Seattle Server Room Humidity', 'HUM-001',
    'Server Room A, Rack 1', 'FAC-SEA-01',
    NOW, 'IT Team', 'ACTIVE'
);
```
</details>

### Exercise 2: Find Devices Due for Maintenance

Create a query to find devices installed over 6 months ago:

<details>
<summary>Solution</summary>

```sql
SELECT
    d.device_id,
    d.device_name,
    d.installed_date,
    FLOOR((NOW - d.installed_date) / 86400) as days_installed,
    f.facility_name
FROM devices d
JOIN facilities f ON d.facility = f.facility_code
WHERE d.installed_date < NOW - INTERVAL '180' DAY
  AND d.status = 'ACTIVE'
ORDER BY d.installed_date ASC;
```
</details>

### Exercise 3: Device Inventory Report

Create a summary report by facility and device type:

<details>
<summary>Solution</summary>

```sql
SELECT
    f.facility_name,
    dt.type_name,
    COUNT(*) as device_count,
    SUM(CASE WHEN d.status = 'ACTIVE' THEN 1 ELSE 0 END) as active_count,
    SUM(CASE WHEN d.status = 'MAINTENANCE' THEN 1 ELSE 0 END) as maintenance_count
FROM devices d
JOIN facilities f ON d.facility = f.facility_code
JOIN device_types dt ON d.device_type = dt.type_code
GROUP BY f.facility_name, dt.type_name
ORDER BY f.facility_name, dt.type_name;
```
</details>

## Real-World Patterns

### Pattern: Hierarchical Reference Data

```sql
-- Organizational hierarchy
CREATE LOOKUP TABLE organizations (
    org_id VARCHAR(50),
    org_name VARCHAR(100),
    parent_org_id VARCHAR(50),
    org_level INTEGER
);

-- Region → Facility → Zone hierarchy
INSERT INTO organizations VALUES ('ORG-USA', 'USA Operations', NULL, 1);
INSERT INTO organizations VALUES ('ORG-USA-EAST', 'East Region', 'ORG-USA', 2);
INSERT INTO organizations VALUES ('FAC-NY-01', 'NY Warehouse', 'ORG-USA-EAST', 3);

-- Query with hierarchy
SELECT
    o1.org_name as region,
    o2.org_name as facility,
    COUNT(d.device_id) as device_count
FROM organizations o1
JOIN organizations o2 ON o2.parent_org_id = o1.org_id
JOIN devices d ON d.facility = o2.org_id
WHERE o1.org_level = 2
GROUP BY o1.org_name, o2.org_name;
```

### Pattern: User Permissions

```sql
CREATE LOOKUP TABLE user_permissions (
    user_id VARCHAR(50),
    user_name VARCHAR(100),
    role VARCHAR(50),
    facility_access VARCHAR(50),
    permissions VARCHAR(200)
);

-- Grant access
INSERT INTO user_permissions VALUES (
    'user123', 'John Smith', 'FACILITY_MANAGER',
    'FAC-NY-01', 'READ,WRITE,CONFIGURE'
);

-- Check permissions
SELECT permissions
FROM user_permissions
WHERE user_id = 'user123'
  AND facility_access = 'FAC-NY-01';
```

### Pattern: Enriched Analytics

```sql
-- Get sensor readings with full context for analytics
SELECT
    f.city,
    dt.manufacturer,
    AVG(sr.value) as avg_reading,
    COUNT(*) as reading_count
FROM sensor_readings sr
JOIN devices d ON sr.sensor_id = d.device_id
JOIN facilities f ON d.facility = f.facility_code
JOIN device_types dt ON d.device_type = dt.type_code
WHERE d.status = 'ACTIVE'
DURATION 24 HOUR
GROUP BY f.city, dt.manufacturer;
```

## Performance Tips

1. **Index frequently queried columns**: Create indexes on common lookup keys
2. **Keep data current**: Regularly update reference data
3. **Normalize appropriately**: Separate reference tables for cleaner design
4. **Use for small-medium datasets**: Lookup tables work best with <1M rows

## Lookup vs Volatile Tables

| Feature | Lookup Table | Volatile Table |
|---------|-------------|----------------|
| **Storage** | Disk | Memory |
| **Persistence** | Yes | No |
| **Speed** | Slower | Faster |
| **Use Case** | Reference data | Real-time cache |
| **Data Volume** | Medium-Large | Small |
