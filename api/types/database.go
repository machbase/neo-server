package types

import (
	"fmt"
)

// 0: Log Table, 1: Fixed Table, 3: Volatile Table,
// 4: Lookup Table, 5: KeyValue Table, 6: Tag Table
type TableType int

const (
	TableTypeLog      TableType = iota + 0
	TableTypeFixed    TableType = 1
	TableTypeVolatile TableType = 3
	TableTypeLookup   TableType = 4
	TableTypeKeyValue TableType = 5
	TableTypeTag      TableType = 6
)

func (typ TableType) String() string {
	switch typ {
	case TableTypeLog:
		return "LogTable"
	case TableTypeFixed:
		return "FixedTable"
	case TableTypeVolatile:
		return "VolatileTable"
	case TableTypeLookup:
		return "LookupTable"
	case TableTypeKeyValue:
		return "KeyValueTable"
	case TableTypeTag:
		return "TagTable"
	default:
		return fmt.Sprintf("UndefinedTable-%d", typ)
	}
}

func (typ TableType) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf(`"%s"`, typ.String())), nil
}

type TableFlag int

const (
	TableFlagNone   TableFlag = 0
	TableFlagData   TableFlag = 1
	TableFlagRollup TableFlag = 2
	TableFlagMeta   TableFlag = 4
	TableFlagStat   TableFlag = 8
)

func (flag TableFlag) String() string {
	switch flag {
	case TableFlagNone:
		return ""
	case TableFlagData:
		return "Data"
	case TableFlagRollup:
		return "Rollup"
	case TableFlagMeta:
		return "Meta"
	case TableFlagStat:
		return "Stat"
	default:
		return fmt.Sprintf("UndefinedTableFlag-%d", flag)
	}
}

func (flag TableFlag) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf(`"%s"`, flag.String())), nil
}

type IndexType int

const (
	IndexTypeBitmap   IndexType = iota + 6
	IndexTypeRedBlack IndexType = 8
	IndexTypeKeyword  IndexType = 9
	IndexTypeTag      IndexType = 11
)

func (typ IndexType) String() string {
	switch typ {
	case IndexTypeBitmap:
		return "BITMAP (LSM)"
	case IndexTypeRedBlack:
		return "REDBLACK"
	case IndexTypeKeyword:
		return "KEYWORD (LSM)"
	case IndexTypeTag:
		return "TAG"
	default:
		return fmt.Sprintf("UndefinedIndex-%d", typ)
	}
}
