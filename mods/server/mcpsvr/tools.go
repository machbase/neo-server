package mcpsvr

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/machbase/neo-server/v8/api"
	"github.com/machbase/neo-server/v8/mods/logging"
	"github.com/machbase/neo-server/v8/mods/tql"
	"github.com/machbase/neo-server/v8/mods/util"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func init() {
	RegisterTools(
		ToolGetNowHandler,
		ToolTimeformat,
		ToolExecSQL,
		ToolExecTQL,
		ToolListTables,
		ToolListTableTags,
		ToolDescribeTable,
		ToolDocsAvailableList,
		ToolDocsFetch,
	)
}

var ToolListTables = server.ServerTool{
	Tool: mcp.NewTool("machbase_list_tables",
		mcp.WithDescription("List tables in Machbase Neo database"),
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

var ToolListTableTags = server.ServerTool{
	Tool: mcp.NewTool("list_table_tags",
		mcp.WithDescription("List tags in Machbase Neo database for a specified table"),
		mcp.WithBoolean("table", mcp.Required(), mcp.Description("Name of the table to list tags")),
	),
	Handler: toolMachbaseListTagsFunc,
}

func toolMachbaseListTagsFunc(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	table, err := request.RequireString("table")
	if err != nil {
		return mcp.NewToolResultError("table must be a string: " + err.Error()), nil
	}

	db := api.Default()
	conn, err := db.Connect(ctx, api.WithTrustUser("sys"))
	if err != nil {
		return mcp.NewToolResultError("failed to connect to database: " + err.Error()), nil
	}
	defer conn.Close()

	tags, err := api.ListTags(ctx, conn, table)
	if err != nil {
		return mcp.NewToolResultError("failed to list tags: " + err.Error()), nil
	}
	list := make([]string, len(tags))
	for i, tag := range tags {
		list[i] = tag.Name
	}
	return mcp.NewToolResultText(strings.Join(list, "\n")), nil
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

var ToolExecSQL = server.ServerTool{
	Tool: mcp.NewTool("execute_sql_query",
		mcp.WithDescription(`Execute a specified SQL query and return the results.

	**IMPORTANT: Always check table structure first to understand column names,
				data types, and time intervals before execution.**
	**MANDATORY: Must use Machbase Neo documentation only.
				Use docs_fetch to find exact syntax before writing any queries.
				General SQL knowledge must not be used
				- only documented Machbase Neo syntax and functions are allowed.**
	**EXECUTION POLICY: Must test and verify all SQL queries before providing
				them as answers. Only provide successfully executed and validated code to users.**

	If no data is returned, it will be treated as a failure.	
`),
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

var ToolExecTQL = server.ServerTool{
	Tool: mcp.NewTool("execute_tql_script",
		mcp.WithDescription(`Execute a specified TQL script and return the results.

	**CRITICAL: Before executing, analyze target table structure and
				time intervals (minute/hour/daily data) as TQL operations
				heavily depend on correct time-based aggregations.**
	**MANDATORY: TQL syntax is unique to Machbase Neo. Must reference
				documentation using docs_fetch before writing any TQL scripts.
				Only use syntax and examples found in official documentation
				- no assumptions or general query language knowledge allowed.**
	**EXECUTION POLICY: Must test and verify all TQL scripts before providing them as answers.
				Only provide successfully executed and validated code to users.**
`),
		mcp.WithString("script", mcp.Required(), mcp.Description("TQL script to execute")),
	),
	Handler: toolExecTQLFunc,
}

func toolExecTQLFunc(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	script, err := request.RequireString("script")
	if err != nil {
		return mcp.NewToolResultError("script must be a string: " + err.Error()), nil
	}
	codeReader := strings.NewReader(script)
	outWriter := &strings.Builder{}

	task := tql.NewTaskContext(ctx)
	task.SetParams(map[string][]string{})
	task.SetLogWriter(logging.GetLog("_llm.tql"))
	task.SetOutputWriterJson(&util.NopCloseWriter{Writer: outWriter}, true)
	task.SetDatabase(api.Default())
	if err := task.Compile(codeReader); err != nil {
		return mcp.NewToolResultError("TQL parse error: " + err.Error()), nil
	}
	// task.SetVolatileAssetsProvider(svr.memoryFs)
	// ctx.Writer.Header().Set("Content-Type", task.OutputContentType())
	// ctx.Writer.Header().Set("Content-Encoding", task.OutputContentEncoding())
	// if chart := task.OutputChartType(); len(chart) > 0 {
	// 	ctx.Writer.Header().Set(TqlHeaderChartType, chart)
	// }
	go func() {
		<-ctx.Done()
		task.Cancel()
	}()

	result := task.Execute()
	if result == nil {
		return mcp.NewToolResultError("task result is empty"), nil
		//	} else if result.IsDbSink {
		// ctx.JSON(http.StatusOK, result)
		//	} else if !outWriter.Written() {
		// clear headers for the json result
		// ctx.Writer.Header().Set("Content-Type", "application/json")
		// ctx.Writer.Header().Del("Content-Encoding")
		// ctx.Writer.Header().Del(TqlHeaderChartType)
		// ctx.JSON(http.StatusOK, result)
	}
	obj := map[string]any{
		"message": result.Message,
		"result":  outWriter.String(),
	}
	b, _ := json.Marshal(obj)
	return mcp.NewToolResultText(string(b)), nil
}

//go:embed docs/*
var docsFS embed.FS

var ToolDocsAvailableList = server.ServerTool{
	Tool: mcp.NewTool("docs_available_list",
		mcp.WithDescription("List all available documentation files."),
	),
	Handler: toolDocsAvailableListFunc,
}

func toolDocsAvailableListFunc(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// return the content of ./docs/index.md file
	content, err := docsFS.ReadFile("docs/index.md")
	if err != nil {
		return mcp.NewToolResultError("failed to read docs/index.md: " + err.Error()), nil
	}
	return mcp.NewToolResultText(string(content)), nil
}

var ToolDocsFetch = server.ServerTool{
	Tool: mcp.NewTool("docs_fetch",
		mcp.WithDescription(`Fetch the content of a specified documentation file.

    **MANDATORY RESTRICTION**: 
    ALWAYS search non-dbms folders (operations/, sql/, tql/, api/, utilities/, etc.) FIRST for all questions.

    Use paths starting with "dbms/" ONLY when:
    - The user's question explicitly mentions "DBMS" keyword, OR
    - You have already searched at least one relevant non-dbms folder and found no information

    Before using dbms/, briefly state which non-dbms folder you searched.
    
    Args:
        file_identifier: relative path (e.g., "sql/rollup.md")
`),
		mcp.WithString("filename", mcp.Required(), mcp.Description("Name of the documentation file to fetch")),
	),
	Handler: toolDocsFetchFunc,
}

func toolDocsFetchFunc(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	docName, err := request.RequireString("filename")
	if err != nil {
		return mcp.NewToolResultError("filename must be a string: " + err.Error()), nil
	}
	// sanitize docName to prevent directory traversal attacks
	if strings.Contains(docName, "..") || strings.HasPrefix(docName, "/") || strings.HasPrefix(docName, "\\") {
		return mcp.NewToolResultError("invalid filename: " + docName), nil
	}
	if !strings.HasPrefix(docName, "docs/") {
		docName = "docs/" + docName
	}
	content, err := docsFS.ReadFile(docName)
	if err != nil {
		return mcp.NewToolResultError("failed to read documentation file: " + err.Error()), nil
	}

	return mcp.NewToolResultText(string(content)), nil
}
