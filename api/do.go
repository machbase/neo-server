package api

import (
	"strings"
	"time"
)

type TableName string

func (tn TableName) String() string {
	return strings.ToUpper(string(tn))
}

// Split splits the full table name that consists of database, user, and table name.
func (tn TableName) SplitOr(dbName string, userName string) (string, string, string) {
	tableName := strings.ToUpper(string(tn))
	parts := strings.SplitN(tableName, ".", 3)
	if len(parts) == 2 {
		userName = parts[0]
		tableName = parts[1]
	} else if len(parts) == 3 {
		dbName = parts[0]
		userName = parts[1]
		tableName = parts[2]
	}
	return dbName, userName, tableName
}

// Split splits the full table name that consists of database, user, and table name.
func (tn TableName) Split() (string, string, string) {
	dbName := "MACHBASEDB"
	userName := "SYS"
	tableName := strings.ToUpper(string(tn))
	parts := strings.SplitN(tableName, ".", 3)
	if len(parts) == 2 {
		userName = parts[0]
		tableName = parts[1]
	} else if len(parts) == 3 {
		dbName = parts[0]
		userName = parts[1]
		tableName = parts[2]
	}
	return dbName, userName, tableName
}

type InfoType interface {
	Columns() Columns
	Values() []interface{}
	Err() error
}

type TableInfo struct {
	Database string    `json:"database"`       // M$SYS_TABLES.DATABASE_ID
	User     string    `json:"user"`           // M$SYS_USERS.NAME
	Name     string    `json:"name"`           // M$SYS_TABLES.NAME
	Id       int64     `json:"id"`             // M$SYS_TABLES.ID
	Type     TableType `json:"type"`           // M$SYS_TABLES.TYPE
	Flag     TableFlag `json:"flag,omitempty"` // M$SYS_TABLES.FLAG
	err      error     `json:"-"`
}

func (ti *TableInfo) Kind() string {
	return TableTypeDescription(ti.Type, ti.Flag)
}

func (ti *TableInfo) Err() error {
	return ti.err
}

func (ti *TableInfo) Columns() Columns {
	return Columns{
		{Name: "DATABASE", DataType: DataTypeString},
		{Name: "USER", DataType: DataTypeString},
		{Name: "NAME", DataType: DataTypeString},
		{Name: "ID", DataType: DataTypeInt64},
		{Name: "TYPE", DataType: DataTypeString},
		{Name: "FLAG", DataType: DataTypeString},
	}
}

func (ti *TableInfo) Values() []interface{} {
	return []interface{}{ti.Database, ti.User, ti.Name, ti.Id, ti.Type.ShortString(), ti.Flag.String()}
}

// TableDescription is represents data that comes as a result of 'desc <table>'
type TableDescription struct {
	Database string              `json:"database"`
	User     string              `json:"user"`
	Name     string              `json:"name"`
	Id       int64               `json:"id"`
	Type     TableType           `json:"type"`
	Flag     TableFlag           `json:"flag,omitempty"`
	Columns  Columns             `json:"columns"`
	Indexes  []*IndexDescription `json:"indexes"`
}

// String returns string representation of table type.
func (td *TableDescription) String() string {
	return TableTypeDescription(td.Type, td.Flag)
}

type IndexDescription struct {
	Id   int64     `json:"id"`
	Name string    `json:"name"`
	Type IndexType `json:"type"`
	Cols []string  `json:"columns"`
}

type IndexInfo struct {
	Id             int64  `json:"id"`
	Database       string `json:"database"`
	DatabaseId     int64  `json:"database_id,omitempty"`
	User           string `json:"user"`
	TableName      string `json:"table_name"`
	ColumnName     string `json:"column_name"`
	IndexName      string `json:"index_name"`
	IndexType      string `json:"index_type"`
	KeyCompress    string `json:"key_compress"`
	MaxLevel       int64  `json:"max_level"`
	PartValueCount int64  `json:"part_value_count"`
	BitMapEncode   string `json:"bitmap_encode"`
	err            error  `json:"-"`
}

