# Table Types

Detailed reference documentation for all four Machbase table types. Each section provides complete syntax, features, and advanced usage patterns.

## Table Types

- [Tag Tables](./tag-tables/) - Sensor/device time-series data
- [Log Tables](./log-tables/) - Event streams and logs
- [Volatile Tables](./volatile-tables/) - In-memory real-time data
- [Lookup Tables](./lookup-tables/) - Reference and master data

## Quick Reference

| Type | Create Syntax | Best For |
|------|--------------|----------|
| Tag | `CREATE TAGDATA TABLE` | Sensor data (ID, time, value) |
| Log | `CREATE TABLE` | Events, logs, flexible schema |
| Volatile | `CREATE VOLATILE TABLE` | Real-time cache, sessions |
| Lookup | `CREATE LOOKUP TABLE` | Device registry, config |

For a comprehensive comparison and decision guide, see [Table Types Overview](../core-concepts/table-types-overview/).
