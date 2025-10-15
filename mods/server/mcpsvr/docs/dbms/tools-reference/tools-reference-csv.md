# csvimport / csvexport

'csvimport' and 'csvexport' are tools used to import/export CSV files to the Machbase server.

The options have been simplified for simpler use of the CSV file using the machloader.

In addition to the options described below, all options available in machloader are available.

## csvimport 

CSV files can be easily entered into the server using csvimport.

### Basic Usage

Enter the table name and data file name according to the following options.

Options:

```
-t: table name specification option
-d: data file naming options
* You can do this with just the table name and data file name without specifying the option.
```

Example:

```
csvimport -t table_name -d table_name.csv
csvimport table_name file_path
csvimport file_path table_name
```

### CSV Header Exception

Use the following option to enter the CSV file except for the header at the time of input.

Options:

```
-H: Will not recognize the first line of the csv file as a header.
```

Example:

```
csvimport -t table_name -d table_name.csv -H
```

### Automatic Table Creation

If a table is not created to be entered at the time of input, the table can be created at the same time through the following options.

Option

```
-C: Automatically creates the table during import. Column names are automatically created as c0, c1, .... The created column is varchar (32767) type.
-H: Creates column name with csv header name during import.
```

Example:

```
csvimport -t table_name -d table_name.csv -C
csvimport -t table_name -d table_name.csv -C -H
```

## csvexport

The database table data can be easily exported to the CSV file with 'csvexport'.

### Basic Usage

Option:

```
-t: table name specification option
-d: data file naming options
* You can do this with just the table name and data file name without specifying the option.
```

Example:

```
csvexport -t table_name -d table_name.csv
csvexport table_name file_path
csvexport file_path table_name
```

### Using CSV Header

With the following option, you can add a header to the CSV file to be exported with a column name.

Option:

```
-H: Creates the header of the csv file with the table column name.
```

Example:

```
csvexport -t table_name -d table_name.csv -H
```
