package util

import "github.com/robfig/cron/v3"

var defaultCron *cron.Cron

func DefaultCron() *cron.Cron {
	return defaultCron
}

func SetDefaultCron(cron *cron.Cron) {
	if defaultCron != nil {
		panic("default cron already set")
	}
	defaultCron = cron
}
