package api

import (
	"regexp"
	"strings"
	"time"
)

type ConnectOption interface {
	connectOption()
}

type ConnectOptionPassword struct {
	User     string
	Password string
}

func (ConnectOptionPassword) connectOption() {}

type ConnectOptionTrustUser struct {
	User string
}

func (ConnectOptionTrustUser) connectOption() {}

func WithPassword(user string, password string) ConnectOption {
	return &ConnectOptionPassword{User: user, Password: password}
}

func WithTrustUser(user string) ConnectOption {
	return &ConnectOptionTrustUser{User: user}
}

type StatementCacheMode int

const (
	StatementCacheOff  StatementCacheMode = 0
	StatementCacheOn   StatementCacheMode = 1
	StatementCacheAuto StatementCacheMode = 2
)

func WithStatementCache(mode StatementCacheMode) ConnectOption {
	return &ConnectOptionStatementCache{Mode: mode}
}

type ConnectOptionStatementCache struct {
	Mode StatementCacheMode
}

func (ConnectOptionStatementCache) connectOption() {}

func WithFetchRows(rows int64) ConnectOption {
	return &ConnectOptionFetchRows{Rows: rows}
}

type ConnectOptionFetchRows struct {
	Rows int64
}

func (ConnectOptionFetchRows) connectOption() {}

type AppenderOption interface {
	appenderOption()
}

type AppenderOptionBuffer struct {
	Threshold int
}

func (AppenderOptionBuffer) appenderOption() {}

func WithAppenderBuffer(threshold int) *AppenderOptionBuffer {
	return &AppenderOptionBuffer{Threshold: threshold}
}

func SqlTidy(sqlTextLines ...string) string {
	sqlText := strings.Join(sqlTextLines, "\n")
	lines := strings.Split(sqlText, "\n")
	for i, ln := range lines {
		lines[i] = strings.TrimSpace(ln)
	}
	return strings.Join(lines, " ")
}

var sqlTidyWidthRegexp = regexp.MustCompile(`\s+`)

func SqlTidyWidth(width int, sqlTextLines ...string) string {
	sqlText := SqlTidy(sqlTextLines...)
	sqlText = sqlTidyWidthRegexp.ReplaceAllString(sqlText, " ")

	words := strings.Split(sqlText, " ")
	lines := []string{}
	currentLine := ""

	for _, word := range words {
		if len(currentLine)+len(word)+1 > width {
			lines = append(lines, currentLine)
			currentLine = word
		} else {
			if currentLine != "" {
				currentLine += " "
			}
			currentLine += word
		}
	}
	if currentLine != "" {
		lines = append(lines, currentLine)
	}
	return strings.Join(lines, "\n")
}

type ConnectOptionTimeout struct {
	Timeout time.Duration
}

func (ConnectOptionTimeout) connectOption() {}

// ConnectTimeout
//
// if ConnectTimeout is set, Connect() will wait for the connection to be established
// if the connection is not established within the timeout, Connect() will return an error
//
//	0 : no timeout
//	> 0 : timeout duration
func WithConnectTimeout(timeout time.Duration) ConnectOption {
	return &ConnectOptionTimeout{Timeout: timeout}
}
