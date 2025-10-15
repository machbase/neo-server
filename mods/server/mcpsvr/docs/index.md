# Machbase Neo Documentation Index

This comprehensive index contains all documentation available for Machbase Neo, organized by category to help LLM agents provide better assistance to users.

## API Documentation

### HTTP API
- **http-guide.md** (`docs/api/api-http/http-guide.md`) - Complete guide for HTTP API usage and configuration
- **http-query.md** (`docs/api/api-http/http-query.md`) - HTTP API query operations and examples
- **http-write.md** (`docs/api/api-http/http-write.md`) - HTTP API write operations for data insertion
- **http-create-drop-table.md** (`docs/api/api-http/http-create-drop-table.md`) - Table management via HTTP API
- **http-ui.md** (`docs/api/api-http/http-ui.md`) - HTTP API user interface documentation
- **http-watch-data.md** (`docs/api/api-http/http-watch-data.md`) - Real-time data monitoring via HTTP API
- **http-upload-files.md** (`docs/api/api-http/http-upload-files.md`) - File upload functionality through HTTP API
- **http-lineprotocol.md** (`docs/api/api-http/http-lineprotocol.md`) - Line protocol support for HTTP API
- **http-python.md** (`docs/api/api-http/http-python.md`) - Python client examples for HTTP API
- **http-javascript.md** (`docs/api/api-http/http-javascript.md`) - JavaScript client examples for HTTP API
- **http-csharp.md** (`docs/api/api-http/http-csharp.md`) - C# client examples for HTTP API
- **http-go.md** (`docs/api/api-http/http-go.md`) - Go client examples for HTTP API

### MQTT API
- **mqtt-guide.md** (`docs/api/api-mqtt/mqtt-guide.md`) - MQTT API complete guide and setup
- **mqtt-query.md** (`docs/api/api-mqtt/mqtt-query.md`) - MQTT query operations
- **mqtt-write.md** (`docs/api/api-mqtt/mqtt-write.md`) - MQTT write operations and data publishing
- **mqtt-writev5.md** (`docs/api/api-mqtt/mqtt-writev5.md`) - MQTT v5 specific write operations
- **mqtt-python.md** (`docs/api/api-mqtt/mqtt-python.md`) - Python MQTT client examples
- **mqtt-javascript-websocket.md** (`docs/api/api-mqtt/mqtt-javascript-websocket.md`) - JavaScript MQTT client with WebSocket
- **mqtt-csharp.md** (`docs/api/api-mqtt/mqtt-csharp.md`) - C# MQTT client examples
- **mqtt-go.md** (`docs/api/api-mqtt/mqtt-go.md`) - Go MQTT client examples

### gRPC API
- **grpc-guide.md** (`docs/api/api-grpc/grpc-guide.md`) - gRPC API complete guide and setup
- **grpc-query.md** (`docs/api/api-grpc/grpc-query.md`) - gRPC query operations
- **grpc-queryrow.md** (`docs/api/api-grpc/grpc-queryrow.md`) - gRPC single row query operations
- **grpc-exec.md** (`docs/api/api-grpc/grpc-exec.md`) - gRPC execution operations
- **grpc-python.md** (`docs/api/api-grpc/grpc-python.md`) - Python gRPC client examples
- **grpc-csharp.md** (`docs/api/api-grpc/grpc-csharp.md`) - C# gRPC client examples
- **grpc-java.md** (`docs/api/api-grpc/grpc-java.md`) - Java gRPC client examples

## Database Management System (DBMS)

### Getting Started
- **getting-started-installation.md** (`docs/dbms/getting-started/getting-started-installation.md`) - Initial installation guide
- **getting-started-quick-start.md** (`docs/dbms/getting-started/getting-started-quick-start.md`) - Quick start tutorial
- **getting-started-first-steps.md** (`docs/dbms/getting-started/getting-started-first-steps.md`) - First steps after installation
- **getting-started-concepts.md** (`docs/dbms/getting-started/getting-started-concepts.md`) - Core concepts and terminology

