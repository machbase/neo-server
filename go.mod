module github.com/machbase/neo-server

go 1.21

require (
	github.com/Azure/go-ansiterm v0.0.0-20230124172434-306776ec8161
	github.com/Masterminds/semver/v3 v3.2.1
	github.com/alecthomas/chroma/v2 v2.12.0
	github.com/alecthomas/kong v0.8.0
	github.com/asaskevich/EventBus v0.0.0-20200907212545-49d423059eef
	github.com/atotto/clipboard v0.1.4
	github.com/creack/pty v1.1.18
	github.com/d5/tengo/v2 v2.16.1
	github.com/eclipse/paho.mqtt.golang v1.4.3
	github.com/gdamore/tcell/v2 v2.7.4
	github.com/gin-contrib/cors v1.4.0
	github.com/gin-gonic/gin v1.9.1
	github.com/gliderlabs/ssh v0.3.5
	github.com/go-git/go-git/v5 v5.11.0
	github.com/go-sql-driver/mysql v1.7.1
	github.com/gofrs/uuid/v5 v5.1.0
	github.com/golang-jwt/jwt/v4 v4.5.0
	github.com/gorilla/websocket v1.5.0
	github.com/hashicorp/hcl/v2 v2.17.0
	github.com/influxdata/line-protocol/v2 v2.2.1
	github.com/jchenry/goldmark-pikchr v0.1.0
	github.com/jedib0t/go-pretty/v6 v6.5.8
	github.com/lib/pq v1.10.9
	github.com/machbase/neo-client v1.0.1
	github.com/machbase/neo-engine v1.3.3
	github.com/magefile/mage v1.15.0
	github.com/mattn/go-colorable v0.1.13
	github.com/mattn/go-sqlite3 v1.14.17
	github.com/mbndr/figlet4go v0.0.0-20190224160619-d6cef5b186ea
	github.com/microsoft/go-mssqldb v1.5.0
	github.com/nats-io/nats.go v1.35.0
	github.com/nyaosorg/go-box/v2 v2.1.4
	github.com/nyaosorg/go-readline-ny v1.0.1
	github.com/orcaman/concurrent-map v1.0.0
	github.com/pkg/errors v0.9.1
	github.com/rcrowley/go-metrics v0.0.0-20201227073835-cf1acfcdf475
	github.com/rivo/tview v0.0.0-20230621164836-6cc0565babaf
	github.com/robfig/cron/v3 v3.0.1
	github.com/sevlyar/go-daemon v0.1.6
	github.com/stretchr/testify v1.9.0
	github.com/tidwall/gjson v1.14.4
	github.com/wroge/wgs84 v1.1.7
	github.com/yuin/goldmark v1.7.0
	github.com/yuin/goldmark-highlighting/v2 v2.0.0-20230729083705-37449abec8cc
	github.com/zclconf/go-cty v1.13.2
	go.abhg.dev/goldmark/mermaid v0.5.0
	golang.org/x/crypto v0.24.0
	golang.org/x/exp v0.0.0-20230522175609-2e198f4a06a1
	golang.org/x/net v0.26.0
	golang.org/x/sys v0.21.0
	golang.org/x/term v0.21.0
	golang.org/x/text v0.16.0
	gonum.org/v1/gonum v0.13.0
	google.golang.org/grpc v1.64.0
	google.golang.org/protobuf v1.34.1
	gopkg.in/natefinch/lumberjack.v2 v2.2.1
	oss.terrastruct.com/d2 v0.5.1
	software.sslmate.com/src/go-pkcs12 v0.2.0
)

