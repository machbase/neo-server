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