### Core Concepts
- **core-concepts-time-series-data.md** (`docs/dbms/core-concepts/core-concepts-time-series-data.md`) - Understanding time series data in Machbase Neo
- **core-concepts-indexing.md** (`docs/dbms/core-concepts/core-concepts-indexing.md`) - Indexing strategies and implementation
- **core-concepts-table-types-overview.md** (`docs/dbms/core-concepts/core-concepts-table-types-overview.md`) - Overview of different table types

### Table Types
- **table-types.md** (`docs/dbms/table-types/table-types.md`) - Complete guide to all table types

#### Tag Tables
- **table-types-tag-tables-creating-tag-tables.md** (`docs/dbms/table-types/tag-tables/table-types-tag-tables-creating-tag-tables.md`) - Creating and configuring tag tables
- **table-types-tag-tables-inserting-data.md** (`docs/dbms/table-types/tag-tables/table-types-tag-tables-inserting-data.md`) - Data insertion methods for tag tables
- **table-types-tag-tables-querying-data.md** (`docs/dbms/table-types/tag-tables/table-types-tag-tables-querying-data.md`) - Querying strategies for tag tables
- **table-types-tag-tables-deleting-data.md** (`docs/dbms/table-types/tag-tables/table-types-tag-tables-deleting-data.md`) - Data deletion in tag tables
- **table-types-tag-tables-tag-indexes.md** (`docs/dbms/table-types/tag-tables/table-types-tag-tables-tag-indexes.md`) - Index management for tag tables
- **table-types-tag-tables-tag-metadata.md** (`docs/dbms/table-types/tag-tables/table-types-tag-tables-tag-metadata.md`) - Metadata handling in tag tables
- **table-types-tag-tables-rollup-tables.md** (`docs/dbms/table-types/tag-tables/table-types-tag-tables-rollup-tables.md`) - Rollup table configuration and usage
- **table-types-tag-tables-varchar-storage.md** (`docs/dbms/table-types/tag-tables/table-types-tag-tables-varchar-storage.md`) - VARCHAR storage optimization
- **table-types-tag-tables-lsl-usl-limits.md** (`docs/dbms/table-types/tag-tables/table-types-tag-tables-lsl-usl-limits.md`) - LSL/USL limit configurations
- **table-types-tag-tables-duplication-removal.md** (`docs/dbms/table-types/tag-tables/table-types-tag-tables-duplication-removal.md`) - Duplicate data handling

#### Log Tables
- **table-types-log-tables-creating-log-tables.md** (`docs/dbms/table-types/log-tables/table-types-log-tables-creating-log-tables.md`) - Creating and configuring log tables
- **table-types-log-tables-insert.md** (`docs/dbms/table-types/log-tables/table-types-log-tables-insert.md`) - Basic insertion operations
- **table-types-log-tables-insert-insert-data.md** (`docs/dbms/table-types/log-tables/table-types-log-tables-insert-insert-data.md`) - Standard data insertion methods
- **table-types-log-tables-insert-append-data.md** (`docs/dbms/table-types/log-tables/table-types-log-tables-insert-append-data.md`) - Append operations for log tables
- **table-types-log-tables-insert-import-data.md** (`docs/dbms/table-types/log-tables/table-types-log-tables-insert-import-data.md`) - Data import procedures
- **table-types-log-tables-insert-load-data.md** (`docs/dbms/table-types/log-tables/table-types-log-tables-insert-load-data.md`) - Bulk data loading
- **table-types-log-tables-select.md** (`docs/dbms/table-types/log-tables/table-types-log-tables-select.md`) - Basic selection operations
- **table-types-log-tables-select-select-data.md** (`docs/dbms/table-types/log-tables/table-types-log-tables-select-select-data.md`) - Standard data selection
- **table-types-log-tables-select-select-time-data.md** (`docs/dbms/table-types/log-tables/table-types-log-tables-select-select-time-data.md`) - Time-based data selection
- **table-types-log-tables-select-text-search.md** (`docs/dbms/table-types/log-tables/table-types-log-tables-select-text-search.md`) - Text search capabilities
- **table-types-log-tables-select-network-type.md** (`docs/dbms/table-types/log-tables/table-types-log-tables-select-network-type.md`) - Network type data handling
- **table-types-log-tables-select-simple-join.md** (`docs/dbms/table-types/log-tables/table-types-log-tables-select-simple-join.md`) - Join operations in log tables
- **table-types-log-tables-deleting-data.md** (`docs/dbms/table-types/log-tables/table-types-log-tables-deleting-data.md`) - Data deletion procedures
- **table-types-log-tables-log-indexes.md** (`docs/dbms/table-types/log-tables/table-types-log-tables-log-indexes.md`) - Index management for log tables