func (ii *IndexInfo) Columns() Columns {
	return Columns{
		{Name: "ID", DataType: DataTypeInt64},
		{Name: "DATABASE", DataType: DataTypeString},
		{Name: "USER", DataType: DataTypeString},
		{Name: "TABLE_NAME", DataType: DataTypeString},
		{Name: "COLUMN_NAME", DataType: DataTypeString},
		{Name: "INDEX_NAME", DataType: DataTypeString},
		{Name: "INDEX_TYPE", DataType: DataTypeString},
		{Name: "KEY_COMPRESS", DataType: DataTypeString},
		{Name: "MAX_LEVEL", DataType: DataTypeInt64},
		{Name: "PART_VALUE_COUNT", DataType: DataTypeInt64},
		{Name: "BITMAP_ENCODE", DataType: DataTypeString},
	}
}

func (ii *IndexInfo) Values() []interface{} {
	return []interface{}{
		ii.Id, ii.Database, ii.User, ii.TableName, ii.ColumnName, ii.IndexName,
		ii.IndexType, ii.KeyCompress, ii.MaxLevel, ii.PartValueCount, ii.BitMapEncode,
	}
}

func (ii *IndexInfo) Err() error {
	return ii.err
}

type LsmIndexInfo struct {
	TableName string `json:"table_name"`
	IndexName string `json:"index_name"`
	Level     int64  `json:"level"`
	Count     int64  `json:"count"`
	err       error  `json:"-"`
}

func (li *LsmIndexInfo) Columns() Columns {
	return Columns{
		{Name: "TABLE_NAME", DataType: DataTypeString},
		{Name: "INDEX_NAME", DataType: DataTypeString},
		{Name: "LEVEL", DataType: DataTypeInt64},
		{Name: "COUNT", DataType: DataTypeInt64},
	}
}

func (li *LsmIndexInfo) Values() []interface{} {
	return []interface{}{
		li.TableName, li.IndexName, li.Level, li.Count,
	}
}

func (li *LsmIndexInfo) Err() error {
	return li.err
}

type LicenseInfo struct {
	Id            string `json:"id"`
	Type          string `json:"type"`
	Customer      string `json:"customer"`
	Project       string `json:"project"`
	CountryCode   string `json:"countryCode"`
	InstallDate   string `json:"installDate"`
	IssueDate     string `json:"issueDate"`
	LicenseStatus string `json:"licenseStatus,omitempty"`
}

func (li *LicenseInfo) Columns() Columns {
	return Columns{
		{Name: "ID", DataType: DataTypeString},
		{Name: "TYPE", DataType: DataTypeString},
		{Name: "CUSTOMER", DataType: DataTypeString},
		{Name: "PROJECT", DataType: DataTypeString},
		{Name: "COUNTRY_CODE", DataType: DataTypeString},
		{Name: "INSTALL_DATE", DataType: DataTypeString},
		{Name: "ISSUE_DATE", DataType: DataTypeString},
		{Name: "LICENSE_STATUS", DataType: DataTypeString},
	}
}

func (li *LicenseInfo) Values() []interface{} {
	return []interface{}{
		"ID", li.Id,
		"TYPE", li.Type,
		"CUSTOMER", li.Customer,
		"PROJECT", li.Project,
		"COUNTRY_CODE", li.CountryCode,
		"INSTALL_DATE", li.InstallDate,
		"ISSUE_DATE", li.IssueDate,
		"LICENSE_STATUS", li.LicenseStatus,
	}
}

type TagInfo struct {
	Database   string       `json:"database"`
	User       string       `json:"user"`
	Table      string       `json:"table"`
	Name       string       `json:"name"`
	Id         int64        `json:"id"`
	Err        error        `json:"-"`
	Summarized bool         `json:"summarized"`
	Stat       *TagStatInfo `json:"stat,omitempty"`
}

