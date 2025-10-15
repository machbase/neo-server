# Connecting to Machbase

Learn all the ways to connect to Machbase, from the command-line client to programmatic APIs. This guide covers connection methods, configuration, and best practices.

## Connection Methods Overview

| Method | Best For | Language | Complexity |
|--------|----------|----------|------------|
| machsql | Interactive queries, testing | CLI | Easy |
| ODBC/CLI | C/C++ applications | C/C++ | Medium |
| JDBC | Java applications | Java | Easy |
| Python | Python applications | Python | Easy |
| REST API | HTTP/web applications | Any | Easy |
| .NET | C# applications | C# | Easy |

## machsql (Command-Line Client)

### Basic Connection

```bash
# Interactive connection
machsql

# You'll be prompted for:
# - Server address (default: 127.0.0.1)
# - User ID (default: SYS)
# - Password (default: MANAGER)
```

### Connection with Parameters

```bash
# Specify all parameters
machsql -s localhost -u SYS -p MANAGER

# Remote server
machsql -s 192.168.1.100 -u analyst -p mypassword

# Custom port
machsql -s localhost -P 7878 -u SYS -p MANAGER

# Specific database
machsql -s localhost -u SYS -p MANAGER -d MYDB
```

### Common Options

```bash
# Execute SQL script
machsql -f script.sql

# Output to file
machsql -o output.txt

# Silent mode (no banner)
machsql -i

# CSV output format
machsql -r csv -o results.csv

# Set timezone
machsql -z +0900  # Korea timezone
```

### Connection String

```bash
# Full connection string
machsql -s 192.168.1.100 -P 5656 -u analyst -p password123 -d MACHBASE -i -f query.sql -o results.csv -r csv
```

## ODBC/CLI Connection

### C/C++ Application

```c
#include "machbase_cli.h"

int main() {
    SQLHENV env;
    SQLHDBC conn;
    SQLHSTMT stmt;
    SQLRETURN rc;

    // Allocate environment
    SQLAllocEnv(&env);

    // Allocate connection
    SQLAllocConnect(env, &conn);

    // Connect
    rc = SQLConnect(conn,
                    (SQLCHAR*)"127.0.0.1", SQL_NTS,  // Server
                    (SQLCHAR*)"SYS", SQL_NTS,        // User
                    (SQLCHAR*)"MANAGER", SQL_NTS);   // Password

    if (rc == SQL_SUCCESS || rc == SQL_SUCCESS_WITH_INFO) {
        printf("Connected!\n");

        // Allocate statement
        SQLAllocStmt(conn, &stmt);

        // Execute query
        rc = SQLExecDirect(stmt,
                          (SQLCHAR*)"SELECT * FROM sensors DURATION 1 HOUR",
                          SQL_NTS);

        // Process results...

        // Cleanup
        SQLFreeStmt(stmt, SQL_DROP);
    }

    SQLDisconnect(conn);
    SQLFreeConnect(conn);
    SQLFreeEnv(env);

    return 0;
}
```

### Compile and Link

```bash
# Linux
gcc -o myapp myapp.c -I$MACHBASE_HOME/include -L$MACHBASE_HOME/lib -lmachcli

# Run
export LD_LIBRARY_PATH=$MACHBASE_HOME/lib:$LD_LIBRARY_PATH
./myapp
```

## JDBC Connection

### Java Application

```java
import java.sql.*;

public class MachbaseExample {
    public static void main(String[] args) {
        String url = "jdbc:machbase://127.0.0.1:5656/MACHBASE";
        String user = "SYS";
        String password = "MANAGER";

        try {
            // Load driver
            Class.forName("com.machbase.jdbc.driver");

            // Connect
            Connection conn = DriverManager.getConnection(url, user, password);
            System.out.println("Connected!");

            // Execute query
            Statement stmt = conn.createStatement();
            ResultSet rs = stmt.executeQuery(
                "SELECT * FROM sensors DURATION 1 HOUR"
            );

            // Process results
            while (rs.next()) {
                String sensorId = rs.getString("sensor_id");
                double value = rs.getDouble("value");
                System.out.println(sensorId + ": " + value);
            }

            // Cleanup
            rs.close();
            stmt.close();
            conn.close();

        } catch (Exception e) {
            e.printStackTrace();
        }
    }
}
```