#### Lookup Tables
- **table-types-lookup-tables-creating-lookup-tables.md** (`docs/dbms/table-types/lookup-tables/table-types-lookup-tables-creating-lookup-tables.md`) - Creating lookup tables
- **table-types-lookup-tables-inserting-data.md** (`docs/dbms/table-types/lookup-tables/table-types-lookup-tables-inserting-data.md`) - Data insertion in lookup tables
- **table-types-lookup-tables-querying-data.md** (`docs/dbms/table-types/lookup-tables/table-types-lookup-tables-querying-data.md`) - Querying lookup tables
- **table-types-lookup-tables-deleting-data.md** (`docs/dbms/table-types/lookup-tables/table-types-lookup-tables-deleting-data.md`) - Data deletion in lookup tables
- **table-types-lookup-tables-lookup-indexes.md** (`docs/dbms/table-types/lookup-tables/table-types-lookup-tables-lookup-indexes.md`) - Index management for lookup tables

#### Volatile Tables
- **table-types-volatile-tables-creating-volatile-tables.md** (`docs/dbms/table-types/volatile-tables/table-types-volatile-tables-creating-volatile-tables.md`) - Creating volatile tables
- **table-types-volatile-tables-insert-update.md** (`docs/dbms/table-types/volatile-tables/table-types-volatile-tables-insert-update.md`) - Insert and update operations
- **table-types-volatile-tables-querying-data.md** (`docs/dbms/table-types/volatile-tables/table-types-volatile-tables-querying-data.md`) - Querying volatile tables
- **table-types-volatile-tables-deleting-data.md** (`docs/dbms/table-types/volatile-tables/table-types-volatile-tables-deleting-data.md`) - Data deletion in volatile tables
- **table-types-volatile-tables-volatile-indexes.md** (`docs/dbms/table-types/volatile-tables/table-types-volatile-tables-volatile-indexes.md`) - Index management for volatile tables

### SQL Reference
- **sql-reference-ddl.md** (`docs/dbms/sql-reference/sql-reference-ddl.md`) - Data Definition Language (DDL) reference
- **sql-reference-dml.md** (`docs/dbms/sql-reference/sql-reference-dml.md`) - Data Manipulation Language (DML) reference
- **sql-reference-select.md** (`docs/dbms/sql-reference/sql-reference-select.md`) - SELECT statement comprehensive guide
- **sql-reference-select-hint.md** (`docs/dbms/sql-reference/sql-reference-select-hint.md`) - Query hints and optimization
- **sql-reference-functions.md** (`docs/dbms/sql-reference/sql-reference-functions.md`) - Built-in functions reference
- **sql-reference-datatypes.md** (`docs/dbms/sql-reference/sql-reference-datatypes.md`) - Data types and their usage
- **sql-reference-time-expressions.md** (`docs/dbms/sql-reference/sql-reference-time-expressions.md`) - Time expressions and formatting
- **sql-reference-user-manage.md** (`docs/dbms/sql-reference/sql-reference-user-manage.md`) - User management SQL commands
- **sql-reference-sys-session-manage.md** (`docs/dbms/sql-reference/sql-reference-sys-session-manage.md`) - System and session management

