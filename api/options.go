package api

import (
	"strings"
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
