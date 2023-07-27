//go:build linux && arm
// +build linux,arm

package sqlite3

import "errors"

type bridge struct {
}

func New(name string, path string) *bridge {
	return nil
}

func (br *bridge) Name() string {
	return ""
}
func (br *bridge) String() string {
	return ""
}
func (br *bridge) BeforeRegister() error {
	return errors.New("not supported")
}
func (br *bridge) AfterUnregister() error {
	return errors.New("not supported")
}
