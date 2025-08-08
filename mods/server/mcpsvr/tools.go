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

var tools = []server.ServerTool{
	{
		Tool:    mcp.NewTool("now", mcp.WithDescription("Get current time in Unix Epoch Nanosecond")),
		Handler: getNowHandler,
	},
	{
		Tool: mcp.NewTool("timeformat", mcp.WithDescription("Format time in a specific format"),
			mcp.WithString("time", mcp.Required(), mcp.Description("Time to format (Unix Epoch Nanosecond)")),
			mcp.WithString("format", mcp.DefaultString("RFC3339"), mcp.Description("Time format (e.g., RFC3339, Unix, etc.)")),
			mcp.WithString("location", mcp.DefaultString("Local"), mcp.Description("Time zone location (e.g., Local, UTC, Asia/Seoul)")),
		),
		Handler: func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
		},
	},
	{
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
		Handler: genSqlHandler,
	},
	{
		Tool: mcp.NewTool("exec_query_sql",
			mcp.WithDescription("Execute a specified SQL query and return the results."),
			mcp.WithString("query",
				mcp.Required(),
				mcp.Description("SQL query to execute"),
			),
		),
		Handler: queryHandler,
	},
}

func getNowHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	now := time.Now().UnixNano()
	return mcp.NewToolResultText(fmt.Sprintf("%d", now)), nil
}

func genSqlHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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

func queryHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
		fmt.Println("Query execution failed:", err)
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
