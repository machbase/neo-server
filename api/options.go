package api

import (
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
