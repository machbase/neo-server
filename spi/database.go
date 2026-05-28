package spi

import (
	"crypto"

	"github.com/machbase/neo-client/api"
)

var defaultDatabase api.Database
var defaultDatabaseKey crypto.PrivateKey

func SetDefault(db api.Database) {
	defaultDatabase = db
}

func Default() api.Database {
	return defaultDatabase
}

func SetDefaultKey(key crypto.PrivateKey) {
	defaultDatabaseKey = key
}

func DefaultKey() crypto.PrivateKey {
	return defaultDatabaseKey
}