### Configuration
- **configuration-property.md** (`docs/dbms/configuration/configuration-property.md`) - System properties configuration
- **configuration-property-cl.md** (`docs/dbms/configuration/configuration-property-cl.md`) - Command-line property configuration
- **configuration-meta-table.md** (`docs/dbms/configuration/configuration-meta-table.md`) - Metadata table configuration
- **configuration-virtual-table.md** (`docs/dbms/configuration/configuration-virtual-table.md`) - Virtual table setup
- **configuration-timezone.md** (`docs/dbms/configuration/configuration-timezone.md`) - Timezone configuration

### Installation
- **installation-package.md** (`docs/dbms/installation/installation-package.md`) - Package installation guide
- **installation-license.md** (`docs/dbms/installation/installation-license.md`) - License installation and management
- **installation-upgrade.md** (`docs/dbms/installation/installation-upgrade.md`) - Upgrade procedures

#### Linux Installation
- **installation-linux-linux-env.md** (`docs/dbms/installation/linux/installation-linux-linux-env.md`) - Linux environment setup
- **installation-linux-tgz-install.md** (`docs/dbms/installation/linux/installation-linux-tgz-install.md`) - TAR.GZ installation on Linux
- **installation-linux-docker-install.md** (`docs/dbms/installation/linux/installation-linux-docker-install.md`) - Docker installation on Linux

#### Windows Installation
- **installation-windows-windows-env.md** (`docs/dbms/installation/windows/installation-windows-windows-env.md`) - Windows environment setup
- **installation-windows-msi-install.md** (`docs/dbms/installation/windows/installation-windows-msi-install.md`) - MSI installation on Windows

#### Cluster Installation
- **installation-cluster-cluster-env.md** (`docs/dbms/installation/cluster/installation-cluster-cluster-env.md`) - Cluster environment setup
- **installation-cluster-command-line.md** (`docs/dbms/installation/cluster/installation-cluster-command-line.md`) - Command-line cluster installation
- **installation-cluster-command-line-coordinator-deployer-install.md** (`docs/dbms/installation/cluster/installation-cluster-command-line-coordinator-deployer-install.md`) - Coordinator and deployer installation
- **installation-cluster-command-line-lookup-broker-warehouse-install.md** (`docs/dbms/installation/cluster/installation-cluster-command-line-lookup-broker-warehouse-install.md`) - Lookup, broker, and warehouse installation

### Common Tasks
- **common-tasks-connecting.md** (`docs/dbms/common-tasks/common-tasks-connecting.md`) - Database connection procedures
- **common-tasks-querying.md** (`docs/dbms/common-tasks/common-tasks-querying.md`) - Common query patterns and examples
- **common-tasks-importing-data.md** (`docs/dbms/common-tasks/common-tasks-importing-data.md`) - Data import procedures
- **common-tasks-user-management.md** (`docs/dbms/common-tasks/common-tasks-user-management.md`) - User and permission management
- **common-tasks-backup-recovery.md** (`docs/dbms/common-tasks/common-tasks-backup-recovery.md`) - Backup and recovery procedures

### Advanced Features
- **advanced-features-overview.md** (`docs/dbms/advanced-features/advanced-features-overview.md`) - Advanced features overview
- **advanced-features-create-delete.md** (`docs/dbms/advanced-features/advanced-features-create-delete.md`) - Advanced create/delete operations
- **advanced-features-startup-shutdown.md** (`docs/dbms/advanced-features/advanced-features-startup-shutdown.md`) - Startup and shutdown procedures
- **advanced-features-retention.md** (`docs/dbms/advanced-features/advanced-features-retention.md`) - Data retention policies
- **advanced-features-sample.md** (`docs/dbms/advanced-features/advanced-features-sample.md`) - Advanced feature examples
- **advanced-features-backup-restore.md** (`docs/dbms/advanced-features/advanced-features-backup-restore.md`) - Advanced backup and restore
- **advanced-features-database-mount.md** (`docs/dbms/advanced-features/advanced-features-database-mount.md`) - Database mounting procedures

