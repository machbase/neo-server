package mcpsvr

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/machbase/neo-server/v8/api"
	"github.com/machbase/neo-server/v8/mods/util"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func init() {
	RegisterTools(
		ToolGetNowHandler,
		ToolTimeformat,
		ToolExecQuerySQL,
		ToolListTables,
		ToolDescribeTable,
	)
}

var ToolListTables = server.ServerTool{
	Tool: mcp.NewTool("machbase_list_tables",
		mcp.WithDescription("Machbase Neo 데이터베이스의 테이블 목록을 조회합니다"),
		mcp.WithBoolean("show_all", mcp.Required(), mcp.Description("Show all tables including hidden and system tables")),
	),
	Handler: toolMachbaseListTablesFunc,
}

func toolMachbaseListTablesFunc(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	db := api.Default()
	conn, err := db.Connect(ctx, api.WithTrustUser("sys"))
	if err != nil {
		return mcp.NewToolResultError("failed to connect to database: " + err.Error()), nil
	}
	defer conn.Close()

	showAll := request.GetBool("show_all", false)
	list, err := api.ListTables(ctx, conn, showAll)
	if err != nil {
		return mcp.NewToolResultError("failed to list tables: " + err.Error()), nil
	}

	var tables []string
	for _, table := range list {
		tables = append(tables, table.Name)
	}
	return mcp.NewToolResultText(strings.Join(tables, "\n")), nil
}

var ToolGetNowHandler = server.ServerTool{
	Tool:    mcp.NewTool("now", mcp.WithDescription("Returns the current time as a Unix Epoch Nanosecond value")),
	Handler: toolGetNowHandlerFunc,
}

func toolGetNowHandlerFunc(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	now := time.Now().UnixNano()
	return mcp.NewToolResultText(fmt.Sprintf("%d", now)), nil
}

var ToolTimeformat = server.ServerTool{
	Tool: mcp.NewTool("timeformat", mcp.WithDescription("Format time in a specific format"),
		mcp.WithString("time", mcp.Required(), mcp.Description("Time to format (Unix Epoch Nanosecond)")),
		mcp.WithString("format", mcp.DefaultString("RFC3339"), mcp.Description("Time format (e.g., RFC3339, Unix, etc.)")),
		mcp.WithString("location", mcp.DefaultString("Local"), mcp.Description("Time zone location (e.g., Local, UTC, Asia/Seoul)")),
	),
	Handler: toolTimeformatFuncFunc,
}

func toolTimeformatFuncFunc(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	timeStr, err := request.RequireString("time")
	if err != nil {
		return mcp.NewToolResultError("time must be a string: " + err.Error()), nil
	}
	format, err := request.RequireString("format")
	if err != nil {
		return mcp.NewToolResultError("format must be a string: " + err.Error()), nil
	}
	locationStr, err := request.RequireString("location")
	if err != nil {
		return mcp.NewToolResultError("location must be a string: " + err.Error()), nil
	}
	location, err := time.LoadLocation(locationStr)
	if err != nil {
		return mcp.NewToolResultError("invalid location: " + err.Error()), nil
	}
	nano, err := strconv.ParseInt(timeStr, 10, 64)
	if err != nil {
		return mcp.NewToolResultError("invalid time format: " + err.Error()), nil
	}
	unixTime := time.Unix(0, nano)
	timeFormat := util.NewTimeFormatter(util.Timeformat(format), util.TimeLocation(location))
	formattedTime := timeFormat.Format(unixTime)
	return mcp.NewToolResultText(formattedTime), nil
}

var ToolDescribeTable = server.ServerTool{
	Tool: mcp.NewTool("machbase_describe_table",
		mcp.WithDescription("Describe the schema of a specified table in Machbase Neo"),
		mcp.WithString("table", mcp.Required(), mcp.Description("Name of the table to describe")),
		mcp.WithBoolean("show_all", mcp.Description("Show all columns including hidden columns")),
	),
	Handler: toolDescribeTableFunc,
}

