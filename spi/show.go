package spi

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	client "github.com/machbase/neo-client/v2"
	"github.com/machbase/neo-client/v2/api"
	"github.com/machbase/neo-server/v8/mods/util"
)

type ShowOption func(*showOptions)

type showOptions struct {
	database    string
	user        string
	like        string
	hasDatabase bool
	hasUser     bool
	hasLike     bool
}

func WithDatabase(database string) ShowOption {
	return func(options *showOptions) {
		options.database = strings.ToUpper(database)
		options.hasDatabase = true
	}
}

func WithUser(user string) ShowOption {
	return func(options *showOptions) {
		options.user = strings.ToUpper(user)
		options.hasUser = true
	}
}

func WithLike(pattern string) ShowOption {
	return func(options *showOptions) {
		options.like = pattern
		options.hasLike = true
	}
}

func newShowOptions(options ...ShowOption) showOptions {
	var result showOptions
	for _, option := range options {
		option(&result)
	}
	return result
}

func (options *showOptions) validate() error {
	if options.hasLike {
		if options.like == "" {
			return fmt.Errorf("LIKE pattern must not be empty")
		}
		_, _, _, err := parseLikePattern(options.like)
		if err != nil {
			return err
		}
	}
	return nil
}

func resolveShowScope(ctx context.Context, conn *sql.Conn, options *showOptions) error {
	if err := options.validate(); err != nil {
		return err
	}

	var database string
	if err := conn.QueryRowContext(ctx, "SELECT current_database()").Scan(&database); err != nil {
		return err
	}
	user, err := currentShowUser(ctx, conn)
	if err != nil {
		return err
	}
	if !options.hasDatabase {
		options.database = strings.ToUpper(database)
	}
	if !options.hasUser {
		options.user = strings.ToUpper(user)
	}
	if !options.hasDatabase {
		return nil
	}

	var exists int
	err = conn.QueryRowContext(ctx, "SELECT 1 FROM V$DATABASES WHERE UPPER(NAME) = ?", options.database).Scan(&exists)
	if err == sql.ErrNoRows {
		return fmt.Errorf("database %q does not exist", options.database)
	}
	return err
}

// TODO: Replace the temporary SYS fallback with SELECT current_user() after
// https://github.com/machbase/dbms-nfx/issues/4131 is available in the engine.
func currentShowUser(context.Context, *sql.Conn) (string, error) {
	return "SYS", nil
}

func (options *showOptions) likeClause(column string, args *[]any) string {
	if !options.hasLike {
		return ""
	}
	_, pattern, hasWildcard, _ := parseLikePattern(options.like)
	if !hasWildcard {
		*args = append(*args, strings.ToUpper(pattern))
		return fmt.Sprintf("AND UPPER(%s) = ?", column)
	}
	*args = append(*args, strings.ToUpper(options.like))
	return fmt.Sprintf("AND UPPER(%s) LIKE ?", column)
}

func (options *showOptions) databaseClause(column string, args *[]any) string {
	*args = append(*args, options.database)
	return fmt.Sprintf("AND UPPER(%s) = ?", column)
}

func (options *showOptions) userClause(column string, args *[]any) string {
	*args = append(*args, options.user)
	return fmt.Sprintf("AND UPPER(%s) = ?", column)
}

func LikeMatch(pattern string, value string) bool {
	regex, _, _, err := parseLikePattern(pattern)
	if err != nil {
		return false
	}
	return regex.MatchString(value)
}

func parseLikePattern(pattern string) (*regexp.Regexp, string, bool, error) {
	var expression strings.Builder
	var literal strings.Builder
	expression.WriteString("(?i)^")
	hasWildcard := false
	for index := 0; index < len(pattern); index++ {
		character := pattern[index]
		if character == '\\' {
			if index+1 == len(pattern) {
				return nil, "", false, fmt.Errorf("LIKE pattern must not end with an escape character")
			}
			index++
			expression.WriteString(regexp.QuoteMeta(string(pattern[index])))
			literal.WriteByte(pattern[index])
			continue
		}
		switch character {
		case '%':
			expression.WriteString(".*")
			hasWildcard = true
		case '_':
			expression.WriteByte('.')
			hasWildcard = true
		default:
			expression.WriteString(regexp.QuoteMeta(string(character)))
			literal.WriteByte(character)
		}
	}
	expression.WriteString("$")
	regex, err := regexp.Compile(expression.String())
	return regex, literal.String(), hasWildcard, err
}

type ShowStatement struct {
	Command string
	Args    []string
	All     bool

	HasFrom  bool
	Database string
	User     string

	HasLike bool
	Like    string
}

type showToken struct {
	value  string
	quoted bool
}

func ParseShowStatement(text string) (*ShowStatement, error) {
	tokens, err := tokenizeShowStatement(text)
	if err != nil {
		return nil, err
	}
	if len(tokens) == 0 {
		return nil, fmt.Errorf("f(SQL) empty SHOW statement")
	}
	if !strings.EqualFold(tokens[0].value, "show") {
		return nil, fmt.Errorf("f(SQL) invalid SHOW statement")
	}

	statement := &ShowStatement{}
	seenWith := false
	for index := 1; index < len(tokens); index++ {
		token := tokens[index]
		switch strings.ToLower(token.value) {
		case "from", "in":
			if statement.HasFrom {
				return nil, fmt.Errorf("f(SQL) show %s: FROM specified more than once", showCommandName(statement))
			}
			if index+1 >= len(tokens) || tokens[index+1].quoted {
				return nil, fmt.Errorf("f(SQL) show %s: missing database after %s", showCommandName(statement), strings.ToUpper(token.value))
			}
			database, user, err := parseShowScope(tokens[index+1].value)
			if err != nil {
				return nil, err
			}
			statement.HasFrom = true
			statement.Database = database
			statement.User = user
			index++
		case "like":
			if statement.HasLike {
				return nil, fmt.Errorf("f(SQL) show %s: LIKE specified more than once", showCommandName(statement))
			}
			if index+1 >= len(tokens) || !tokens[index+1].quoted {
				return nil, fmt.Errorf("f(SQL) show %s: LIKE pattern must be quoted", showCommandName(statement))
			}
			if tokens[index+1].value == "" {
				return nil, fmt.Errorf("f(SQL) show %s: LIKE pattern must not be empty", showCommandName(statement))
			}
			statement.HasLike = true
			statement.Like = tokens[index+1].value
			index++
		case "with":
			if seenWith {
				return nil, fmt.Errorf("f(SQL) show %s: WITH specified more than once", showCommandName(statement))
			}
			if index+1 >= len(tokens) {
				return nil, fmt.Errorf("f(SQL) show %s: missing option after WITH", showCommandName(statement))
			}
			if !strings.EqualFold(tokens[index+1].value, "all") && !strings.EqualFold(tokens[index+1].value, "hidden") {
				return nil, fmt.Errorf("f(SQL) show %s: unsupported WITH option %q", showCommandName(statement), tokens[index+1].value)
			}
			seenWith = true
			statement.All = true
			index++
		case "-a", "--all":
			if seenWith {
				return nil, fmt.Errorf("f(SQL) show %s: WITH specified more than once", showCommandName(statement))
			}
			seenWith = true
			statement.All = true
		default:
			if strings.HasPrefix(token.value, "-") {
				return nil, fmt.Errorf("f(SQL) unsupported show option %q", token.value)
			}
			if statement.Command == "" {
				statement.Command = strings.ToLower(token.value)
			} else {
				statement.Args = append(statement.Args, token.value)
			}
		}
	}
	if statement.Command == "" {
		return nil, fmt.Errorf("f(SQL) missing show command")
	}
	return statement, nil
}

func (statement *ShowStatement) ShowOptions() []ShowOption {
	options := make([]ShowOption, 0, 3)
	if statement.HasFrom {
		options = append(options, WithDatabase(statement.Database))
		if statement.User != "" {
			options = append(options, WithUser(statement.User))
		}
	}
	if statement.HasLike {
		options = append(options, WithLike(statement.Like))
	}
	return options
}

func parseShowScope(value string) (string, string, error) {
	parts := strings.Split(value, ".")
	if len(parts) > 2 || parts[0] == "" || (len(parts) == 2 && parts[1] == "") {
		return "", "", fmt.Errorf("f(SQL) invalid database scope %q", value)
	}
	database := strings.ToUpper(parts[0])
	if len(parts) == 1 {
		return database, "", nil
	}
	return database, strings.ToUpper(parts[1]), nil
}

func showCommandName(statement *ShowStatement) string {
	if statement.Command == "" {
		return ""
	}
	return statement.Command
}

