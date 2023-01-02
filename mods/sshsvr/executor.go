package shell

import (
	"strings"
)

func (sess *Session) executor(line string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}
	lineUpper := strings.ToUpper(line)
	if lineUpper == "EXIT" {
		sess.Close()
	} else if strings.HasPrefix(lineUpper, "SHOW") {
		sess.exec_show(lineUpper)
	} else {
		sess.exec_sql(line)
	}
}