### SDK Integration
- **sdk-integration-cli-odbc.md** (`docs/dbms/sdk-integration/sdk-integration-cli-odbc.md`) - CLI and ODBC integration
- **sdk-integration-cli-odbc-example.md** (`docs/dbms/sdk-integration/sdk-integration-cli-odbc-example.md`) - CLI/ODBC examples
- **sdk-integration-jdbc.md** (`docs/dbms/sdk-integration/sdk-integration-jdbc.md`) - JDBC driver integration
- **sdk-integration-python.md** (`docs/dbms/sdk-integration/sdk-integration-python.md`) - Python SDK integration
- **sdk-integration-nodejs.md** (`docs/dbms/sdk-integration/sdk-integration-nodejs.md`) - Node.js SDK integration
- **sdk-integration-dotnet.md** (`docs/dbms/sdk-integration/sdk-integration-dotnet.md`) - .NET SDK integration

### Tools Reference
- **tools-reference-machsql.md** (`docs/dbms/tools-reference/tools-reference-machsql.md`) - MachSQL tool reference
- **tools-reference-machloader.md** (`docs/dbms/tools-reference/tools-reference-machloader.md`) - MachLoader tool reference
- **tools-reference-machadmin.md** (`docs/dbms/tools-reference/tools-reference-machadmin.md`) - MachAdmin tool reference
- **tools-reference-machcoordinatoradmin.md** (`docs/dbms/tools-reference/tools-reference-machcoordinatoradmin.md`) - MachCoordinatorAdmin reference
- **tools-reference-machdeployeradmin.md** (`docs/dbms/tools-reference/tools-reference-machdeployeradmin.md`) - MachDeployerAdmin reference
- **tools-reference-csv.md** (`docs/dbms/tools-reference/tools-reference-csv.md`) - CSV tools reference

### Tutorials
- **tutorials-iot-sensor-data.md** (`docs/dbms/tutorials/tutorials-iot-sensor-data.md`) - IoT sensor data tutorial
- **tutorials-application-logs.md** (`docs/dbms/tutorials/tutorials-application-logs.md`) - Application log analysis tutorial
- **tutorials-realtime-analytics.md** (`docs/dbms/tutorials/tutorials-realtime-analytics.md`) - Real-time analytics tutorial
- **tutorials-reference-data.md** (`docs/dbms/tutorials/tutorials-reference-data.md`) - Reference data management tutorial

### Troubleshooting
- **troubleshooting-common-issues.md** (`docs/dbms/troubleshooting/troubleshooting-common-issues.md`) - Common issues and solutions
- **troubleshooting-error-code.md** (`docs/dbms/troubleshooting/troubleshooting-error-code.md`) - Error codes and meanings
- **troubleshooting-memory-error.md** (`docs/dbms/troubleshooting/troubleshooting-memory-error.md`) - Memory-related error troubleshooting

## TQL (Time Query Language)

### TQL Fundamentals
- **tql-guide.md** (`docs/tql/tql-guide.md`) - Complete TQL guide and syntax
- **tql-reference.md** (`docs/tql/tql-reference.md`) - TQL reference documentation
- **tql-reading.md** (`docs/tql/tql-reading.md`) - Data reading with TQL
- **tql-writing.md** (`docs/tql/tql-writing.md`) - Data writing with TQL
- **tql-http.md** (`docs/tql/tql-http.md`) - TQL over HTTP API

