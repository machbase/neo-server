package machsvr

import (
	"crypto"
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

func WithPassword(user string, password string) ConnectOption {
	return &ConnectOptionPassword{User: user, Password: password}
}

type ConnectOptionAuthKey struct {
	User     string
	Key      crypto.PrivateKey
	AuthMode string
}

func (ConnectOptionAuthKey) connectOption() {}

func WithAuthKey(user string, key crypto.PrivateKey) ConnectOption {
	return &ConnectOptionAuthKey{
		User:     user,
		Key:      key,
		AuthMode: "CHALLENGE",
	}
}

type ConnectOptionProxyUser struct {
	ProxyUser string
}

func WithProxyUser(proxyUser string) ConnectOption {
	return &ConnectOptionProxyUser{ProxyUser: proxyUser}
}

func (ConnectOptionProxyUser) connectOption() {}

type ConnectOptionDatabase struct {
	Database string
}

// WithDatabase selects the initial database for the connection.
func WithDatabase(database string) ConnectOption {
	return &ConnectOptionDatabase{Database: database}
}

func (ConnectOptionDatabase) connectOption() {}

func WithTimeLocation(loc *time.Location) ConnectOption {
	return &ConnectOptionTimeLocation{Location: loc}
}

type ConnectOptionTimeLocation struct {
	Location *time.Location
}

func (ConnectOptionTimeLocation) connectOption() {}

func WithFetchRows(rows int64) ConnectOption {
	return &ConnectOptionFetchRows{Rows: rows}
}

type ConnectOptionFetchRows struct {
	Rows int64
}

func (ConnectOptionFetchRows) connectOption() {}

func WithIOMetrics(enabled bool) ConnectOption {
	return &ConnectOptionIOMetrics{Enabled: enabled}
}

type ConnectOptionIOMetrics struct {
	Enabled bool
}

func (ConnectOptionIOMetrics) connectOption() {}

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
