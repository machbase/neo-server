package allowance

import (
	"strings"
	"sync"

	"github.com/machbase/neo-server/v8/mods/logging"
	"github.com/machbase/neo-server/v8/mods/util/glob"
)

type Policy int

const (
	PolicyNone Policy = iota
	PolicyBlackOnly
	PolicyWhiteOnly
	PolicyBlackThenWhite
	PolicyWhiteThenBlack
)

type BlackOrWhite int

const (
	Blacklist BlackOrWhite = 1 + iota
	Whitelist
)

var PolicyNames = []string{
	"NONE",
	"BLACKLISTONLY",
	"WHITELISTONLY",
	"BLACKLIST_WHITELIST",
	"WHITELIST_BLACKLIST",
}

type AllowanceConfig struct {
	Policy    string   `default:"NONE" name:"policy" enum:"NONE,DENY,ALLOW,DENY_ALLOW,ALLOW_DENY"`
	Whitelist []string `default:"" name:"allow"`
	Blacklist []string `default:"" name:"deny"`
}

func NewAllowanceFromConfig(conf *AllowanceConfig) Allowance {
	if conf == nil {
		return NewAllowance(PolicyNone)
	}

	policy := ParsePolicyName(conf.Policy)
	allowance := NewAllowance(policy)

	if len(conf.Whitelist) > 0 {
		allowance.Update(Whitelist, conf.Whitelist)
	}
	if len(conf.Blacklist) > 0 {
		allowance.Update(Blacklist, conf.Blacklist)
	}
	return allowance
}

type Allowance interface {
	Allow(remoteHost string) bool
	Update(blackOrWhite BlackOrWhite, hosts []string)
}

func NewAllowance(policy Policy) Allowance {
	switch policy {
	default:
		return newNonePolicy()
	case PolicyBlackOnly:
		return newBlacklistPolicy()
	case PolicyWhiteOnly:
		return newWhitelistPolicy()
	case PolicyBlackThenWhite:
		return &multiPolicy{
			policies: []Allowance{newBlacklistPolicy(), newWhitelistPolicy()},
		}
	case PolicyWhiteThenBlack:
		return &multiPolicy{
			policies: []Allowance{newWhitelistPolicy(), newBlacklistPolicy()},
		}
	}
}

type nonePolicy struct {
	log logging.Log
}

func newNonePolicy() Allowance {
	return &nonePolicy{
		log: logging.GetLog("allowance-all"),
	}
}
func (p *nonePolicy) Allow(remoteHost string) bool {
	// none policy returns always true
	if p.log.TraceEnabled() {
		p.log.Tracef("allow %s", remoteHost)
	}
	return true
}

func (p *nonePolicy) Update(blackOrWhite BlackOrWhite, entires []string) {
	// do nothing, no updates
}

type entryPolicy struct {
	blackOrWhite BlackOrWhite
	mutex        sync.Mutex
	log          logging.Log
	hosts        []string
}

func newBlacklistPolicy() Allowance {
	return &entryPolicy{
		blackOrWhite: Blacklist,
		mutex:        sync.Mutex{},
		log:          logging.GetLog("allowance-deny"),
	}
}

func newWhitelistPolicy() Allowance {
	return &entryPolicy{
		blackOrWhite: Whitelist,
		mutex:        sync.Mutex{},
		log:          logging.GetLog("allowance-allow"),
	}
}

func (p *entryPolicy) Allow(remoteHost string) bool {
	var found = false
	p.mutex.Lock()
	for _, h := range p.hosts {
		if h == remoteHost {
			found = true
		} else {
			matched, err := glob.Match(h, remoteHost)
			if err != nil {
				p.log.Warnf("invalid glob match, pattern %s against %s, %s", h, remoteHost, err)
			} else {
				found = matched
			}
		}

		if found {
			break
		}
	}
	p.mutex.Unlock()

	switch p.blackOrWhite {
	case Blacklist:
		if found {
			return false
		} else {
			return true
		}
	case Whitelist:
		if found {
			return true
		} else {
			return false
		}
	}
	return false
}

func (p *entryPolicy) Update(blackOrWhite BlackOrWhite, hosts []string) {
	if p.blackOrWhite != blackOrWhite {
		return
	}
	p.mutex.Lock()
	p.hosts = hosts
	p.mutex.Unlock()
}

type multiPolicy struct {
	policies []Allowance
}

func (mp *multiPolicy) Allow(remoteHost string) bool {
	for _, p := range mp.policies {
		allow := p.Allow(remoteHost)
		if !allow {
			return false
		}
	}
	return true
}

func (mp *multiPolicy) Update(blackOrWhite BlackOrWhite, hosts []string) {
	for _, p := range mp.policies {
		p.Update(blackOrWhite, hosts)
	}
}

func ParsePolicyName(n string) Policy {
	name := strings.ToUpper(n)
	switch name {
	default:
		return PolicyNone
	case "NONE", "ALL":
		return PolicyNone
	case "BLACKLISTONLY", "BLACKLIST", "BLACK", "DENY":
		return PolicyBlackOnly
	case "WHITELISTONLY", "WHITELIST", "WHITE", "ALLOW":
		return PolicyWhiteOnly
	case "BLACKLIST_WHITELIST", "BLACK_WHITE", "BLACKWHITE", "DENY_ALLOW":
		return PolicyBlackThenWhite
	case "WHITELIST_BLACKLIST", "WHITE_BLACK", "WHITEBLACK", "ALLOW_DENY":
		return PolicyWhiteThenBlack
	}
}
