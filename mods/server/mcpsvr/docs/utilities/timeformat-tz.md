# Machbase Neo Time Format and Timezone Options Guide

This guide covers all time-related configuration options for Machbase Neo API queries, including time formats and timezone settings.

## Time Format Options

### Standard Time Formats

| timeformat | Result Example | Use Case |
|:-----------|:---------------|:---------|
| `DEFAULT` | 2006-01-02 15:04:05.999 | Human-readable default |
| `DEFAULT_MS` | 2006-01-02 15:04:05.999 | Millisecond precision |
| `DEFAULT_US` | 2006-01-02 15:04:05.999999 | Microsecond precision |
| `DEFAULT_NS` | 2006-01-02 15:04:05.999999999 | Nanosecond precision |
| `DEFAULT.MS` | 2006-01-02 15:04:05.000 | Fixed millisecond format |
| `DEFAULT.US` | 2006-01-02 15:04:05.000000 | Fixed microsecond format |
| `DEFAULT.NS` | 2006-01-02 15:04:05.000000000 | Fixed nanosecond format |

### RFC Standard Formats

| timeformat | Result Example | Use Case |
|:-----------|:---------------|:---------|
| `RFC3339` | 2006-01-02T15:04:05Z07:00 | ISO 8601 standard |
| `RFC3339Nano` | 2006-01-02T15:04:05.999999999Z07:00 | ISO 8601 with nanoseconds |
| `RFC822` | 02 Jan 06 15:04 MST | Email headers |
| `RFC822Z` | 02 Jan 06 15:04 -0700 | Email with timezone offset |
| `RFC1123` | Mon, 02 Jan 2006 15:04:05 MST | HTTP headers |
| `RFC1123Z` | Mon, 02 Jan 2006 15:04:05 -0700 | HTTP with timezone offset |

### Unix-style Formats

| timeformat | Result Example | Use Case |
|:-----------|:---------------|:---------|
| `Ansic` | Mon Jan _2 15:04:05 2006 | ANSI C format |
| `Unix` | Mon Jan _2 15:04:05 MST 2006 | Unix timestamp format |
| `Ruby` | Mon Jan 02 15:04:05 -0700 2006 | Ruby language format |
| `Kitchen` | 3:04:05PM | Time only, 12-hour |
| `Stamp` | Jan _2 15:04:05 | Short timestamp |
| `StampMilli` | Jan _2 15:04:05.000 | Short with milliseconds |
| `StampMicro` | Jan _2 15:04:05.000000 | Short with microseconds |
| `StampNano` | Jan _2 15:04:05.000000000 | Short with nanoseconds |

### Seconds-only Formats

| timeformat | Result Example | Use Case |
|:-----------|:---------------|:---------|
| `S_NS` | 05.999999999 | Seconds with nanosecond precision |
| `S_US` | 05.999999 | Seconds with microsecond precision |
| `S_MS` | 05.999 | Seconds with millisecond precision |
| `S.NS` | 05.000000000 | Fixed nanosecond format (seconds only) |
| `S.US` | 05.000000 | Fixed microsecond format (seconds only) |
| `S.MS` | 05.000 | Fixed millisecond format (seconds only) |

### Unix Epoch Formats

> **Since**: Machbase Neo v8.0.40

| timeformat | Result Example | Description |
|:-----------|:---------------|:------------|
| `ns` | 1676432361999999999 | Unix epoch nanoseconds (number) |
| `ns.str` | "1676432361999999999" | Unix epoch nanoseconds (string) |
| `us` | 1676432361999999 | Unix epoch microseconds (number) |
| `us.str` | "1676432361999999" | Unix epoch microseconds (string) |
| `ms` | 1676432361999 | Unix epoch milliseconds (number) |
| `ms.str` | "1676432361999" | Unix epoch milliseconds (string) |
| `s` | 1676432361 | Unix epoch seconds (number) |
| `s.str` | "1676432361" | Unix epoch seconds (string) |

### Custom Time Formats

You can create custom time formats using specific placeholder numbers:

```
Format Components:
- Year:    2006
- Month:   01
- Day:     02  
- Hour:    03 (12-hour) or 15 (24-hour)
- Minute:  04
- Second:  05 or 05.999999999 (with sub-seconds)
```

**Example Custom Format**: `2006-01-02 15:04:05.999999999`

## Timezone Options

### Supported Timezone Types

The `tz` option accepts timezone identifiers from the tz database (2024b version):

1. **IANA Timezone Identifiers**
   - `Asia/Seoul`
   - `America/New_York`
   - `Europe/London`
   - `Australia/Sydney`

2. **Common Abbreviations**
   - `UTC` - Coordinated Universal Time
   - `Local` - System local timezone
   - `EST` - Eastern Standard Time
   - `CET` - Central European Time
   - `GMT` - Greenwich Mean Time