func (ti *TagInfo) Columns() Columns {
	return Columns{
		{Name: "DATABASE", DataType: DataTypeString},
		{Name: "USER", DataType: DataTypeString},
		{Name: "TABLE", DataType: DataTypeString},
		{Name: "NAME", DataType: DataTypeString},
		{Name: "ID", DataType: DataTypeInt64},
		{Name: "SUMMARIZED", DataType: DataTypeBoolean},
	}
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

func (tsi *TagStatInfo) Columns() Columns {
	return Columns{
		{Name: "DATABASE", DataType: DataTypeString},
		{Name: "USER", DataType: DataTypeString},
		{Name: "TABLE", DataType: DataTypeString},
		{Name: "NAME", DataType: DataTypeString},
		{Name: "ROW_COUNT", DataType: DataTypeInt64},
		{Name: "MIN_TIME", DataType: DataTypeDatetime},
		{Name: "MAX_TIME", DataType: DataTypeDatetime},
		{Name: "MIN_VALUE", DataType: DataTypeFloat64},
		{Name: "MIN_VALUE_TIME", DataType: DataTypeDatetime},
		{Name: "MAX_VALUE", DataType: DataTypeFloat64},
		{Name: "MAX_VALUE_TIME", DataType: DataTypeDatetime},
		{Name: "RECENT_ROW_TIME", DataType: DataTypeDatetime},
	}
}

func (tsi *TagStatInfo) Values() []interface{} {
	return []interface{}{
		tsi.Database, tsi.User, tsi.Table, tsi.Name, tsi.RowCount,
		tsi.MinTime, tsi.MaxTime, tsi.MinValue, tsi.MinValueTime,
		tsi.MaxValue, tsi.MaxValueTime, tsi.RecentRowTime,
	}
}

type IndexGapInfo struct {
	ID         int64  `json:"id"`         // indexgap, tagindexgap
	TableName  string `json:"table_name"` // indexgap
	IndexName  string `json:"index_name"` // indexgap
	Gap        int64  `json:"gap"`        // indexgap
	IsTagIndex bool   `json:"is_tag_index"`
	Status     string `json:"status"`     // tagindexgap
	DiskGap    int64  `json:"disk_gap"`   // tagindexgap
	MemoryGap  int64  `json:"memory_gap"` // tagindexgap
	err        error  `json:"-"`
}

func (igi *IndexGapInfo) Columns() Columns {
	if igi.IsTagIndex {
		return Columns{
			{Name: "ID", DataType: DataTypeInt64},
			{Name: "STATUS", DataType: DataTypeString},
			{Name: "DISK_GAP", DataType: DataTypeInt64},
			{Name: "MEMORY_GAP", DataType: DataTypeInt64},
		}
	} else {
		return Columns{
			{Name: "ID", DataType: DataTypeInt64},
			{Name: "TABLE", DataType: DataTypeString},
			{Name: "INDEX", DataType: DataTypeString},
			{Name: "GAP", DataType: DataTypeInt64},
		}
	}
}

func (igi *IndexGapInfo) Values() []interface{} {
	if igi.IsTagIndex {
		return []interface{}{
			igi.ID, igi.Status, igi.DiskGap, igi.MemoryGap,
		}
	} else {
		return []interface{}{
			igi.ID, igi.TableName, igi.IndexName, igi.Gap,
		}
	}
}

func (igi *IndexGapInfo) Err() error {
	return igi.err
}

type RollupGapInfo struct {
	SrcTable     string        `json:"src_table"`
	RollupTable  string        `json:"rollup_table"`
	SrcEndRID    int64         `json:"src_end_rid"`
	RollupEndRID int64         `json:"rollup_end_rid"`
	Gap          int64         `json:"gap"`
	LastElapsed  time.Duration `json:"last_time"`
	err          error         `json:"-"`
}

func (rgi *RollupGapInfo) Columns() Columns {
	return Columns{
		{Name: "SRC_TABLE", DataType: DataTypeString},
		{Name: "ROLLUP_TABLE", DataType: DataTypeString},
		{Name: "SRC_END_RID", DataType: DataTypeInt64},
		{Name: "ROLLUP_END_RID", DataType: DataTypeInt64},
		{Name: "GAP", DataType: DataTypeInt64},
		{Name: "LAST_TIME", DataType: DataTypeInt64},
	}
}

func (rgi *RollupGapInfo) Values() []interface{} {
	return []interface{}{
		rgi.SrcTable, rgi.RollupTable, rgi.SrcEndRID, rgi.RollupEndRID, rgi.Gap, rgi.LastElapsed,
	}
}

func (rgi *RollupGapInfo) Err() error {
	return rgi.err
}

type StorageInfo struct {
	TableName string `json:"table_name"`
	DataSize  int64  `json:"data_size"`
	IndexSize int64  `json:"index_size"`
	TotalSize int64  `json:"total_size"`
	err       error  `json:"-"`
}

func (si *StorageInfo) Columns() Columns {
	return Columns{
		{Name: "TABLE_NAME", DataType: DataTypeString},
		{Name: "DATA_SIZE", DataType: DataTypeInt64},
		{Name: "INDEX_SIZE", DataType: DataTypeInt64},
		{Name: "TOTAL_SIZE", DataType: DataTypeInt64},
	}
}

func (si *StorageInfo) Values() []interface{} {
	return []interface{}{
		si.TableName, si.DataSize, si.IndexSize, si.TotalSize,
	}
}

func (si *StorageInfo) Err() error {
	return si.err
}

type TableUsageInfo struct {
	TableName    string `json:"table_name"`
	StorageUsage int64  `json:"storage_usage"`
	err          error  `json:"-"`
}

func (tui *TableUsageInfo) Columns() Columns {
	return Columns{
		{Name: "TABLE_NAME", DataType: DataTypeString},
		{Name: "STORAGE_USAGE", DataType: DataTypeInt64},
	}
}

func (tui *TableUsageInfo) Values() []interface{} {
	return []interface{}{
		tui.TableName, tui.StorageUsage,
	}
}

func (tui *TableUsageInfo) Err() error {
	return tui.err
}

type StatementInfo struct {
	ID                 int64  `json:"id"`                   // v$stmt, v$neo_stmt
	SessionID          int64  `json:"session_id"`           // v$stmt, v$neo_stmt
	State              string `json:"state"`                // v$stmt, v$neo_stmt
	Query              string `json:"query"`                // v$stmt, v$neo_stmt
	RecordSize         int64  `json:"record_size"`          // v$stmt
	IsNeo              bool   `json:"is_neo"`               // v$neo_stmt
	AppendSuccessCount int64  `json:"append_success_count"` // v$neo_stmt
	AppendFailureCount int64  `json:"append_failure_count"` // v$neo_stmt
	err                error  `json:"-"`
}

func (si *StatementInfo) Columns() Columns {
	return Columns{
		{Name: "ID", DataType: DataTypeInt64},
		{Name: "SESSION_ID", DataType: DataTypeInt64},
		{Name: "STATE", DataType: DataTypeString},
		{Name: "TYPE", DataType: DataTypeString},
		{Name: "RECORD_SIZE", DataType: DataTypeInt64},
		{Name: "APPEND_SUCCESS_CNT", DataType: DataTypeInt64},
		{Name: "APPEND_FAILURE_CNT", DataType: DataTypeInt64},
		{Name: "QUERY", DataType: DataTypeString},
	}
}

func (si *StatementInfo) Values() []interface{} {
	var typ string
	var recordSize any
	var appendSuccessCount any
	var appendFailureCount any
	if si.IsNeo {
		typ = "neo"
		appendSuccessCount = si.AppendSuccessCount
		appendFailureCount = si.AppendFailureCount
	} else {
		typ = ""
		recordSize = si.RecordSize
	}
	return []interface{}{
		si.ID, si.SessionID, si.State, typ, recordSize, appendSuccessCount, appendFailureCount, si.Query,
	}
}

func (si *StatementInfo) Err() error {
	return si.err
}

type SessionInfo struct {
	ID        int64     `json:"id"`          // v$session, v$neo_session
	UserID    int64     `json:"user_id"`     // v$session, v$neo_session
	UserName  string    `json:"user_name"`   // v$session, v$neo_session
	LoginTime time.Time `json:"login_time"`  // v$session
	MaxQPXMem int64     `json:"max_qpx_mem"` // v$session
	IsNeo     bool      `json:"is_neo"`      // v$neo_session
	StmtCount int64     `json:"stmt_count"`  // v$neo_session
	err       error     `json:"-"`
}

func (si *SessionInfo) Columns() Columns {
	return Columns{
		{Name: "ID", DataType: DataTypeInt64},
		{Name: "USER_ID", DataType: DataTypeInt64},
		{Name: "USER_NAME", DataType: DataTypeString},
		{Name: "TYPE", DataType: DataTypeString},
		{Name: "LOGIN_TIME", DataType: DataTypeDatetime},
		{Name: "MAX_QPX_MEM", DataType: DataTypeInt64},
		{Name: "STMT_COUNT", DataType: DataTypeInt64},
	}
}

func (si *SessionInfo) Values() []interface{} {
	typ := ""
	var qpxMem any
	var stmtCount any
	var loginTime any
	if si.IsNeo {
		typ = "neo"
		stmtCount = si.StmtCount
	} else {
		loginTime = si.LoginTime
		qpxMem = si.MaxQPXMem
	}
	return []interface{}{
		si.ID, si.UserID, si.UserName, typ, loginTime, qpxMem, stmtCount,
	}
}

func (si *SessionInfo) Err() error {
	return si.err
}