### JDBC URL Format

```
jdbc:machbase://[host]:[port]/[database]

Examples:
jdbc:machbase://localhost:5656/MACHBASE
jdbc:machbase://192.168.1.100:5656/MYDB
```

### Connection Properties

```java
Properties props = new Properties();
props.setProperty("user", "SYS");
props.setProperty("password", "MANAGER");
props.setProperty("connectionTimeout", "30");

Connection conn = DriverManager.getConnection(url, props);
```

## Python Connection

### Using machbase-python

```python
import machbase

# Connect
conn = machbase.connect('127.0.0.1', 5656, 'SYS', 'MANAGER')

# Create cursor
cur = conn.cursor()

# Execute query
cur.execute("SELECT * FROM sensors DURATION 1 HOUR")

# Fetch results
rows = cur.fetchall()
for row in rows:
    print(row)

# Cleanup
cur.close()
conn.close()
```

### Insert Data

```python
# Single insert
cur.execute("INSERT INTO sensors VALUES (?, ?, ?)",
            ('sensor01', '2025-10-10 14:00:00', 25.3))

# Batch insert
data = [
    ('sensor01', '2025-10-10 14:00:01', 25.4),
    ('sensor01', '2025-10-10 14:00:02', 25.5),
    ('sensor02', '2025-10-10 14:00:01', 22.1)
]
cur.executemany("INSERT INTO sensors VALUES (?, ?, ?)", data)

conn.commit()
```

### Connection Pooling

```python
from machbase import ConnectionPool

# Create pool
pool = ConnectionPool(
    host='127.0.0.1',
    port=5656,
    user='SYS',
    password='MANAGER',
    min_connections=5,
    max_connections=20
)

# Get connection from pool
conn = pool.get_connection()

# Use connection...

# Return to pool
pool.release_connection(conn)
```

## REST API Connection

### HTTP Endpoints

```
Base URL: http://[host]:5654

Endpoints:
- POST /machbase - Execute SQL
- POST /machbase/query - Execute SELECT
- POST /machbase/insert - Execute INSERT
- GET /machbase/tables - List tables
```

### Execute Query (curl)

```bash
# Query data
curl -X POST http://localhost:5654/machbase \
  -H "Content-Type: application/json" \
  -d '{
    "sql": "SELECT * FROM sensors DURATION 1 HOUR",
    "format": "json"
  }'

# Insert data
curl -X POST http://localhost:5654/machbase \
  -H "Content-Type: application/json" \
  -d '{
    "sql": "INSERT INTO sensors VALUES (?, ?, ?)",
    "params": ["sensor01", "2025-10-10 14:00:00", 25.3]
  }'
```

### JavaScript Example

```javascript
// Query data
async function querySensors() {
    const response = await fetch('http://localhost:5654/machbase', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({
            sql: 'SELECT * FROM sensors DURATION 1 HOUR',
            format: 'json'
        })
    });

    const data = await response.json();
    console.log(data);
}

// Insert data
async function insertSensor(sensorId, value) {
    const response = await fetch('http://localhost:5654/machbase', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({
            sql: 'INSERT INTO sensors VALUES (?, ?, ?)',
            params: [sensorId, new Date().toISOString(), value]
        })
    });

    return response.json();
}
```

## .NET Connection

### C# Example