3. **Reference**: [Complete List of Time Zones](https://en.wikipedia.org/wiki/List_of_tz_database_time_zones)

> **Note**: If a timezone is not recognized, Machbase Neo relies on the operating system's tz database. Contact your system administrator if needed.

## Usage Examples

### Basic Time Format Selection

```bash
# Default format
curl -o - http://127.0.0.1:5654/db/query \
    --data-urlencode "q=select * from EXAMPLE" \
    --data-urlencode "format=box" \
    --data-urlencode "timeformat=DEFAULT"
```

**Response**:
```
+----------+-------------------------+----------+
| NAME     | TIME                    | VALUE    |
+----------+-------------------------+----------+
| wave.sin | 2023-02-15 03:39:21     | 0.111111 |
| wave.sin | 2023-02-15 03:39:22.111 | 0.222222 |
+----------+-------------------------+----------+
```

### RFC3339 Format

```bash
curl -o - http://127.0.0.1:5654/db/query \
    --data-urlencode "q=select * from EXAMPLE" \
    --data-urlencode "format=box" \
    --data-urlencode "timeformat=RFC3339"
```

**Response**:
```
+----------+----------------------+----------+
| NAME     | TIME                 | VALUE    |
+----------+----------------------+----------+
| wave.sin | 2023-02-15T03:39:21Z | 0.111111 |
| wave.sin | 2023-02-15T03:39:22Z | 0.222222 |
+----------+----------------------+----------+
```

### Timezone Configuration

```bash
# Asian timezone
curl -o - http://127.0.0.1:5654/db/query \
    --data-urlencode "q=select * from EXAMPLE" \
    --data-urlencode "format=box" \
    --data-urlencode "timeformat=DEFAULT" \
    --data-urlencode "tz=Asia/Seoul"
```

**Response**:
```
+----------+-------------------------+----------+
| NAME     | TIME                    | VALUE    |
+----------+-------------------------+----------+
| wave.sin | 2023-02-15 12:39:21     | 0.111111 |
| wave.sin | 2023-02-15 12:39:22.111 | 0.222222 |
+----------+-------------------------+----------+
```

### High Precision with Timezone

```bash
# RFC3339 with nanosecond precision in New York timezone
curl -o - http://127.0.0.1:5654/db/query \
    --data-urlencode "q=select * from EXAMPLE" \
    --data-urlencode "format=box" \
    --data-urlencode "timeformat=RFC3339Nano" \
    --data-urlencode "tz=America/New_York"
```

**Response**:
```
+----------+-------------------------------------+----------+
| NAME     | TIME                                | VALUE    |
+----------+-------------------------------------+----------+
| wave.sin | 2023-02-14T22:39:21.111111111-05:00| 0.111111 |
| wave.sin | 2023-02-14T22:39:22.222222222-05:00| 0.222222 |
+----------+-------------------------------------+----------+
```

### Custom Format Examples

```bash
# Custom format with reordered components
curl -o - http://127.0.0.1:5654/db/query \
    --data-urlencode "q=select * from EXAMPLE" \
    --data-urlencode "format=box" \
    --data-urlencode "timeformat=03:04:05.999999999-ReOrder-2006-01-02"
```

**Response**:
```
+----------+----------------------------------------+----------+
| NAME     | TIME                                   | VALUE    |
+----------+----------------------------------------+----------+
| wave.sin | 03:39:21.111111111-ReOrder-2023-02-15  | 0.111111 |
| wave.sin | 03:39:22.222222222-ReOrder-2023-02-15  | 0.222222 |
+----------+----------------------------------------+----------+
```

## Configuration Summary

| Scenario | timeformat | tz | Result |
|----------|------------|----|---------| 
| Default display | `DEFAULT` | `Local` | 2023-02-15 03:39:21 |
| ISO standard | `RFC3339` | `UTC` | 2023-02-15T03:39:21Z |
| Asian localization | `DEFAULT` | `Asia/Seoul` | 2023-02-15 12:39:21 |
| High precision | `RFC3339Nano` | `UTC` | 2023-02-15T03:39:21.111111111Z |
| Unix timestamp | `ms` | `UTC` | 1676432361999 |
| Custom format | `2006/01/02 15:04` | `Local` | 2023/02/15 03:39 |

## Quick Reference

| Option | Common Values | Description |
|--------|---------------|-------------|
| `timeformat` | `DEFAULT`, `RFC3339`, `ns`, `ms` | Controls time display format |
| `tz` | `UTC`, `Local`, `Asia/Seoul`, `America/New_York` | Sets timezone for time display |

> **Note**: All timeformat values are case-insensitive.

This guide provides comprehensive coverage of all time-related options available in Machbase Neo REST API queries.

