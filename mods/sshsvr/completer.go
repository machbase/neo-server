package shell

import (
	"regexp"
	"strings"

	"github.com/c-bata/go-prompt"
)

func (sess *Session) completer(d prompt.Document) []prompt.Suggest {
	head := strings.ToUpper(d.CurrentLineBeforeCursor())
	match, _ := regexp.MatchString(`\s+FROM\s+\S*$`, head)
	if match {
		rows, err := sess.db.Query("select NAME, TYPE, FLAG from M$SYS_TABLES order by NAME")
		if err != nil {
			sess.log.Errorf("select m$sys_tables fail; %s", err.Error())
			return nil
		}
		defer rows.Close()
		rt := []prompt.Suggest{}
		for rows.Next() {
			var name string
			var typ int
			var flg int
			rows.Scan(&name, &typ, &flg)
			desc := tableTypeDesc(typ, flg)
			rt = append(rt, prompt.Suggest{Text: name, Description: desc})
		}
		tableNamePrefix := d.GetWordBeforeCursor()
		if len(tableNamePrefix) == 0 {
			return rt
		}
		return prompt.FilterHasPrefix(rt, tableNamePrefix, true)
	}
	return nil
}

func tableTypeDesc(typ int, flg int) string {
	desc := "undef"
	switch typ {
	case 0:
		desc = "Log Table"
	case 1:
		desc = "Fixed Table"
	case 3:
		desc = "Volatile Table"
	case 4:
		desc = "Lookup Table"
	case 5:
		desc = "KeyValue Table"
	case 6:
		desc = "Tag Table"
	}
	switch flg {
	case 1:
		desc += " (data)"
	case 2:
		desc += " (rollup)"
	case 4:
		desc += " (meta)"
	case 8:
		desc += " (stat)"
	}
	return desc
}
