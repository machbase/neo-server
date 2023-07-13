package main

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/machbase/neo-server/booter"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
)

func main() {
	booter.Startup()
	booter.WaitSignal()
	booter.ShutdownAndExit(0)
}

// //////////////////////////////////////////////
// register module
var AmodId = "github.com/booter/amod"
var BmodId = "github.com/booter/bmod"

func init() {
	booter.Register(AmodId,
		func() *AmodConf {
			return new(AmodConf)
		},
		func(conf *AmodConf) (booter.Boot, error) {
			instance := &Amod{
				conf: conf,
			}
			return instance, nil
		})
	booter.Register(BmodId,
		func() *BmodConf {
			return new(BmodConf)
		},
		func(conf *BmodConf) (booter.Boot, error) {
			instance := &Bmod{
				conf: *conf,
			}
			return instance, nil
		})

	os.Args = append(os.Args,
		"--logging-default-level", "WARN",
		"--logging-default-enable-source-location", "true",
		"--logging-default-prefix-width", "30",
	)
	var GetVersionFunc = function.New(&function.Spec{
		Params: []function.Parameter{},
		Type:   function.StaticReturnType(cty.String),
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			return cty.StringVal("1.2.3"), nil
		},
	})

	booter.SetFunction("customfunc", GetVersionFunc)
}

type AmodConf struct {
	Version   string
	TcpConfig TcpConfig
	Timeout   time.Duration
}

type TcpConfig struct {
	ListenAddress    string
	AdvertiseAddress string
	SoLinger         int
	KeepAlive        int
	NoDelay          bool
	Tls              TlsConfig
}

type TlsConfig struct {
	LoadSystemCAs    bool
	LoadPrivateCAs   bool
	CertFile         string
	KeyFile          string
	HandshakeTimeout time.Duration
}

type Amod struct {
	conf             *AmodConf
	Bmod             *Bmod
	OtherNameForBmod BmodInterface
}

func (am *Amod) Start() error {
	fmt.Println("amod start")
	fmt.Printf("    with Amod.Bmod             = %p\n", am.Bmod)
	fmt.Printf("    with Amod.OtherNameForBmod = %p\n", am.OtherNameForBmod)
	fmt.Printf("    config timeout = %v\n", am.conf.Timeout)
	if am.Bmod != am.OtherNameForBmod {
		return errors.New("amod.Bmod and amod.OtherNameForBmod has different references")
	}
	am.OtherNameForBmod.DoWork()
	return nil
}

func (am *Amod) Stop() {
	fmt.Println("amod stop")
}

type BmodConf struct {
	Filename                    string
	Append                      bool
	MaxBackups                  int
	RotateSchedule              string
	DefaultLevel                string
	DefaultPrefixWidth          int
	DefaultEnableSourceLocation bool
	Levels                      []LevelConf
}

type LevelConf struct {
	Pattern string
	Level   string
}

type BmodInterface interface {
	DoWork()
}

type Bmod struct {
	conf BmodConf
}

func (bm *Bmod) Start() error {
	fmt.Println("bmod start")
	return nil
}

func (bm *Bmod) Stop() {
	fmt.Println("bmod stop")
}

func (bm *Bmod) DoWork() {
	fmt.Println("bmod work...")
}