require (
	cdr.dev/slog v1.4.2-0.20221206192828-e4803b10ae17 // indirect
	cloud.google.com/go/logging v1.9.0 // indirect
	cloud.google.com/go/longrunning v0.5.4 // indirect
	dario.cat/mergo v1.0.0 // indirect
	github.com/Microsoft/go-winio v0.6.1 // indirect
	github.com/ProtonMail/go-crypto v0.0.0-20230828082145-3c4c8a2d2371 // indirect
	github.com/PuerkitoBio/goquery v1.8.0 // indirect
	github.com/agext/levenshtein v1.2.3 // indirect
	github.com/alecthomas/chroma v0.10.0 // indirect
	github.com/andybalholm/cascadia v1.3.1 // indirect
	github.com/anmitsu/go-shlex v0.0.0-20200514113438-38f4b401e2be // indirect
	github.com/apparentlymart/go-textseg/v13 v13.0.0 // indirect
	github.com/bytedance/sonic v1.9.2 // indirect
	github.com/chenzhuoyu/base64x v0.0.0-20221115062448-fe3a3abad311 // indirect
	github.com/cloudflare/circl v1.3.7 // indirect
	github.com/cyphar/filepath-securejoin v0.2.4 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/dlclark/regexp2 v1.11.0 // indirect
	github.com/dop251/goja v0.0.0-20230122112309-96b1610dd4f7 // indirect
	github.com/emirpasic/gods v1.18.1 // indirect
	github.com/fatih/color v1.13.0 // indirect
	github.com/gabriel-vasile/mimetype v1.4.2 // indirect
	github.com/gdamore/encoding v1.0.0 // indirect
	github.com/gebv/pikchr v1.0.2 // indirect
	github.com/gin-contrib/sse v0.1.0 // indirect
	github.com/go-git/gcfg v1.5.1-0.20230307220236-3a3c6141e376 // indirect
	github.com/go-git/go-billy/v5 v5.5.0 // indirect
	github.com/go-playground/locales v0.14.1 // indirect
	github.com/go-playground/universal-translator v0.18.1 // indirect
	github.com/go-playground/validator/v10 v10.14.1 // indirect
	github.com/go-sourcemap/sourcemap v2.1.3+incompatible // indirect
	github.com/goccy/go-json v0.10.2 // indirect
	github.com/golang-sql/civil v0.0.0-20220223132316-b832511892a9 // indirect
	github.com/golang-sql/sqlexp v0.1.0 // indirect
	github.com/golang/freetype v0.0.0-20170609003504-e2365dfdc4a0 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/jbenet/go-context v0.0.0-20150711004518-d14ea06fba99 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/kardianos/osext v0.0.0-20190222173326-2bc1f35cddc0 // indirect
	github.com/kevinburke/ssh_config v1.2.0 // indirect
	github.com/klauspost/compress v1.17.2 // indirect
	github.com/klauspost/cpuid/v2 v2.2.5 // indirect
	github.com/leodido/go-urn v1.2.4 // indirect
	github.com/lucasb-eyer/go-colorful v1.2.0 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mattn/go-runewidth v0.0.15 // indirect
	github.com/mattn/go-tty v0.0.5 // indirect
	github.com/mazznoer/csscolorparser v0.1.3 // indirect
	github.com/mitchellh/go-wordwrap v1.0.1 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/nats-io/nkeys v0.4.7 // indirect
	github.com/nats-io/nuid v1.0.1 // indirect
	github.com/orcaman/concurrent-map/v2 v2.0.1 // indirect
	github.com/pelletier/go-toml/v2 v2.0.8 // indirect
	github.com/pjbgf/sha1cd v0.3.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/rivo/uniseg v0.4.4 // indirect
	github.com/sergi/go-diff v1.2.0 // indirect
	github.com/skeema/knownhosts v1.2.1 // indirect
	github.com/sony/sonyflake v1.2.0 // indirect
	github.com/tidwall/match v1.1.1 // indirect
	github.com/tidwall/pretty v1.2.1 // indirect
	github.com/twitchyliquid64/golang-asm v0.15.1 // indirect
	github.com/ugorji/go/codec v1.2.11 // indirect
	github.com/xanzy/ssh-agent v0.3.3 // indirect
	go.opencensus.io v0.24.0 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	golang.org/x/arch v0.3.0 // indirect
	golang.org/x/image v0.11.0 // indirect
	golang.org/x/mod v0.17.0 // indirect
	golang.org/x/sync v0.7.0 // indirect
	golang.org/x/tools v0.21.1-0.20240508182429-e35e4ccd0d2d // indirect
	golang.org/x/xerrors v0.0.0-20220907171357-04be3eba64a2 // indirect
	gonum.org/v1/plot v0.12.0 // indirect
	google.golang.org/genproto v0.0.0-20231016165738-49dd2c1f3d0b // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240604185151-ef581f913117 // indirect
	gopkg.in/warnings.v0 v0.1.2 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	oss.terrastruct.com/util-go v0.0.0-20230604222829-11c3c60fec14 // indirect
)