### TQL Operations
- **tql-src.md** (`docs/tql/tql-src.md`) - TQL source operations
- **tql-sink.md** (`docs/tql/tql-sink.md`) - TQL sink operations
- **tql-map.md** (`docs/tql/tql-map.md`) - TQL map transformations
- **tql-group.md** (`docs/tql/tql-group.md`) - TQL grouping operations
- **tql-filters.md** (`docs/tql/tql-filters.md`) - TQL filtering capabilities
- **tql-utilities.md** (`docs/tql/tql-utilities.md`) - TQL utility functions
- **tql-time-examples.md** (`docs/tql/tql-time-examples.md`) - Time-based TQL examples
- **tql-fft.md** (`docs/tql/tql-fft.md`) - Fast Fourier Transform in TQL
- **tql-script.md** (`docs/tql/tql-script.md`) - TQL scripting capabilities
- **tql-html.md** (`docs/tql/tql-html.md`) - HTML output generation

### TQL Charts
- **line-chart.md** (`docs/tql/chart/line-chart.md`) - Line chart generation
- **bar-chart.md** (`docs/tql/chart/bar-chart.md`) - Bar chart generation
- **pie-chart.md** (`docs/tql/chart/pie-chart.md`) - Pie chart generation
- **scatter-chart.md** (`docs/tql/chart/scatter-chart.md`) - Scatter plot generation
- **heatmap-chart.md** (`docs/tql/chart/heatmap-chart.md`) - Heatmap visualization
- **gauge-chart.md** (`docs/tql/chart/gauge-chart.md`) - Gauge chart creation
- **radar-chart.md** (`docs/tql/chart/radar-chart.md`) - Radar chart generation
- **boxplot-chart.md** (`docs/tql/chart/boxplot-chart.md`) - Box plot visualization
- **candlestick-chart.md** (`docs/tql/chart/candlestick-chart.md`) - Candlestick chart for financial data
- **liquidfill-chart.md** (`docs/tql/chart/liquidfill-chart.md`) - Liquid fill gauge charts
- **3d-line-chart.md** (`docs/tql/chart/3d-line-chart.md`) - 3D line chart visualization
- **3d-bar-chart.md** (`docs/tql/chart/3d-bar-chart.md`) - 3D bar chart generation
- **3d-globe-chart.md** (`docs/tql/chart/3d-globe-chart.md`) - 3D globe visualization
- **geojson-chart.md** (`docs/tql/chart/geojson-chart.md`) - GeoJSON mapping charts
- **others-chart.md** (`docs/tql/chart/others-chart.md`) - Other chart types
- **chart-html-embedding.md** (`docs/tql/chart/chart-html-embedding.md`) - Embedding charts in HTML

### TQL Geographic Maps
- **geomap_guide.md** (`docs/tql/geomap/geomap_guide.md`) - Geographic mapping guide
- **geomap-html-embedding.md** (`docs/tql/geomap/geomap-html-embedding.md`) - Embedding geographic maps in HTML

## SQL Examples and Guides

### SQL Operations
- **sql-guide.md** (`docs/sql/sql-guide.md`) - General SQL guide for Machbase Neo
- **sql-tag-table.md** (`docs/sql/sql-tag-table.md`) - Tag table SQL operations
- **sql-tag-statistics.md** (`docs/sql/sql-tag-statistics.md`) - Tag statistics calculations
- **sql-storage-size.md** (`docs/sql/sql-storage-size.md`) - Storage size queries
- **sql-backup-mount.md** (`docs/sql/sql-backup-mount.md`) - Backup and mount SQL commands
- **sql-rollup.md** (`docs/sql/sql-rollup.md`) - Rollup operations with SQL
- **sql-duplicate-removal.md** (`docs/sql/sql-duplicate-removal.md`) - Duplicate data removal techniques
- **sql-outlier-removal.md** (`docs/sql/sql-outlier-removal.md`) - Outlier detection and removal

## JavaScript Shell (JSH)

### JSH Core
- **javascript-guide.md** (`docs/jsh/javascript-guide.md`) - JavaScript shell complete guide