func tokenizeShowStatement(text string) ([]showToken, error) {
	text = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(text), ";"))
	var tokens []showToken
	for index := 0; index < len(text); {
		for index < len(text) && (text[index] == ' ' || text[index] == '\t' || text[index] == '\n' || text[index] == '\r') {
			index++
		}
		if index == len(text) {
			break
		}
		if text[index] != '\'' && text[index] != '"' {
			start := index
			for index < len(text) && !strings.ContainsRune(" \t\n\r", rune(text[index])) {
				index++
			}
			tokens = append(tokens, showToken{value: text[start:index]})
			continue
		}

		quote := text[index]
		index++
		var value strings.Builder
		closed := false
		for index < len(text) {
			if text[index] != quote {
				value.WriteByte(text[index])
				index++
				continue
			}
			if index+1 < len(text) && text[index+1] == quote {
				value.WriteByte(quote)
				index += 2
				continue
			}
			index++
			closed = true
			break
		}
		if !closed {
			return nil, fmt.Errorf("f(SQL) unterminated quoted string in SHOW statement")
		}
		if index < len(text) && !strings.ContainsRune(" \t\n\r", rune(text[index])) {
			return nil, fmt.Errorf("f(SQL) invalid quoted string in SHOW statement")
		}
		tokens = append(tokens, showToken{value: value.String(), quoted: true})
	}
	return tokens, nil
}

type ServicePort struct {
	Service string
	Address string
}

type ResultSet interface {
	Columns() client.Columns
	Err() error
	Iter(func(values []interface{}) bool)
	Message() string
}

type ResultSetBase struct {
	err error
	msg string
}

func (rs *ResultSetBase) Err() error {
	return rs.err
}

func (rs *ResultSetBase) Message() string {
	if rs.err != nil {
		return rs.err.Error()
	}
	return rs.msg
}

var serverInfoProvider func() map[string]any

func SetServerInfoProvider(provider func() map[string]any) {
	serverInfoProvider = provider
}

type ShowInfoResultSet struct {
	ResultSetBase
	keys []string
	data map[string]any
}

var _ ResultSet = (*ShowInfoResultSet)(nil)

func (si *ShowInfoResultSet) Columns() client.Columns {
	return client.Columns{
		client.MakeColumnString("NAME"),
		client.MakeColumnAny("VALUE"),
	}
}

func (si *ShowInfoResultSet) Iter(callback func(values []interface{}) bool) {
	if si.err != nil {
		return
	}

	for _, k := range si.keys {
		v := si.data[k]
		if !callback([]interface{}{k, v}) {
			return
		}
	}
}

