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

type TableInfo struct {
	Database string    `json:"database"`       // M$SYS_TABLES.DATABASE_ID
	User     string    `json:"user"`           // M$SYS_USERS.NAME
	Name     string    `json:"name"`           // M$SYS_TABLES.NAME
	Id       int64     `json:"id"`             // M$SYS_TABLES.ID
	Type     TableType `json:"type"`           // M$SYS_TABLES.TYPE
	Flag     TableFlag `json:"flag,omitempty"` // M$SYS_TABLES.FLAG
	Err      error     `json:"-"`
}

func (ti *TableInfo) Kind() string {
	return TableTypeDescription(ti.Type, ti.Flag)
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
	Id         int64    `json:"id"`
	Database   string   `json:"database"`
	User       string   `json:"user"`
	Name       string   `json:"name"`
	Type       string   `json:"type"`
	Table      string   `json:"table"`
	Cols       []string `json:"columns"`
	DatabaseId int      `json:"database_id"`
	Err        error    `json:"-"`
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