func toolDescribeTableFunc(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	table, err := request.RequireString("table")
	if err != nil {
		return mcp.NewToolResultError("table must be a string: " + err.Error()), nil
	}
	showAll := request.GetBool("show_all", false)

	db := api.Default()
	conn, err := db.Connect(ctx, api.WithTrustUser("sys"))
	if err != nil {
		return mcp.NewToolResultError("failed to connect to database: " + err.Error()), nil
	}
	defer conn.Close()

	schema, err := api.DescribeTable(ctx, conn, table, showAll)
	if err != nil {
		return mcp.NewToolResultError("failed to describe table: " + err.Error()), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Table: %s\n", table))
	sb.WriteString("Columns:\n")
	for _, col := range schema.Columns {
		sb.WriteString(fmt.Sprintf("- %s: %s\n", col.Name, col.Type))
	}
	return mcp.NewToolResultText(sb.String()), nil
}

var ToolGenSQL = server.ServerTool{
	Tool: mcp.NewTool("gen_sql",
		mcp.WithDescription("Generate SQL query to retrieve data from a specified table within a given time range."),
		mcp.WithString("table",
			mcp.Required(),
			mcp.Description("Name of the target table to query"),
		),
		mcp.WithNumber("timeFrom",
			mcp.Required(),
			mcp.Description("Start time to query (Unix Epoch Nanosecond)"),
		),
		mcp.WithNumber("timeTo",
			mcp.Description("End time to query (Unix Epoch Nanosecond)"),
		),
		mcp.WithNumber("limit",
			mcp.DefaultNumber(10),
			mcp.Description("Maximum number of rows to retrieve"),
		),
	),
	Handler: toolGenSqlFunc,
}

func toolGenSqlFunc(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	table, err := request.RequireString("table")
	if err != nil {
		return mcp.NewToolResultError("table must be a string: " + err.Error()), nil
	}
	timeFrom := request.GetInt("timeFrom", 0)
	timeTo := request.GetInt("timeTo", 0)
	limit := request.GetInt("limit", 10)

	sql := fmt.Sprintf("SELECT * FROM %s", table)
	if timeFrom != 0 {
		sql += fmt.Sprintf(" WHERE time >= %d", timeFrom)
	}
	if timeTo != 0 && timeFrom == 0 {
		sql += fmt.Sprintf(" AND time <= %d", timeTo)
	}
	if limit > 0 {
		sql += fmt.Sprintf(" LIMIT %d", limit)
	}

	return mcp.NewToolResultText(sql), nil
}

var ToolExecQuerySQL = server.ServerTool{
	Tool: mcp.NewTool("machbase_execute_sql",
		mcp.WithDescription("Execute a specified SQL query and return the results."),
		mcp.WithString("query",
			mcp.Required(),
			mcp.Description("SQL query to execute"),
		),
	),
	Handler: toolExecQuerySQLFunc,
}

func toolExecQuerySQLFunc(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	query, err := request.RequireString("query")
	if err != nil {
		return mcp.NewToolResultError("query must be a string: " + err.Error()), nil
	}
	db := api.Default()
	conn, err := db.Connect(ctx, api.WithPassword("sys", "manager"))
	if err != nil {
		return mcp.NewToolResultError("failed to connect to database: " + err.Error()), nil
	}
	defer conn.Close()

	row, err := conn.Query(ctx, query)
	if err != nil {
		return mcp.NewToolResultError("query execution failed: " + err.Error()), nil
	}
	defer row.Close()

	cols, err := row.Columns()
	if err != nil {
		return mcp.NewToolResultError("failed to get columns: " + err.Error()), nil
	}

	timeformat := util.NewTimeFormatter(util.Timeformat(time.RFC3339), util.TimeLocation(time.Local))

	sb := &strings.Builder{}
	sb.WriteString(strings.Join(cols.Names(), ",") + "\n")
	for row.Next() {
		values, err := cols.MakeBuffer()
		if err != nil {
			return mcp.NewToolResultError("failed to create buffer for columns: " + err.Error()), nil
		}
		if err := row.Scan(values...); err != nil {
			return mcp.NewToolResultError("failed to scan row: " + err.Error()), nil
		}
		for i := range values {
			if i > 0 {
				sb.WriteString(",")
			}
			var v any
			switch val := values[i].(type) {
			case *string:
				v = *val
			case *float64:
				v = *val
			case *float32:
				v = *val
			case *int64:
				v = *val
			case *int32:
				v = *val
			case *int16:
				v = *val
			case *int:
				v = *val
			case *time.Time:
				v = timeformat.Format(*val)
			case time.Time:
				v = timeformat.Format(val)
			default:
				v = val
			}
			sb.WriteString(fmt.Sprintf("%v", v))
		}
		sb.WriteString("\n")
	}

	return mcp.NewToolResultText(sb.String()), nil
}