```csharp
using System;
using System.Data;
using Machbase.Data.MachbaseClient;

class Program {
    static void Main() {
        string connString = "Server=127.0.0.1;Port=5656;User Id=SYS;Password=MANAGER;Database=MACHBASE;";

        using (MachConnection conn = new MachConnection(connString)) {
            conn.Open();
            Console.WriteLine("Connected!");

            // Execute query
            using (MachCommand cmd = new MachCommand(
                "SELECT * FROM sensors DURATION 1 HOUR", conn)) {

                using (MachDataReader reader = cmd.ExecuteReader()) {
                    while (reader.Read()) {
                        string sensorId = reader.GetString(0);
                        double value = reader.GetDouble(1);
                        Console.WriteLine($"{sensorId}: {value}");
                    }
                }
            }
        }
    }
}
```

## Connection Best Practices

### 1. Use Connection Pooling

For applications, maintain a connection pool:

```java
// Java example
HikariConfig config = new HikariConfig();
config.setJdbcUrl("jdbc:machbase://localhost:5656/MACHBASE");
config.setUsername("SYS");
config.setPassword("MANAGER");
config.setMaximumPoolSize(20);
config.setMinimumIdle(5);

HikariDataSource pool = new HikariDataSource(config);
```

### 2. Handle Connection Errors

```python
import machbase
import time

def get_connection(retries=3):
    for i in range(retries):
        try:
            return machbase.connect('127.0.0.1', 5656, 'SYS', 'MANAGER')
        except Exception as e:
            if i == retries - 1:
                raise
            time.sleep(1)
```

### 3. Close Connections Properly

```java
try (Connection conn = DriverManager.getConnection(url, user, password);
     Statement stmt = conn.createStatement();
     ResultSet rs = stmt.executeQuery(sql)) {

    // Use connection...

} // Auto-closed with try-with-resources
```

### 4. Set Timeouts

```java
// Connection timeout
props.setProperty("connectionTimeout", "30");  // seconds

// Query timeout
stmt.setQueryTimeout(60);  // seconds
```

### 5. Use Prepared Statements

```java
// Prevent SQL injection
String sql = "SELECT * FROM sensors WHERE sensor_id = ?";
PreparedStatement pstmt = conn.prepareStatement(sql);
pstmt.setString(1, userInput);
ResultSet rs = pstmt.executeQuery();
```

## Connection Troubleshooting

### Server Not Running

```bash
# Check server status
machadmin -e

# Expected output: "Machbase server is running"

# If not running, start it
machadmin -u
```

### Connection Refused

```bash
# Check if port is listening
netstat -an | grep 5656

# Check firewall
sudo iptables -L | grep 5656

# Allow port through firewall
sudo iptables -A INPUT -p tcp --dport 5656 -j ACCEPT
```

### Authentication Failed

```sql
-- Check user exists
SHOW USERS;

-- Reset password
ALTER USER username IDENTIFIED BY 'newpassword';
```

### Network Issues

```bash
# Test network connectivity
ping 192.168.1.100

# Test port connectivity
telnet 192.168.1.100 5656

# Check DNS resolution
nslookup machbase-server.company.com
```

## Security Considerations

### 1. Use Strong Passwords

```sql
-- Create user with strong password
CREATE USER analyst IDENTIFIED BY 'Str0ng!P@ssw0rd123';
```

### 2. Limit Network Access

```bash
# Bind to specific IP (in machbase.conf)
BIND_IP_ADDRESS = 192.168.1.100

# Only accept connections from specific IPs
```

### 3. Use SSL/TLS

```bash
# Enable SSL (in machbase.conf)
SSL_ENABLE = 1
SSL_CERT = /path/to/cert.pem
SSL_KEY = /path/to/key.pem
```

### 4. Principle of Least Privilege

```sql
-- Grant only necessary permissions
CREATE USER readonly IDENTIFIED BY 'password';
GRANT SELECT ON sensors TO readonly;
```

## Next Steps

- **Import Data**: [Importing Data](../importing-data/) - Load data into Machbase
- **Query Data**: [Querying Data](../querying/) - Query patterns and optimization
- **User Management**: [User Management](../user-management/) - Create and manage users

---

Choose the connection method that fits your application and start building with Machbase!