### JSH Modules
- **javascript-db-module.md** (`docs/jsh/javascript-db-module.md`) - Database interaction module
- **javascript-http-module.md** (`docs/jsh/javascript-http-module.md`) - HTTP client module
- **javascript-mqtt-module.md** (`docs/jsh/javascript-mqtt-module.md`) - MQTT client module
- **javascript-system-module.md** (`docs/jsh/javascript-system-module.md`) - System operations module
- **javascript-process-module.md** (`docs/jsh/javascript-process-module.md`) - Process management module
- **javascript-filter-module.md** (`docs/jsh/javascript-filter-module.md`) - Data filtering module
- **javascript-generator-module.md** (`docs/jsh/javascript-generator-module.md`) - Data generation module
- **javascript-analysis-module.md** (`docs/jsh/javascript-analysis-module.md`) - Data analysis module
- **javascript-spatial-module.md** (`docs/jsh/javascript-spatial-module.md`) - Spatial data module
- **javascript-psutil-module.md** (`docs/jsh/javascript-psutil-module.md`) - System utilities module
- **javascript-opcua-module.md** (`docs/jsh/javascript-opcua-module.md`) - OPC UA client module
- **javascript-mat-module.md** (`docs/jsh/javascript-mat-module.md`) - Matrix operations module
- **javascript-examples.md** (`docs/jsh/javascript-examples.md`) - JavaScript examples and use cases

## Operations and Administration

### Server Operations
- **command-line.md** (`docs/operations/command-line.md`) - Command-line interface reference
- **server-config.md** (`docs/operations/server-config.md`) - Server configuration options
- **address-ports.md** (`docs/operations/address-ports.md`) - Address and port configuration
- **metrics.md** (`docs/operations/metrics.md`) - System metrics and monitoring

### Service Management
- **service-linux.md** (`docs/operations/service-linux.md`) - Linux service management
- **service-windows.md** (`docs/operations/service-windows.md`) - Windows service management

## Utilities

### Core Utilities
- **dashboard.md** (`docs/utilities/dashboard.md`) - Dashboard functionality and usage
- **import-export.md** (`docs/utilities/import-export.md`) - Data import and export utilities
- **tag-analyzer.md** (`docs/utilities/tag-analyzer.md`) - Tag analysis utilities
- **timer.md** (`docs/utilities/timer.md`) - Timer and scheduling utilities
- **timeformat-tz.md** (`docs/utilities/timeformat-tz.md`) - Time formatting and timezone utilities

### Shell Utilities
- **shell-access.md** (`docs/utilities/shell/shell-access.md`) - Shell access methods
- **shell-run.md** (`docs/utilities/shell/shell-run.md`) - Running shell commands
- **shell-custom.md** (`docs/utilities/shell/shell-custom.md`) - Custom shell configuration

## Data Bridges

### Bridge Overview
- **bridge-overview.md** (`docs/bridges/bridge-overview.md`) - Data bridge architecture and overview

### Specific Bridges
- **bridge-postgresql.md** (`docs/bridges/bridge-postgresql.md`) - PostgreSQL bridge configuration
- **bridge-mysql.md** (`docs/bridges/bridge-mysql.md`) - MySQL bridge configuration
- **bridge-mssql.md** (`docs/bridges/bridge-mssql.md`) - Microsoft SQL Server bridge
- **bridge-sqlite.md** (`docs/bridges/bridge-sqlite.md`) - SQLite bridge configuration
- **bridge-mqtt.md** (`docs/bridges/bridge-mqtt.md`) - MQTT bridge configuration
- **bridge-nats.md** (`docs/bridges/bridge-nats.md`) - NATS bridge configuration

## Security

### Security Documentation
- **security.md** (`docs/security/security.md`) - Comprehensive security guide and best practices

## Installation

### General Installation
- **installation.md** (`docs/installation/installation.md`) - General installation guide and overview

---

*This index is automatically generated and contains all available documentation for Machbase Neo. Use this reference to help users find the most relevant documentation for their specific needs.*