func ShowInfo() *ShowInfoResultSet {
	if serverInfoProvider == nil {
		return &ShowInfoResultSet{ResultSetBase: ResultSetBase{err: errors.New("server info provider is not set")}}
	}
	serverInfo := serverInfoProvider()
	keys := make([]string, 0, len(serverInfo))
	for k := range serverInfo {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return &ShowInfoResultSet{keys: keys, data: serverInfo}
}

type LicenseResultSet struct {
	ResultSetBase
	lic *LicenseInfo
}

var _ ResultSet = (*LicenseResultSet)(nil)

func (li *LicenseResultSet) Columns() client.Columns {
	return client.Columns{
		{Name: "ID", DataType: api.DataTypeString},
		{Name: "TYPE", DataType: api.DataTypeString},
		{Name: "CUSTOMER", DataType: api.DataTypeString},
		{Name: "PROJECT", DataType: api.DataTypeString},
		{Name: "COUNTRY_CODE", DataType: api.DataTypeString},
		{Name: "INSTALL_DATE", DataType: api.DataTypeString},
		{Name: "ISSUE_DATE", DataType: api.DataTypeString},
		{Name: "STATUS", DataType: api.DataTypeString},
	}
}

func (li *LicenseResultSet) Iter(callback func(values []interface{}) bool) {
	callback([]interface{}{
		li.lic.Id, li.lic.Type, li.lic.Customer, li.lic.Project, li.lic.CountryCode,
		li.lic.InstallDate, li.lic.IssueDate, strings.ToUpper(li.lic.LicenseStatus),
	})
}

func ShowLicense(ctx context.Context, conn *sql.Conn) *LicenseResultSet {
	licenseInfo, err := GetLicenseInfo(ctx, conn)
	return &LicenseResultSet{ResultSetBase: ResultSetBase{err: err}, lic: licenseInfo}
}

var serverPortsProvider func(string) ([]*ServicePort, error)

func SetServerPortsProvider(provider func(string) ([]*ServicePort, error)) {
	serverPortsProvider = provider
}

type ShowPortsResultSet struct {
	ResultSetBase
	data []*ServicePort
}

var _ ResultSet = (*ShowPortsResultSet)(nil)

func (si *ShowPortsResultSet) Columns() client.Columns {
	return client.Columns{
		client.MakeColumnString("PORT"),
		client.MakeColumnString("ADDRESS"),
	}
}

func (si *ShowPortsResultSet) Iter(callback func(values []interface{}) bool) {
	if si.err != nil {
		return
	}

	for _, sp := range si.data {
		if !callback([]interface{}{sp.Service, sp.Address}) {
			return
		}
	}
}

func ShowPorts(portType string) *ShowPortsResultSet {
	if serverPortsProvider == nil {
		return &ShowPortsResultSet{ResultSetBase: ResultSetBase{err: errors.New("server ports provider is not set")}}
	}
	serverInfo, err := serverPortsProvider(portType)
	if err != nil {
		return &ShowPortsResultSet{ResultSetBase: ResultSetBase{err: err}}
	}
	return &ShowPortsResultSet{data: serverInfo}
}

type ShowUsersResultSet struct {
	ResultSetBase
	data []*UserInfo
}

type UserInfo struct {
	UserId int64  `json:"user_id"`
	Name   string `json:"name"`
}

var _ ResultSet = (*ShowUsersResultSet)(nil)

func (si *ShowUsersResultSet) Columns() client.Columns {
	return client.Columns{
		client.MakeColumnInt64("USER_ID"),
		client.MakeColumnString("NAME"),
	}
}

func (si *ShowUsersResultSet) Iter(callback func(values []interface{}) bool) {
	if si.err != nil {
		return
	}

	for _, u := range si.data {
		if !callback([]interface{}{u.UserId, u.Name}) {
			return
		}
	}
}

func ShowUsers(ctx context.Context, conn *sql.Conn, opts ...ShowOption) *ShowUsersResultSet {
	options := newShowOptions(opts...)
	if err := options.validate(); err != nil {
		return &ShowUsersResultSet{ResultSetBase: ResultSetBase{err: err}}
	}
	args := []any{}
	sqlText := SqlTidy("SELECT USER_ID, NAME FROM M$SYS_USERS WHERE 1 = 1", options.likeClause("NAME", &args), "ORDER BY USER_ID")
	rows, err := conn.QueryContext(ctx, sqlText, args...)
	if err != nil {
		return &ShowUsersResultSet{ResultSetBase: ResultSetBase{err: err}}
	}
	defer rows.Close()

	var users []*UserInfo
	for rows.Next() {
		var u UserInfo
		if err := rows.Scan(&u.UserId, &u.Name); err != nil {
			return &ShowUsersResultSet{ResultSetBase: ResultSetBase{err: err}}
		}
		users = append(users, &u)
	}
	if err := rows.Err(); err != nil {
		return &ShowUsersResultSet{ResultSetBase: ResultSetBase{err: err}}
	}
	return &ShowUsersResultSet{data: users}
}

type ShowDatabasesResultSet struct {
	ResultSetBase
	list []*DatabaseInfo
}

type DatabaseInfo struct {
	DatabaseId       int64  `json:"database_id"`
	TablespaceId     int64  `json:"tablespace_id"`
	SourceDatabaseId int64  `json:"source_database_id"`
	Name             string `json:"name"`
	Kind             string `json:"kind"`
	AccessMode       string `json:"access_mode"`
	CanUse           int    `json:"can_use"`
	State            string `json:"state"`
	IsDefault        int    `json:"is_default"`
}

var _ ResultSet = (*ShowDatabasesResultSet)(nil)

func (di *ShowDatabasesResultSet) Columns() client.Columns {
	return client.Columns{
		{Name: "DATABASE_ID", DataType: api.DataTypeInt64},
		{Name: "NAME", DataType: api.DataTypeString},
		{Name: "KIND", DataType: api.DataTypeString},
		{Name: "ACCESS_MODE", DataType: api.DataTypeString},
		{Name: "CAN_USE", DataType: api.DataTypeInt32},
		{Name: "STATE", DataType: api.DataTypeInt32},
		{Name: "IS_DEFAULT", DataType: api.DataTypeInt32},
	}
}

func (di *ShowDatabasesResultSet) Iter(callback func(values []interface{}) bool) {
	for _, d := range di.list {
		if !callback([]interface{}{d.DatabaseId, d.Name, d.Kind, d.AccessMode, d.CanUse, d.State, d.IsDefault}) {
			return
		}
	}
}

func ShowDatabases(ctx context.Context, conn *sql.Conn, opts ...ShowOption) *ShowDatabasesResultSet {
	options := newShowOptions(opts...)
	if err := options.validate(); err != nil {
		return &ShowDatabasesResultSet{ResultSetBase: ResultSetBase{err: err}}
	}
	args := []any{}
	sqlText := SqlTidy(`SELECT
		DATABASE_ID,
		TABLESPACE_ID,
		SOURCE_DATABASE_ID,
		NAME,
		KIND,
		ACCESS_MODE,
		CAN_USE,
		STATE,
		IS_DEFAULT
	FROM
		V$DATABASES
	WHERE 1 = 1`, options.likeClause("NAME", &args), `ORDER BY DATABASE_ID`)
	rows, err := conn.QueryContext(ctx, sqlText, args...)
	if err != nil {
		return &ShowDatabasesResultSet{ResultSetBase: ResultSetBase{err: err}}
	}
	defer rows.Close()

	var databases []*DatabaseInfo
	for rows.Next() {
		var d DatabaseInfo
		if err := rows.Scan(&d.DatabaseId, &d.TablespaceId, &d.SourceDatabaseId, &d.Name, &d.Kind, &d.AccessMode, &d.CanUse, &d.State, &d.IsDefault); err != nil {
			return &ShowDatabasesResultSet{ResultSetBase: ResultSetBase{err: err}}
		}
		databases = append(databases, &d)
	}
	if err := rows.Err(); err != nil {
		return &ShowDatabasesResultSet{ResultSetBase: ResultSetBase{err: err}}
	}
	return &ShowDatabasesResultSet{list: databases}
}

type ShowTablesResultSet struct {
	ResultSetBase
	list []*TableInfo
}

var _ ResultSet = (*ShowTablesResultSet)(nil)

func (ti *ShowTablesResultSet) Columns() client.Columns {
	return client.Columns{
		client.MakeColumnString("DATABASE_NAME"),
		client.MakeColumnString("USER_NAME"),
		client.MakeColumnString("TABLE_NAME"),
		client.MakeColumnInt64("TABLE_ID"),
		client.MakeColumnString("TABLE_TYPE"),
		client.MakeColumnString("TABLE_FLAG"),
	}
}

func (ti *ShowTablesResultSet) Iter(callback func(values []interface{}) bool) {
	for _, t := range ti.list {
		if !callback([]interface{}{t.Database, t.User, t.Name, t.Id, t.Type.ShortString(), t.Flag.String()}) {
			return
		}
	}
}

func ShowTables(ctx context.Context, conn *sql.Conn, showAll bool, opts ...ShowOption) *ShowTablesResultSet {
	var list = []*TableInfo{}
	err := ListTablesWalk(ctx, conn, showAll, func(t *TableInfo, err error) bool {
		if err != nil {
			return false
		}
		list = append(list, t)
		return true
	}, opts...)
	return &ShowTablesResultSet{ResultSetBase: ResultSetBase{err: err}, list: list}
}

type ShowTableResultSet struct {
	ResultSetBase
	Description *TableDescription
}

var _ ResultSet = (*ShowTableResultSet)(nil)

func (tr *ShowTableResultSet) Err() error {
	return tr.err
}

func (tr *ShowTableResultSet) Message() string {
	if tr.err != nil {
		return tr.err.Error()
	}
	return ""
}

func (tr *ShowTableResultSet) Columns() client.Columns {
	return client.Columns{
		client.MakeColumnString("COLUMN"),
		client.MakeColumnString("TYPE"),
		client.MakeColumnInt32("LENGTH"),
		client.MakeColumnString("FLAG"),
		client.MakeColumnString("INDEX"),
	}
}

func (tr *ShowTableResultSet) Iter(callback func(values []interface{}) bool) {
	for _, col := range tr.Description.Columns {
		indexes := []string{}
		for _, idxDesc := range tr.Description.Indexes {
			for _, colName := range idxDesc.Cols {
				if colName == col.Name {
					indexes = append(indexes, idxDesc.Name)
					break
				}
			}
		}
		values := []any{
			col.Name, col.Type.String(), col.Width(), col.Flag.String(), strings.Join(indexes, ","),
		}
		if !callback(values) {
			return
		}
	}
}

func ShowTable(ctx context.Context, conn *sql.Conn, fallbackDatabaseName string, fallbackUserName string, tableName string, all bool, opts ...ShowOption) *ShowTableResultSet {
	options := newShowOptions(opts...)
	if err := resolveShowScope(ctx, conn, &options); err != nil {
		return &ShowTableResultSet{ResultSetBase: ResultSetBase{err: err}}
	}
	if fallbackDatabaseName == "" {
		fallbackDatabaseName = options.database
	}
	if fallbackUserName == "" {
		fallbackUserName = options.user
	}
	databaseName := fallbackDatabaseName
	userName := fallbackUserName
	parts := strings.SplitN(strings.ToUpper(tableName), ".", 3)
	if len(parts) == 3 {
		databaseName = parts[0]
		userName = parts[1]
		tableName = parts[2]
	} else if len(parts) == 2 {
		userName = parts[0]
		tableName = parts[1]
	}

	tableName = fmt.Sprintf("%s.%s.%s", databaseName, userName, tableName)
	desc, err := DescribeTable(ctx, conn, tableName, all)
	return &ShowTableResultSet{ResultSetBase: ResultSetBase{err: err}, Description: desc}
}

type ShowMetaTablesResultSet struct {
	ResultSetBase
	list []*TableInfo
}

type MetaTableInfo struct {
	Id   int64            `json:"id"`
	Name string           `json:"name"`
	Type client.TableType `json:"type"`
}

var _ ResultSet = (*ShowMetaTablesResultSet)(nil)

func (ti *ShowMetaTablesResultSet) Columns() client.Columns {
	return client.Columns{
		client.MakeColumnInt64("ID"),
		client.MakeColumnString("NAME"),
		client.MakeColumnString("TYPE"),
	}
}

func (ti *ShowMetaTablesResultSet) Iter(callback func(values []interface{}) bool) {
	for _, t := range ti.list {
		if !callback([]interface{}{t.Id, t.Name, t.Type.ShortString()}) {
			return
		}
	}
}

func ShowMetaTables(ctx context.Context, conn *sql.Conn, opts ...ShowOption) *ShowMetaTablesResultSet {
	var list = []*TableInfo{}
	options := newShowOptions(opts...)
	if err := options.validate(); err != nil {
		return &ShowMetaTablesResultSet{ResultSetBase: ResultSetBase{err: err}}
	}
	args := []any{}
	sqlText := SqlTidy("SELECT ID, NAME, TYPE FROM M$TABLES WHERE 1 = 1", options.likeClause("NAME", &args), "ORDER BY ID")
	rows, err := conn.QueryContext(ctx, sqlText, args...)
	if err != nil {
		return &ShowMetaTablesResultSet{ResultSetBase: ResultSetBase{err: err}}
	}
	for rows.Next() {
		var t TableInfo
		if err = rows.Scan(&t.Id, &t.Name, &t.Type); err != nil {
			return &ShowMetaTablesResultSet{ResultSetBase: ResultSetBase{err: err}}
		}
		list = append(list, &t)
	}
	return &ShowMetaTablesResultSet{list: list}
}

type ShowVirtualTablesResultSet struct {
	ResultSetBase
	list []*TableInfo
}

type VirtualTableInfo struct {
	Id   int64            `json:"id"`
	Name string           `json:"name"`
	Type client.TableType `json:"type"`
}

var _ ResultSet = (*ShowVirtualTablesResultSet)(nil)

func (ti *ShowVirtualTablesResultSet) Columns() client.Columns {
	return client.Columns{
		{Name: "ID", DataType: api.DataTypeInt64},
		{Name: "NAME", DataType: api.DataTypeString},
		{Name: "TYPE", DataType: api.DataTypeString},
	}
}

func (ti *ShowVirtualTablesResultSet) Iter(callback func(values []interface{}) bool) {
	for _, t := range ti.list {
		if !callback([]interface{}{t.Id, t.Name, t.Type.ShortString()}) {
			return
		}
	}
}

func ShowVirtualTables(ctx context.Context, conn *sql.Conn, opts ...ShowOption) *ShowVirtualTablesResultSet {
	var list = []*TableInfo{}
	options := newShowOptions(opts...)
	if err := options.validate(); err != nil {
		return &ShowVirtualTablesResultSet{ResultSetBase: ResultSetBase{err: err}}
	}
	args := []any{}
	sqlText := SqlTidy("SELECT ID, NAME, TYPE FROM V$TABLES WHERE 1 = 1", options.likeClause("NAME", &args), "ORDER BY ID")
	rows, err := conn.QueryContext(ctx, sqlText, args...)
	if err != nil {
		return &ShowVirtualTablesResultSet{ResultSetBase: ResultSetBase{err: err}}
	}
	for rows.Next() {
		var t TableInfo
		if err = rows.Scan(&t.Id, &t.Name, &t.Type); err != nil {
			return &ShowVirtualTablesResultSet{ResultSetBase: ResultSetBase{err: err}}
		}
		list = append(list, &t)
	}
	return &ShowVirtualTablesResultSet{list: list}
}

type ShowSessionsResultSet struct {
	ResultSetBase
	rows [][]any
}

var _ ResultSet = (*ShowSessionsResultSet)(nil)

func (sri *ShowSessionsResultSet) Columns() client.Columns {
	return client.Columns{
		{Name: "ID", DataType: api.DataTypeInt64},
		{Name: "USER_NAME", DataType: api.DataTypeString},
		{Name: "USER_ID", DataType: api.DataTypeInt64},
		{Name: "LOGIN_TIME", DataType: api.DataTypeDatetime},
		{Name: "TYPE", DataType: api.DataTypeString},
		{Name: "USER_IP", DataType: api.DataTypeString},
		{Name: "MAX_QPX_MEM", DataType: api.DataTypeInt64},
	}
}

func (sri *ShowSessionsResultSet) Iter(callback func(values []interface{}) bool) {
	for _, row := range sri.rows {
		if !callback(row) {
			return
		}
	}
}

func ShowSessions(ctx context.Context, conn *sql.Conn, opts ...ShowOption) *ShowSessionsResultSet {
	ret := &ShowSessionsResultSet{}
	options := newShowOptions(opts...)
	if err := options.validate(); err != nil {
		ret.err = err
		return ret
	}
	func() {
		args := []any{}
		sqlText := SqlTidy("SELECT ID, USER_ID, LOGIN_TIME, CLIENT_TYPE, USER_NAME, USER_IP, MAX_QPX_MEM FROM V$SESSION WHERE 1 = 1", options.likeClause("USER_NAME", &args))
		rows, err := conn.QueryContext(ctx, sqlText, args...)
		if err != nil {
			ret.err = err
			return
		}
		defer rows.Close()
		for rows.Next() {
			var id int64
			var userId int64
			var loginTime time.Time
			var clientType string
			var userName string
			var userIp string
			var maxQpxMem int64
			if err := rows.Scan(&id, &userId, &loginTime, &clientType, &userName, &userIp, &maxQpxMem); err != nil {
				ret.err = err
				return
			}
			row := []any{id, userName, userId, loginTime, clientType, userIp, util.HumanizeByteCount(maxQpxMem)}
			ret.rows = append(ret.rows, row)
		}
		if err := rows.Err(); err != nil {
			ret.err = err
			return
		}
	}()
	if ret.err != nil {
		return ret
	}
	func() {
		args := []any{}
		sqlText := SqlTidy("SELECT ID, USER_ID, USER_NAME FROM V$NEO_SESSION WHERE 1 = 1", options.likeClause("USER_NAME", &args))
		rows, err := conn.QueryContext(ctx, sqlText, args...)
		if err != nil {
			ret.err = err
			return
		}
		defer rows.Close()

		for rows.Next() {
			var id int64
			var userId int64
			var userName string
			if err := rows.Scan(&id, &userId, &userName); err != nil {
				ret.err = err
				return
			}
			row := []any{id, userName, userId, nil, "neo", nil, nil}
			ret.rows = append(ret.rows, row)
		}
		if err := rows.Err(); err != nil {
			ret.err = err
			return
		}
	}()
	return ret
}

type ShowStatementsResultSet struct {
	ResultSetBase
	list []*StatementInfo
}

type StatementInfo struct {
	ID         int64  `json:"id"`
	SessionID  int64  `json:"session_id"`
	State      string `json:"state"`
	Query      string `json:"query"`
	RecordSize int64  `json:"record_size"`
}

func (si *StatementInfo) Values() []interface{} {
	var recordSize any
	recordSize = si.RecordSize
	return []interface{}{
		si.ID, si.SessionID, si.State, recordSize, si.Query,
	}
}

var _ ResultSet = (*ShowStatementsResultSet)(nil)

func (sri *ShowStatementsResultSet) Columns() client.Columns {
	return client.Columns{
		{Name: "ID", DataType: api.DataTypeInt64},
		{Name: "SESSION_ID", DataType: api.DataTypeInt64},
		{Name: "STATE", DataType: api.DataTypeString},
		{Name: "RECORD_SIZE", DataType: api.DataTypeInt64},
		{Name: "QUERY", DataType: api.DataTypeString},
	}
}

func (sri *ShowStatementsResultSet) Iter(callback func(values []interface{}) bool) {
	for _, s := range sri.list {
		if !callback(s.Values()) {
			return
		}
	}
}

func ShowStatements(ctx context.Context, conn *sql.Conn, opts ...ShowOption) *ShowStatementsResultSet {
	options := newShowOptions(opts...)
	if err := options.validate(); err != nil {
		return &ShowStatementsResultSet{ResultSetBase: ResultSetBase{err: err}}
	}
	args := []any{}
	sqlText := SqlTidy("SELECT ID, SESS_ID, STATE, RECORD_SIZE, QUERY FROM V$STMT WHERE 1 = 1", options.likeClause("QUERY", &args))
	stmtRows, err := conn.QueryContext(ctx, sqlText, args...)
	if err != nil {
		return &ShowStatementsResultSet{ResultSetBase: ResultSetBase{err: err}}
	}
	defer stmtRows.Close()

	list := []*StatementInfo{}
	for stmtRows.Next() {
		rec := &StatementInfo{}
		err = stmtRows.Scan(&rec.ID, &rec.SessionID, &rec.State, &rec.RecordSize, &rec.Query)
		if err != nil {
			return &ShowStatementsResultSet{ResultSetBase: ResultSetBase{err: err}}
		}
		list = append(list, rec)
	}
	return &ShowStatementsResultSet{list: list}
}

type ShowIndexesResultSet struct {
	ResultSetBase
	list []*IndexInfo
}

type IndexInfo struct {
	Id             int64    `json:"id"`
	DatabaseName   string   `json:"database_name"`
	DatabaseId     int64    `json:"database_id"`
	User           string   `json:"user"`
	TableName      string   `json:"table_name"`
	ColumnNames    []string `json:"column_names"`
	IndexName      string   `json:"index_name"`
	IndexType      string   `json:"index_type"`
	KeyCompress    string   `json:"key_compress"`
	MaxLevel       int64    `json:"max_level"`
	PartValueCount int64    `json:"part_value_count"`
	BitMapEncode   string   `json:"bitmap_encode"`
}

var _ ResultSet = (*ShowIndexesResultSet)(nil)

func (ii *ShowIndexesResultSet) Columns() client.Columns {
	return client.Columns{
		{Name: "ID", DataType: api.DataTypeInt64},
		{Name: "DATABASE", DataType: api.DataTypeString},
		{Name: "USER", DataType: api.DataTypeString},
		{Name: "TABLE", DataType: api.DataTypeString},
		{Name: "COLUMN", DataType: api.DataTypeString},
		{Name: "INDEX_NAME", DataType: api.DataTypeString},
		{Name: "INDEX_TYPE", DataType: api.DataTypeString},
		{Name: "KEY_COMPRESS", DataType: api.DataTypeString},
		{Name: "MAX_LEVEL", DataType: api.DataTypeInt64},
		{Name: "PART_VALUE_COUNT", DataType: api.DataTypeInt64},
		{Name: "BITMAP_ENCODE", DataType: api.DataTypeString},
	}
}

func (ii *ShowIndexesResultSet) Iter(callback func(values []interface{}) bool) {
	for _, idx := range ii.list {
		cont := callback([]interface{}{
			idx.Id, idx.DatabaseName, idx.User, idx.TableName, strings.Join(idx.ColumnNames, ","), idx.IndexName,
			idx.IndexType, idx.KeyCompress, idx.MaxLevel, idx.PartValueCount, idx.BitMapEncode,
		})
		if !cont {
			return
		}
	}
}

func ShowIndexes(ctx context.Context, conn *sql.Conn, opts ...ShowOption) *ShowIndexesResultSet {
	options := newShowOptions(opts...)
	if err := resolveShowScope(ctx, conn, &options); err != nil {
		return &ShowIndexesResultSet{ResultSetBase: ResultSetBase{err: err}}
	}
	return showIndexes(ctx, conn, "", &options)
}

func showIndexes(ctx context.Context, conn *sql.Conn, indexName string, options *showOptions) *ShowIndexesResultSet {
	args := []any{}
	indexClause := func() string {
		if indexName == "" {
			return ""
		}
		args = append(args, indexName)
		return "AND b.name = ?"
	}
	var listIndexesSql = SqlTidy(`
		SELECT
			j.DB_NAME as DATABASE_NAME,
			j.DATABASE_ID as DATABASE_ID,
			u.name as USER_NAME,
			j.TABLE_NAME as TABLE_NAME,
			b.name as INDEX_NAME,
			b.id as INDEX_ID,
			case b.type
				when 1 then 'BITMAP'
				when 2 then 'KEYWORD'
				when 5 then 'REDBLACK'
				when 6 then 'LSM'
				when 8 then 'REDBLACK'
				when 9 then 'KEYWORD_LSM'
				when 11 then 'TAG'
				else 'LSM' 
			end as INDEX_TYPE,
			case b.key_compress
				when 0 then 'UNCOMPRESSED'
				else 'COMPRESSED'
			end as KEY_COMPRESS,
			b.max_level as MAX_LEVEL,
			b.part_value_count as PART_VALUE_COUNT,
			case b.bitmap_encode
				when 0 then 'EQUAL'
				else 'RANGE'
			end as BITMAP_ENCODE
		FROM
			m$sys_indexes b, 
			m$sys_users u,
			(
				select
					a.DATABASE_NAME DB_NAME,
					a.DATABASE_ID as DATABASE_ID,
					a.name as TABLE_NAME,
					a.id as TABLE_ID,
					a.USER_ID as USER_ID
				from
					M$SYS_TABLES a
				where
					1 = 1`,
		options.databaseClause("a.DATABASE_NAME", &args), `
			) as j
		WHERE
			j.TABLE_ID = b.TABLE_ID
		AND j.DATABASE_ID = b.DATABASE_ID
		AND j.USER_ID = u.USER_ID`,
		options.userClause("u.NAME", &args),
		indexClause(),
		options.likeClause("b.NAME", &args), `
		ORDER BY
			j.DATABASE_ID, u.USER_ID, j.TABLE_NAME, b.ID
	`)

	rows, err := conn.QueryContext(ctx, listIndexesSql, args...)
	if err != nil {
		return &ShowIndexesResultSet{ResultSetBase: ResultSetBase{err: err}}
	}
	defer rows.Close()

	list := []*IndexInfo{}
	for rows.Next() {
		nfo := &IndexInfo{}
		err = rows.Scan(
			&nfo.DatabaseName, &nfo.DatabaseId, &nfo.User, &nfo.TableName, &nfo.IndexName,
			&nfo.Id, &nfo.IndexType, &nfo.KeyCompress,
			&nfo.MaxLevel, &nfo.PartValueCount, &nfo.BitMapEncode)
		if err != nil {
			return &ShowIndexesResultSet{ResultSetBase: ResultSetBase{err: err}}
		}
		rowsCols, err := conn.QueryContext(ctx, `select name from M$SYS_INDEX_COLUMNS where index_id = ? AND database_id = ? order by col_id`, nfo.Id, nfo.DatabaseId)
		if err != nil {
			return &ShowIndexesResultSet{ResultSetBase: ResultSetBase{err: err}}
		}
		for rowsCols.Next() {
			var col string
			if err = rowsCols.Scan(&col); err != nil {
				rowsCols.Close()
				return &ShowIndexesResultSet{ResultSetBase: ResultSetBase{err: err}}
			}
			nfo.ColumnNames = append(nfo.ColumnNames, col)
		}
		rowsCols.Close()
		list = append(list, nfo)
	}
	err = rows.Err()
	return &ShowIndexesResultSet{ResultSetBase: ResultSetBase{err: err}, list: list}
}

type ShowIndexResultSet struct {
	ResultSetBase
	desc *IndexInfo
}

var _ ResultSet = (*ShowIndexResultSet)(nil)

func (qir *ShowIndexResultSet) Columns() client.Columns {
	return client.Columns{
		{Name: "ID", DataType: api.DataTypeInt64},
		{Name: "DATABASE", DataType: api.DataTypeString},
		{Name: "USER", DataType: api.DataTypeString},
		{Name: "TABLE", DataType: api.DataTypeString},
		{Name: "COLUMN", DataType: api.DataTypeString},
		{Name: "INDEX_NAME", DataType: api.DataTypeString},
		{Name: "INDEX_TYPE", DataType: api.DataTypeString},
		{Name: "KEY_COMPRESS", DataType: api.DataTypeString},
		{Name: "MAX_LEVEL", DataType: api.DataTypeInt64},
		{Name: "PART_VALUE_COUNT", DataType: api.DataTypeInt64},
		{Name: "BITMAP_ENCODE", DataType: api.DataTypeString},
	}
}

func (qir *ShowIndexResultSet) Iter(callback func(values []interface{}) bool) {
	if qir.desc == nil {
		return
	}
	cont := callback([]interface{}{
		qir.desc.Id,
		qir.desc.DatabaseName,
		qir.desc.User,
		qir.desc.TableName,
		strings.Join(qir.desc.ColumnNames, ","),
		qir.desc.IndexName,
		qir.desc.IndexType,
		qir.desc.KeyCompress,
		qir.desc.MaxLevel,
		qir.desc.PartValueCount,
		qir.desc.BitMapEncode,
	})
	if !cont {
		return
	}
}

func ShowIndex(ctx context.Context, conn *sql.Conn, indexName string, opts ...ShowOption) *ShowIndexResultSet {
	options := newShowOptions(opts...)
	if err := resolveShowScope(ctx, conn, &options); err != nil {
		return &ShowIndexResultSet{ResultSetBase: ResultSetBase{err: err}}
	}
	r := showIndexes(ctx, conn, indexName, &options)
	if r.err != nil {
		return &ShowIndexResultSet{ResultSetBase: ResultSetBase{err: r.err}}
	}
	if len(r.list) == 0 {
		return &ShowIndexResultSet{ResultSetBase: ResultSetBase{err: fmt.Errorf("index %s not found", indexName)}}
	}
	return &ShowIndexResultSet{desc: r.list[0]}
}

type ShowStorageResultSet struct {
	ResultSetBase
	list []*StorageInfo
}

type StorageInfo struct {
	DatabaseName string `json:"database_name"`
	DatabaseId   int64  `json:"database_id"`
	TableName    string `json:"table_name"`
	TableId      int64  `json:"table_id"`
	DataSize     int64  `json:"data_size"`
	IndexSize    int64  `json:"index_size"`
	TotalSize    int64  `json:"total_size"`
}

var _ ResultSet = (*ShowStorageResultSet)(nil)

func (sui *ShowStorageResultSet) Columns() client.Columns {
	return client.Columns{
		{Name: "DATABASE_NAME", DataType: api.DataTypeString},
		{Name: "TABLE_NAME", DataType: api.DataTypeString},
		{Name: "DATA_SIZE", DataType: api.DataTypeInt64},
		{Name: "INDEX_SIZE", DataType: api.DataTypeInt64},
		{Name: "TOTAL_SIZE", DataType: api.DataTypeInt64},
	}
}

func (sui *ShowStorageResultSet) Iter(callback func(values []interface{}) bool) {
	for _, t := range sui.list {
		if !callback([]interface{}{t.DatabaseName, t.TableName, t.DataSize, t.IndexSize, t.TotalSize}) {
			return
		}
	}
}

func ShowStorage(ctx context.Context, conn *sql.Conn, opts ...ShowOption) *ShowStorageResultSet {
	options := newShowOptions(opts...)
	if err := resolveShowScope(ctx, conn, &options); err != nil {
		return &ShowStorageResultSet{ResultSetBase: ResultSetBase{err: err}}
	}
	args := []any{}
	sqlText := SqlTidy(`
		SELECT
			a.database_id as DATABASE_ID,
			a.database_name as DATABASE_NAME,
			a.table_id as TABLE_ID,
			a.table_name as TABLE_NAME,
			a.data_size as DATA_SIZE,
			case b.index_size when b.index_size then b.index_size else 0
			end as INDEX_SIZE,
			case a.data_size + b.index_size
				when a.data_size + b.index_size then a.data_size + b.index_size
				else a.data_size
			end as TOTAL_SIZE
		FROM (
			select
				a.database_id as database_id,
				a.database_name as database_name,
				a.id as table_id,
				a.name as table_name,
				sum(b.storage_usage) as data_size
			from m$sys_tables a, v$storage_tables b, m$sys_users u
			where a.id = b.id
			and a.tablespace_id = b.tablespace_id
			and a.user_id = u.user_id`,
		options.databaseClause("a.database_name", &args),
		options.userClause("u.name", &args),
		options.likeClause("a.name", &args), `
			group by a.database_id, a.database_name, a.id, a.name
		) as a
		left outer join (
			select a.database_id, a.id as table_id, sum(b.disk_file_size) as index_size
			from m$sys_tables a, v$storage_dc_table_indexes b, m$sys_users u
			where a.id = b.table_id
			and a.tablespace_id = b.tablespace_id
			and a.user_id = u.user_id`,
		options.databaseClause("a.database_name", &args),
		options.userClause("u.name", &args),
		options.likeClause("a.name", &args), `
			group by a.database_id, a.id
		) as b
		on a.database_id = b.database_id and a.table_id = b.table_id
		order by a.database_name, a.table_name`)

	rows, err := conn.QueryContext(ctx, sqlText, args...)
	if err != nil {
		return &ShowStorageResultSet{ResultSetBase: ResultSetBase{err: err}}
	}
	defer rows.Close()

	list := []*StorageInfo{}
	for rows.Next() {
		rec := &StorageInfo{}
		err = rows.Scan(&rec.DatabaseId, &rec.DatabaseName, &rec.TableId, &rec.TableName, &rec.DataSize, &rec.IndexSize, &rec.TotalSize)
		if err != nil {
			return &ShowStorageResultSet{ResultSetBase: ResultSetBase{err: err}}
		}
		list = append(list, rec)
	}
	err = rows.Err()
	return &ShowStorageResultSet{ResultSetBase: ResultSetBase{err: err}, list: list}
}

type ShowTableUsageResultSet struct {
	ResultSetBase
	list []*TableUsageInfo
}

type TableUsageInfo struct {
	DatabaseName string `json:"database_name"`
	UserName     string `json:"user_name"`
	TableName    string `json:"table_name"`
	StorageUsage int64  `json:"storage_usage"`
}

var _ ResultSet = (*ShowTableUsageResultSet)(nil)

func (tui *ShowTableUsageResultSet) Columns() client.Columns {
	return client.Columns{
		{Name: "DATABASE", DataType: api.DataTypeString},
		{Name: "USER", DataType: api.DataTypeString},
		{Name: "TABLE", DataType: api.DataTypeString},
		{Name: "STORAGE_USAGE", DataType: api.DataTypeInt64},
	}
}

func (tui *ShowTableUsageResultSet) Iter(callback func(values []interface{}) bool) {
	for _, t := range tui.list {
		if !callback([]interface{}{t.DatabaseName, t.UserName, t.TableName, t.StorageUsage}) {
			return
		}
	}
}

func ShowTableUsage(ctx context.Context, conn *sql.Conn, opts ...ShowOption) *ShowTableUsageResultSet {
	options := newShowOptions(opts...)
	if err := resolveShowScope(ctx, conn, &options); err != nil {
		return &ShowTableUsageResultSet{ResultSetBase: ResultSetBase{err: err}}
	}
	args := []any{}
	sqlText := SqlTidy(`
		SELECT
			a.DATABASE_NAME as DATABASE_NAME,
			u.NAME as USER_NAME,
			a.NAME as TABLE_NAME,
			s.STORAGE_USAGE
		FROM
			M$SYS_USERS u,
			V$STORAGE_TABLES s,
			M$SYS_TABLES a
		WHERE
			u.USER_ID = a.USER_ID
		AND s.ID = a.ID
		AND s.TABLESPACE_ID = a.TABLESPACE_ID`,
		options.databaseClause("a.DATABASE_NAME", &args),
		options.userClause("u.NAME", &args),
		options.likeClause("a.NAME", &args), `
		ORDER BY a.DATABASE_ID, u.USER_ID, s.ID
	`)

	rows, err := conn.QueryContext(ctx, sqlText, args...)
	if err != nil {
		return &ShowTableUsageResultSet{ResultSetBase: ResultSetBase{err: err}}
	}
	defer rows.Close()

	list := []*TableUsageInfo{}
	for rows.Next() {
		rec := &TableUsageInfo{}
		err = rows.Scan(&rec.DatabaseName, &rec.UserName, &rec.TableName, &rec.StorageUsage)
		if err != nil {
			return &ShowTableUsageResultSet{ResultSetBase: ResultSetBase{err: err}}
		}
		list = append(list, rec)
	}
	err = rows.Err()
	return &ShowTableUsageResultSet{ResultSetBase: ResultSetBase{err: err}, list: list}
}

type ShowLsmResultSet struct {
	ResultSetBase
	list []*LsmIndexInfo
}

type LsmIndexInfo struct {
	TableName string `json:"table_name"`
	IndexName string `json:"index_name"`
	Level     int64  `json:"level"`
	Count     int64  `json:"count"`
}

var _ ResultSet = (*ShowLsmResultSet)(nil)

func (li *ShowLsmResultSet) Columns() client.Columns {
	return client.Columns{
		{Name: "TABLE_NAME", DataType: api.DataTypeString},
		{Name: "INDEX_NAME", DataType: api.DataTypeString},
		{Name: "LEVEL", DataType: api.DataTypeInt64},
		{Name: "COUNT", DataType: api.DataTypeInt64},
	}
}

func (li *ShowLsmResultSet) Iter(callback func(values []interface{}) bool) {
	for _, idx := range li.list {
		cont := callback([]interface{}{
			idx.TableName, idx.IndexName, idx.Level, idx.Count,
		})
		if !cont {
			return
		}
	}
}

func ShowLsm(ctx context.Context, conn *sql.Conn, opts ...ShowOption) *ShowLsmResultSet {
	options := newShowOptions(opts...)
	if err := resolveShowScope(ctx, conn, &options); err != nil {
		return &ShowLsmResultSet{ResultSetBase: ResultSetBase{err: err}}
	}
	args := []any{}
	sqlText := SqlTidy(`select
		b.name as TABLE_NAME,
		c.name as INDEX_NAME,
		a.level as LEVEL,
		a.end_rid - a.begin_rid as COUNT
	from
		v$storage_dc_lsmindex_levels a,
		m$sys_tables b, m$sys_indexes c, m$sys_users u
	where
		c.id = a.index_id
	and c.tablespace_id = a.tablespace_id
	and b.id = a.table_id
	and b.database_id = c.database_id
	and b.user_id = u.user_id`, options.databaseClause("b.database_name", &args), options.userClause("u.name", &args), options.likeClause("b.name", &args), `order by 1, 2, 3`)
	rows, err := conn.QueryContext(ctx, sqlText, args...)
	if err != nil {
		return &ShowLsmResultSet{ResultSetBase: ResultSetBase{err: err}}
	}
	defer rows.Close()
	var list []*LsmIndexInfo
	for rows.Next() {
		rec := &LsmIndexInfo{}
		err = rows.Scan(&rec.TableName, &rec.IndexName, &rec.Level, &rec.Count)
		if err != nil {
			return &ShowLsmResultSet{ResultSetBase: ResultSetBase{err: err}}
		}
		list = append(list, rec)
	}
	err = rows.Err()
	return &ShowLsmResultSet{ResultSetBase: ResultSetBase{err: err}, list: list}
}

type ShowIndexGapResultSet struct {
	ResultSetBase
	list []*IndexGapInfo
}

type IndexGapInfo struct {
	ID        int64  `json:"id"`
	TableName string `json:"table_name"`
	IndexName string `json:"index_name"`
	Gap       int64  `json:"gap"`
}

var _ ResultSet = (*ShowIndexGapResultSet)(nil)

func (igi *ShowIndexGapResultSet) Columns() client.Columns {
	return client.Columns{
		{Name: "INDEX_ID", DataType: api.DataTypeInt64},
		{Name: "TABLE_NAME", DataType: api.DataTypeString},
		{Name: "INDEX_NAME", DataType: api.DataTypeString},
		{Name: "GAP", DataType: api.DataTypeInt64},
	}
}

func (igi *ShowIndexGapResultSet) Iter(callback func(values []interface{}) bool) {
	for _, idx := range igi.list {
		cont := callback([]interface{}{
			idx.ID, idx.TableName, idx.IndexName, idx.Gap,
		})
		if !cont {
			return
		}
	}
}

func ShowIndexGap(ctx context.Context, conn *sql.Conn, opts ...ShowOption) *ShowIndexGapResultSet {
	options := newShowOptions(opts...)
	if err := resolveShowScope(ctx, conn, &options); err != nil {
		return &ShowIndexGapResultSet{ResultSetBase: ResultSetBase{err: err}}
	}
	args := []any{}
	sqlText := SqlTidy(`select
		c.id,
		b.name as TABLE_NAME, 
		c.name as INDEX_NAME, 
		a.table_end_rid - a.end_rid as GAP
	from
		v$storage_dc_table_indexes a,
		m$sys_tables b,
		m$sys_indexes c,
		m$sys_users u
	where
		a.id = c.id
	and a.tablespace_id = c.tablespace_id
	and c.table_id = b.id
	and c.database_id = b.database_id
	and b.user_id = u.user_id`, options.databaseClause("b.database_name", &args), options.userClause("u.name", &args), options.likeClause("b.name", &args), `order by 3 desc, 1, 2`)

	rows, err := conn.QueryContext(ctx, sqlText, args...)
	if err != nil {
		return &ShowIndexGapResultSet{ResultSetBase: ResultSetBase{err: err}}
	}
	defer rows.Close()

	list := []*IndexGapInfo{}
	for rows.Next() {
		rec := &IndexGapInfo{}
		err = rows.Scan(&rec.ID, &rec.TableName, &rec.IndexName, &rec.Gap)
		if err != nil {
			return &ShowIndexGapResultSet{ResultSetBase: ResultSetBase{err: err}}
		}
		list = append(list, rec)
	}
	err = rows.Err()
	return &ShowIndexGapResultSet{ResultSetBase: ResultSetBase{err: err}, list: list}
}

type TagIndexGapResultSet struct {
	ResultSetBase
	list []*TagIndexGapInfo
}

type TagIndexGapInfo struct {
	TableId   int64  `json:"id"`
	TableName string `json:"table_name"`
	Status    string `json:"status"`
	DiskGap   int64  `json:"disk_gap"`
	MemoryGap int64  `json:"memory_gap"`
}

var _ ResultSet = (*TagIndexGapResultSet)(nil)

func (tigi *TagIndexGapResultSet) Columns() client.Columns {
	return client.Columns{
		{Name: "TABLE_ID", DataType: api.DataTypeInt64},
		{Name: "TABLE_NAME", DataType: api.DataTypeString},
		{Name: "STATUS", DataType: api.DataTypeString},
		{Name: "DISK_GAP", DataType: api.DataTypeInt64},
		{Name: "MEMORY_GAP", DataType: api.DataTypeInt64},
	}
}

func (tigi *TagIndexGapResultSet) Iter(callback func(values []interface{}) bool) {
	for _, idx := range tigi.list {
		cont := callback([]interface{}{
			idx.TableId, idx.TableName, idx.Status, idx.DiskGap, idx.MemoryGap,
		})
		if !cont {
			return
		}
	}
}

func ShowTagIndexGap(ctx context.Context, conn *sql.Conn, opts ...ShowOption) *TagIndexGapResultSet {
	options := newShowOptions(opts...)
	if err := resolveShowScope(ctx, conn, &options); err != nil {
		return &TagIndexGapResultSet{ResultSetBase: ResultSetBase{err: err}}
	}
	args := []any{}
	sqlText := SqlTidy(`SELECT
			t.ID AS ID,
            t.NAME AS TABLE_NAME,
            i.INDEX_STATE AS STATUS,
            i.TABLE_END_RID - i.DISK_INDEX_END_RID AS DISK_GAP,
            i.TABLE_END_RID - i.MEMORY_INDEX_END_RID AS MEMORY_GAP
        from
            M$SYS_TABLES t,
			V$STORAGE_TAG_INDEX i,
			M$SYS_USERS u
        where
            t.ID = i.TABLE_ID
		AND t.USER_ID = u.USER_ID`, options.databaseClause("t.DATABASE_NAME", &args), options.userClause("u.NAME", &args), options.likeClause("t.NAME", &args), `order by id`)

	rows, err := conn.QueryContext(ctx, sqlText, args...)
	if err != nil {
		return &TagIndexGapResultSet{ResultSetBase: ResultSetBase{err: err}}
	}
	defer rows.Close()

	list := []*TagIndexGapInfo{}
	for rows.Next() {
		rec := &TagIndexGapInfo{}
		err := rows.Scan(&rec.TableId, &rec.TableName, &rec.Status, &rec.DiskGap, &rec.MemoryGap)
		if err != nil {
			return &TagIndexGapResultSet{ResultSetBase: ResultSetBase{err: err}}
		}
		list = append(list, rec)
	}
	err = rows.Err()
	return &TagIndexGapResultSet{ResultSetBase: ResultSetBase{err: err}, list: list}
}

type ShowRollupGapResultSet struct {
	ResultSetBase
	list []*RollupGapInfo
}

type RollupGapInfo struct {
	UserName        string    `json:"user_name"`
	RollupName      string    `json:"rollup_name"`
	SrcTable        string    `json:"src_table"`
	RollupTable     string    `json:"rollup_table"`
	SrcEndRID       int64     `json:"src_end_rid"`
	RollupEndRID    int64     `json:"rollup_end_rid"`
	Gap             int64     `json:"gap"`
	RunState        string    `json:"run_state"`
	LastElapsedMsec float64   `json:"last_elapsed_msec"`
	LastWakeupTime  time.Time `json:"last_wakeup_time"`
	NextWakeupTime  time.Time `json:"next_wakeup_time"`
}

var _ ResultSet = (*ShowRollupGapResultSet)(nil)

func (rgi *ShowRollupGapResultSet) Columns() client.Columns {
	return client.Columns{
		{Name: "USER_NAME", DataType: api.DataTypeString},
		{Name: "ROLLUP_NAME", DataType: api.DataTypeString},
		{Name: "SRC_TABLE", DataType: api.DataTypeString},
		{Name: "ROLLUP_TABLE", DataType: api.DataTypeString},
		{Name: "SRC_END_RID", DataType: api.DataTypeInt64},
		{Name: "ROLLUP_END_RID", DataType: api.DataTypeInt64},
		{Name: "GAP", DataType: api.DataTypeInt64},
		{Name: "RUN_STATE", DataType: api.DataTypeString},
		{Name: "LAST_ELAPSED_MSEC", DataType: api.DataTypeInt64},
		{Name: "LAST_WAKEUP_TIME", DataType: api.DataTypeDatetime},
		{Name: "NEXT_WAKEUP_TIME", DataType: api.DataTypeDatetime},
	}
}

func (rgi *ShowRollupGapResultSet) Iter(callback func(values []interface{}) bool) {
	for _, idx := range rgi.list {
		cont := callback([]interface{}{
			idx.UserName, idx.RollupName, idx.SrcTable, idx.RollupTable,
			idx.SrcEndRID, idx.RollupEndRID, idx.Gap, idx.RunState,
			idx.LastElapsedMsec, idx.LastWakeupTime, idx.NextWakeupTime,
		})
		if !cont {
			return
		}
	}
}

func ShowRollupGap(ctx context.Context, conn *sql.Conn, opts ...ShowOption) *ShowRollupGapResultSet {
	options := newShowOptions(opts...)
	if err := resolveShowScope(ctx, conn, &options); err != nil {
		return &ShowRollupGapResultSet{ResultSetBase: ResultSetBase{err: err}}
	}
	args := []any{}
	sqlText := SqlTidy(`SELECT
            U.NAME AS USER_NAME,
            C.ROLLUP_NAME AS ROLLUP_NAME,
            C.SOURCE_TABLE AS SRC_TABLE,
            C.ROLLUP_TABLE,
            B.TABLE_END_RID AS SRC_END_RID,
            C.END_RID AS ROLLUP_END_RID,
            B.TABLE_END_RID - C.END_RID AS GAP,
            CASE C.RUN_STATE WHEN 'I' THEN 'INIT' WHEN 'S' THEN 'SLEEPING' WHEN 'R' THEN 'RUNNING' ELSE 'UNKNOWN' END AS RUN_STATE,
            C.LAST_ELAPSED_MSEC AS LAST_ELAPSED_MSEC,
            C.LAST_WAKEUP_TIME AS LAST_WAKEUP_TIME,
            C.NEXT_WAKEUP_TIME AS NEXT_WAKEUP_TIME
        FROM
            M$SYS_TABLES A,
            V$STORAGE_TAG_TABLES B,
            V$ROLLUP C,
            M$SYS_USERS U
        WHERE 
			A.DATABASE_ID=C.DATABASE_ID
        AND A.ID=B.ID
        AND A.USER_ID=C.USER_ID
        AND A.NAME=C.SOURCE_TABLE
		AND U.USER_ID=C.USER_ID`,
		options.databaseClause("C.DATABASE_NAME", &args),
		options.userClause("U.NAME", &args),
		options.likeClause("C.SOURCE_TABLE", &args),
		`ORDER BY U.USER_ID, SRC_TABLE`)

	rows, err := conn.QueryContext(ctx, sqlText, args...)
	if err != nil {
		return &ShowRollupGapResultSet{ResultSetBase: ResultSetBase{err: err}}
	}
	defer rows.Close()

	list := []*RollupGapInfo{}
	for rows.Next() {
		rec := &RollupGapInfo{}
		err := rows.Scan(&rec.UserName, &rec.RollupName, &rec.SrcTable, &rec.RollupTable,
			&rec.SrcEndRID, &rec.RollupEndRID, &rec.Gap, &rec.RunState,
			&rec.LastElapsedMsec, &rec.LastWakeupTime, &rec.NextWakeupTime)
		if err != nil {
			return &ShowRollupGapResultSet{ResultSetBase: ResultSetBase{err: err}}
		}
		list = append(list, rec)
	}
	err = rows.Err()
	return &ShowRollupGapResultSet{ResultSetBase: ResultSetBase{err: err}, list: list}
}

type ShowTagsResultSet struct {
	ResultSetBase
	conn      *sql.Conn
	tableName string
	tagNames  []string
	like      string
	hasLike   bool
	desc      *TableDescription
}

var _ ResultSet = (*ShowTagsResultSet)(nil)

func (tr *ShowTagsResultSet) Columns() client.Columns {
	return client.Columns{
		{Name: "ID", DataType: api.DataTypeInt64},
		{Name: "NAME", DataType: api.DataTypeString},
		{Name: "ROW_COUNT", DataType: api.DataTypeInt64},
		{Name: "MIN_TIME", DataType: api.DataTypeDatetime},
		{Name: "MAX_TIME", DataType: api.DataTypeDatetime},
		{Name: "RECENT_ROW_TIME", DataType: api.DataTypeDatetime},
		{Name: "MIN_VALUE", DataType: api.DataTypeFloat64},
		{Name: "MIN_VALUE_TIME", DataType: api.DataTypeDatetime},
		{Name: "MAX_VALUE", DataType: api.DataTypeFloat64},
		{Name: "MAX_VALUE_TIME", DataType: api.DataTypeDatetime},
	}
}

func (tr *ShowTagsResultSet) Iter(callback func(values []interface{}) bool) {
	ctx := context.Background()
	ListTagsWalk(ctx, tr.conn, tr.tableName, tr.desc.TagNameColumn, func(tagInfo *TagInfo, err error) bool {
		if err != nil {
			return false
		}
		if len(tr.tagNames) > 0 {
			if !slices.Contains(tr.tagNames, tagInfo.Name) {
				return true // skip this tag
			}
		}
		if tr.hasLike && !LikeMatch(tr.like, tagInfo.Name) {
			return true
		}
		tagInfo.Summarized = tr.desc.Summarized
		if stat, err := QueryTagStat(ctx, tr.conn, tr.tableName, tagInfo.Name); err != nil {
			// some tags may not have stat
			// the err may be 'no rows in result set'
			// ignore the error, for processing the next tag
		} else {
			tagInfo.Stat = stat
		}

		var values []any
		if tagInfo.Stat != nil {
			if tagInfo.Summarized {
				values = []any{tagInfo.Id, tagInfo.Name, tagInfo.Stat.RowCount,
					tagInfo.Stat.MinTime, tagInfo.Stat.MaxTime, tagInfo.Stat.RecentRowTime,
					tagInfo.Stat.MinValue, tagInfo.Stat.MinValueTime,
					tagInfo.Stat.MaxValue, tagInfo.Stat.MaxValueTime}
			} else {
				values = []any{tagInfo.Id, tagInfo.Name, tagInfo.Stat.RowCount,
					tagInfo.Stat.MinTime, tagInfo.Stat.MaxTime, tagInfo.Stat.RecentRowTime,
					nil, nil, nil, nil}
			}
		} else {
			values = []any{tagInfo.Id, tagInfo.Name, nil,
				nil, nil, nil,
				nil, nil, nil, nil}
		}
		if !callback(values) {
			return false
		}
		return true
	})
}

func ShowTags(ctx context.Context, conn *sql.Conn, fallbackDatabaseName string, fallbackUserName string, tableName string, tagNames ...string) *ShowTagsResultSet {
	return ShowTagsWithOptions(ctx, conn, fallbackDatabaseName, fallbackUserName, tableName, tagNames)
}

func ShowTagsWithOptions(ctx context.Context, conn *sql.Conn, fallbackDatabaseName string, fallbackUserName string, tableName string, tagNames []string, opts ...ShowOption) *ShowTagsResultSet {
	if len(tagNames) > 0 {
		for _, option := range opts {
			options := newShowOptions(option)
			if options.hasLike {
				return &ShowTagsResultSet{ResultSetBase: ResultSetBase{err: fmt.Errorf("cannot use LIKE with explicit tag names")}}
			}
		}
	}

	options := newShowOptions(opts...)
	if err := options.validate(); err != nil {
		return &ShowTagsResultSet{ResultSetBase: ResultSetBase{err: err}}
	}
	if fallbackDatabaseName == "" || fallbackUserName == "" || len(opts) > 0 {
		if err := resolveShowScope(ctx, conn, &options); err != nil {
			return &ShowTagsResultSet{ResultSetBase: ResultSetBase{err: err}}
		}
		if fallbackDatabaseName == "" || options.hasDatabase {
			fallbackDatabaseName = options.database
		}
		if fallbackUserName == "" || options.hasUser {
			fallbackUserName = options.user
		}
	}
	originalTableName := strings.ToUpper(tableName)
	databaseName := fallbackDatabaseName
	userName := fallbackUserName
	parts := strings.SplitN(strings.ToUpper(tableName), ".", 3)
	if len(parts) == 3 {
		databaseName = parts[0]
		userName = parts[1]
		tableName = parts[2]
	} else if len(parts) == 2 {
		userName = parts[0]
		tableName = parts[1]
	}

	tableName = fmt.Sprintf("%s.%s.%s", databaseName, userName, tableName)
	rs := ShowTable(ctx, conn, "", "", tableName, false)
	if rs.err != nil {
		return &ShowTagsResultSet{ResultSetBase: ResultSetBase{err: rs.err}}
	}
	if rs.Description.Type != client.TableTypeTag {
		err := fmt.Errorf("table '%s' is not a tag table", originalTableName)
		return &ShowTagsResultSet{ResultSetBase: ResultSetBase{err: err}}
	}
	return &ShowTagsResultSet{conn: conn, tableName: tableName, tagNames: tagNames, like: options.like, hasLike: options.hasLike, desc: rs.Description}
}

type TagInfo struct {
	Database   string       `json:"database"`
	User       string       `json:"user"`
	Table      string       `json:"table"`
	Name       string       `json:"name"`
	Id         int64        `json:"id"`
	Summarized bool         `json:"summarized"`
	Stat       *TagStatInfo `json:"stat,omitempty"`
}

func (ti *TagInfo) Values() []interface{} {
	return []interface{}{
		ti.Database, ti.User, ti.Table, ti.Name, ti.Id, ti.Summarized,
	}
}

type TagStatInfo struct {
	Database      string    `json:"database"`
	User          string    `json:"user"`
	Table         string    `json:"table"`
	Name          string    `json:"name"`
	RowCount      int64     `json:"row_count"`
	MinTime       time.Time `json:"min_time"`
	MaxTime       time.Time `json:"max_time"`
	MinValue      float64   `json:"min_value"`
	MinValueTime  time.Time `json:"min_value_time"`
	MaxValue      float64   `json:"max_value"`
	MaxValueTime  time.Time `json:"max_value_time"`
	RecentRowTime time.Time `json:"recent_row_time"`
}

func (tsi *TagStatInfo) Values() []interface{} {
	return []interface{}{
		tsi.Database, tsi.User, tsi.Table, tsi.Name, tsi.RowCount,
		tsi.MinTime, tsi.MaxTime, tsi.MinValue, tsi.MinValueTime,
		tsi.MaxValue, tsi.MaxValueTime, tsi.RecentRowTime,
	}
}

func ListTagsWalk(ctx context.Context, conn *sql.Conn, table string, tagNameColumn string, callback func(*TagInfo, error) bool) {
	database, userName, tableName := TableName(table).Split()
	metaTableName := ""
	if database != "MACHBASEDB" {
		metaTableName = fmt.Sprintf("%s.%s._%s_META", database, userName, tableName)
	} else {
		metaTableName = fmt.Sprintf("%s._%s_META", userName, tableName)
	}
	rows, err := conn.QueryContext(ctx, fmt.Sprintf(`SELECT _ID, %s FROM %s`, tagNameColumn, metaTableName))
	if err != nil {
		callback(nil, err)
		return
	}
	defer rows.Close()
	for rows.Next() {
		nfo := &TagInfo{Database: database, User: userName, Table: tableName}
		err = rows.Scan(&nfo.Id, &nfo.Name)
		if !callback(nfo, err) {
			return
		}
	}
}

func QueryTagStat(ctx context.Context, conn *sql.Conn, table string, tag string) (*TagStatInfo, error) {
	database, user, table := TableName(table).Split()
	sqlText := SqlTidy(`SELECT`,
		`NAME, ROW_COUNT,`,
		`MIN_TIME, MAX_TIME,`,
		`MIN_VALUE, MIN_VALUE_TIME, MAX_VALUE, MAX_VALUE_TIME,`,
		`RECENT_ROW_TIME`,
		`FROM`,
		fmt.Sprintf("%s.%s.V$%s_STAT", database, user, table),
		`WHERE NAME = ?`)
	nfo := &TagStatInfo{
		Database: database,
		User:     user,
		Table:    table,
	}
	row := conn.QueryRowContext(ctx, sqlText, tag)
	if err := row.Err(); err != nil {
		return nil, row.Err()
	}
	var name sql.NullString
	var rowCount sql.NullInt64
	var minTime sql.NullTime
	var maxTime sql.NullTime
	var minValue sql.NullFloat64
	var minValueTime sql.NullTime
	var maxValue sql.NullFloat64
	var maxValueTime sql.NullTime
	var recentRowTime sql.NullTime
	err := row.Scan(
		&name, &rowCount,
		&minTime, &maxTime,
		&minValue, &minValueTime, &maxValue, &maxValueTime,
		&recentRowTime)

	if name.Valid {
		nfo.Name = name.String
	}
	if rowCount.Valid {
		nfo.RowCount = rowCount.Int64
	}
	if minTime.Valid {
		nfo.MinTime = minTime.Time
	}
	if maxTime.Valid {
		nfo.MaxTime = maxTime.Time
	}
	if minValue.Valid {
		nfo.MinValue = minValue.Float64
	}
	if minValueTime.Valid {
		nfo.MinValueTime = minValueTime.Time
	}
	if maxValue.Valid {
		nfo.MaxValue = maxValue.Float64
	}
	if maxValueTime.Valid {
		nfo.MaxValueTime = maxValueTime.Time
	}
	if recentRowTime.Valid {
		nfo.RecentRowTime = recentRowTime.Time
	}
	return nfo, err
}
