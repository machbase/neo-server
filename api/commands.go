package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strings"

	"github.com/spf13/cobra"
)

type CommandHandler struct {
	Database      func(context.Context) (Conn, error)
	SilenceUsage  bool
	SilenceErrors bool
	FallbackVerb  string
	OutWriter     io.Writer
	ErrWriter     io.Writer

	PreExecute  func(args []string)
	PostExecute func(args []string, message string, err error)

	DescribeTable   func(*TableDescription)
	ShowTables      func(*TableInfo, int64) bool
	ShowIndexes     func(*IndexInfo, int64) bool
	ShowIndex       func(*IndexInfo) bool
	ShowLsmIndexes  func(*LsmIndexInfo, int64) bool
	ShowTags        func(*TagInfo, int64) bool
	ShowRollupGap   func(*RollupGapInfo, int64) bool
	ShowIndexGap    func(*IndexGapInfo, int64) bool
	ShowTagIndexGap func(*IndexGapInfo, int64) bool
	ShowSessions    func(*SessionInfo, int64) bool
	ShowStatements  func(*StatementInfo, int64) bool
	ShowStorage     func(*StorageInfo, int64) bool
	ShowTableUsage  func(*TableUsageInfo, int64) bool
	ShowLicense     func(*LicenseInfo) bool
	Explain         func(string, error)
	SqlQuery        func(*Query, int64) bool

	params []any
}

func NewCommandHandler() *CommandHandler {
	return &CommandHandler{
		FallbackVerb:  "sql --",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
}

func (ch *CommandHandler) MakeCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "machbase-neo",
		Short: "machbase-neo commands",
	}

	if sc := ch.NewShowCommand(); sc != nil {
		cmd.AddCommand(sc)
	}
	if sc := ch.NewDescribeCommand(); sc != nil {
		cmd.AddCommand(sc)
	}
	if sc := ch.NewExplainCommand(); sc != nil {
		cmd.AddCommand(sc)
	}
	if sc := ch.NewSqlCommand(); sc != nil {
		cmd.AddCommand(sc)
	}
	cmd.SilenceUsage = ch.SilenceUsage
	cmd.SilenceErrors = ch.SilenceErrors
	cmd.SetErr(ch.ErrWriter)
	cmd.SetOut(ch.OutWriter)
	return cmd
}

func (ch *CommandHandler) Verbs() []string {
	var verbs []string
	if ch.ShowTables != nil {
		verbs = append(verbs, "show")
	}
	if ch.DescribeTable != nil {
		verbs = append(verbs, "desc")
	}
	if ch.Explain != nil {
		verbs = append(verbs, "explain")
	}
	if ch.SqlQuery != nil {
		verbs = append(verbs, "sql")
	}
	return verbs
}

var ErrCommandNotFound = errors.New("command not found")
var spaces = []rune{' ', '\t', '\n', '\r'}

func ParseCommandLine(commandLine string) []string {
	// Special treatment for the first token 'sql'
	for i, r := range commandLine {
		if slices.Contains(spaces, r) {
			verb := commandLine[:i]
			if verb != "sql" && verb != "explain" {
				break
			}
			tokenizer := &CommandTokenizer{verb: verb, stream: []rune(commandLine[i:])}
			return tokenizer.Tokens()
		}
	}
	// Regular expression to match words or quoted phrases
	re := regexp.MustCompile(`"((?:[^"\\]|\\.)*)"|'((?:[^'\\]|\\.)*)'|(\S+)`)
	matches := re.FindAllStringSubmatch(commandLine, -1)

	var result []string
	for _, match := range matches {
		if match[1] != "" {
			result = append(result, strings.ReplaceAll(match[1], `\"`, `"`))
		} else if match[2] != "" {
			result = append(result, strings.ReplaceAll(match[2], `\'`, `'`))
		} else {
			result = append(result, match[3])
		}
	}
	for i, tok := range result {
		if tok == "--" {
			result = append(result[0:i], strings.Join(result[i+1:], " "))
			break
		}
	}
	return result
}

type CommandTokenizer struct {
	verb   string
	stream []rune
	idx    int
}

var sqlFlags = map[string]bool{
	"-o":           true,
	"--output":     true,
	"-f":           true,
	"--format":     true,
	"--compress":   true,
	"-d":           true,
	"--delimiter":  true,
	"--rownum":     false,
	"--no-rownum":  false,
	"-t":           true,
	"--timeformat": true,
	"--tz":         true,
	"--heading":    false,
	"--no-heading": false,
	"--footer":     false,
	"--no-footer":  false,
	"-p":           true,
	"--precision":  true,
}

var explainFlags = map[string]bool{
	"-f":     false,
	"--full": false,
}

func (p *CommandTokenizer) nextToken() string {
	for p.idx < len(p.stream) && slices.Contains(spaces, p.stream[p.idx]) {
		p.idx++
	}
	if p.idx >= len(p.stream) {
		return ""
	}
	start := p.idx
	for p.idx < len(p.stream) && !slices.Contains(spaces, p.stream[p.idx]) {
		p.idx++
	}
	return string(p.stream[start:p.idx])
}
func (p *CommandTokenizer) Tokens() []string {
	ret := []string{p.verb}
	var flagsTable map[string]bool
	if p.verb == "sql" {
		flagsTable = sqlFlags
	} else if p.verb == "explain" {
		flagsTable = explainFlags
	}
	for tok := p.nextToken(); tok != ""; tok = p.nextToken() {
		if tok == "--" {
			ret = append(ret, strings.TrimSpace(string(p.stream[p.idx:])))
			break
		}
		if requireFlagValue, ok := flagsTable[tok]; ok {
			ret = append(ret, tok)
			if requireFlagValue {
				ret = append(ret, p.nextToken())
			}
		} else {
			// trim spaces and single and double quotes from remains of the stream
			remaining := tok + " " + strings.Trim(string(p.stream[p.idx:]), " \t\n\r")
			if strings.HasPrefix(remaining, "'") && strings.HasSuffix(remaining, "'") {
				remaining = remaining[1 : len(remaining)-1]
				remaining = strings.ReplaceAll(remaining, `\'`, `'`)
			} else if strings.HasPrefix(remaining, "\"") && strings.HasSuffix(remaining, "\"") {
				remaining = remaining[1 : len(remaining)-1]
				remaining = strings.ReplaceAll(remaining, `\"`, `"`)
			}
			ret = append(ret, remaining)
			break
		}
	}
	return ret
}

func (ch *CommandHandler) IsKnownVerb(v string) bool {
	return slices.Contains(ch.Verbs(), v)
}

func (ch *CommandHandler) Exec(ctx context.Context, args []string, params ...any) error {
	cmd := ch.MakeCommand()
	cmd.SetArgs(args)
	ch.params = params
	if ch.PreExecute != nil {
		ch.PreExecute(args)
	}
	err := cmd.ExecuteContext(ctx)
	if ch.PostExecute != nil {
		ch.PostExecute(args, "", err)
	}
	return err
}

func (ch *CommandHandler) NewShowCommand() *cobra.Command {
	showCmd := &cobra.Command{
		Use: "show",
	}

	if ch.ShowTables != nil {
		showTables := &cobra.Command{
			Use:  "tables [-a]",
			Args: cobra.NoArgs,
		}
		showTablesAll := showTables.Flags().BoolP("all", "a", false, "show all tables")
		showTables.RunE = ch.runShowTables(showTablesAll)
		showCmd.AddCommand(showTables)
	}

	if ch.ShowIndexes != nil {
		showIndexes := &cobra.Command{
			Use:  "indexes",
			Args: cobra.NoArgs,
		}
		showIndexes.RunE = ch.runShowIndexes()
		showCmd.AddCommand(showIndexes)
	}

	if ch.ShowIndex != nil {
		showIndex := &cobra.Command{
			Use:  "index <index_name>",
			Args: cobra.ExactArgs(1),
		}
		showIndex.RunE = ch.runShowIndex
		showCmd.AddCommand(showIndex)
	}

	if ch.ShowLsmIndexes != nil {
		showLsmIndexes := &cobra.Command{
			Use:  "lsm",
			Args: cobra.NoArgs,
		}
		showLsmIndexes.RunE = ch.runShowLsm
		showCmd.AddCommand(showLsmIndexes)
	}

	if ch.DescribeTable != nil {
		showTable := &cobra.Command{
			Use:  "table [-a] <table_name>",
			Args: cobra.ExactArgs(1),
		}
		descTableAll := showTable.Flags().BoolP("all", "a", false, "describe all columns")
		showTable.RunE = ch.runDescTable(descTableAll)
		showCmd.AddCommand(showTable)
	}

	if ch.ShowTags != nil {
		showTags := &cobra.Command{
			Use:  "tags <table_name>",
			Args: cobra.ExactArgs(1),
		}
		showTags.RunE = ch.runShowTags
		showCmd.AddCommand(showTags)
	}

	if ch.ShowIndexGap != nil {
		showIndexGap := &cobra.Command{
			Use:  "indexgap",
			Args: cobra.NoArgs,
		}
		showIndexGap.RunE = ch.runShowIndexGap
		showCmd.AddCommand(showIndexGap)
	}

	if ch.ShowTagIndexGap != nil {
		showTagIndexGap := &cobra.Command{
			Use:  "tagindexgap",
			Args: cobra.NoArgs,
		}
		showTagIndexGap.RunE = ch.runShowTagIndexGap
		showCmd.AddCommand(showTagIndexGap)
	}
	if ch.ShowRollupGap != nil {
		showRollupGap := &cobra.Command{
			Use:  "rollupgap",
			Args: cobra.NoArgs,
		}
		showRollupGap.RunE = ch.runShowRollupGap
		showCmd.AddCommand(showRollupGap)
	}

	if ch.ShowSessions != nil {
		showSessions := &cobra.Command{
			Use:  "sessions",
			Args: cobra.NoArgs,
		}
		showSessions.RunE = ch.runShowSessions
		showCmd.AddCommand(showSessions)
	}

	if ch.ShowStatements != nil {
		showStatements := &cobra.Command{
			Use:  "statements",
			Args: cobra.NoArgs,
		}
		showStatements.RunE = ch.runShowStatements
		showCmd.AddCommand(showStatements)
	}

	if ch.ShowStorage != nil {
		showStorage := &cobra.Command{
			Use:  "storage",
			Args: cobra.NoArgs,
		}
		showStorage.RunE = ch.runShowStorage
		showCmd.AddCommand(showStorage)
	}

	if ch.ShowTableUsage != nil {
		showTableUsage := &cobra.Command{
			Use:  "table-usage",
			Args: cobra.NoArgs,
		}
		showTableUsage.RunE = ch.runShowTableUsage
		showCmd.AddCommand(showTableUsage)
	}

	if ch.ShowLicense != nil {
		showLicense := &cobra.Command{
			Use:  "license",
			Args: cobra.NoArgs,
		}
		showLicense.RunE = ch.runShowLicense
		showCmd.AddCommand(showLicense)
	}

	if showCmd.HasSubCommands() {
		showCmd.SilenceUsage = true
		showCmd.SilenceErrors = true
		return showCmd
	} else {
		return nil
	}
}

func (ch *CommandHandler) NewDescribeCommand() *cobra.Command {
	if ch.DescribeTable == nil {
		return nil
	}
	descCmd := &cobra.Command{
		Use:   "desc [-a] <table_name>",
		Short: "describe table",
	}
	all := descCmd.Flags().BoolP("all", "a", false, "describe all columns")
	descCmd.RunE = ch.runDescTable(all)
	return descCmd
}

func (ch *CommandHandler) NewExplainCommand() *cobra.Command {
	if ch.Explain == nil {
		return nil
	}
	explainCmd := &cobra.Command{
		Use:   "explain [full] <query>",
		Short: "explain query",
		Args:  cobra.MinimumNArgs(1),
	}
	explainCmd.RunE = ch.runExplain()
	return explainCmd
}

func (ch *CommandHandler) NewSqlCommand() *cobra.Command {
	if ch.SqlQuery == nil {
		return nil
	}
	sqlCmd := &cobra.Command{
		Use:                "sql <sql_text>",
		Short:              "execute sql",
		Args:               cobra.MinimumNArgs(1),
		FParseErrWhitelist: cobra.FParseErrWhitelist{UnknownFlags: true},
	}
	opt := SqlCommandOptions{}
	opt.pBridge = sqlCmd.Flags().StringP("bridge", "b", "", "bridge name")
	opt.pOutput = sqlCmd.Flags().StringP("output", "o", "", "output file (default:'-' stdout)")
	opt.pFormat = sqlCmd.Flags().StringP("format", "f", "box", "output format (box,csv,json, default:box)")
	opt.pCompress = sqlCmd.Flags().String("compress", "", "compression method [gzip] (default is not compressed)")
	opt.pDelimiter = sqlCmd.Flags().StringP("delimiter", "d", ",", "csv delimiter (default:',')")
	opt.pRownum = sqlCmd.Flags().Bool("rownum", true, "include rownum as first column (default:true)")
	opt.pTimeformat = sqlCmd.Flags().StringP("timeformat", "t", "", "time format [ns|ms|s|<timeformat>] (default:'default')")
	opt.pTz = sqlCmd.Flags().String("tz", "", "timezone for handling datetime")
	opt.pHeading = sqlCmd.Flags().Bool("heading", true, "print header")
	opt.pFooter = sqlCmd.Flags().Bool("footer", true, "print footer message")
	opt.pPrecision = sqlCmd.Flags().IntP("precision", "p", -1, "set precision of float value to force round")
	sqlCmd.RunE = ch.runSql(opt)
	return sqlCmd
}

func (ch *CommandHandler) runShowTables(showAll *bool) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		conn, err := ch.Database(ctx)
		if err != nil {
			return err
		}
		defer conn.Close()
		nrow := int64(0)
		ListTablesWalk(ctx, conn, *showAll, func(ti *TableInfo) bool {
			nrow++
			return ch.ShowTables(ti, nrow)
		})
		return nil
	}
}

func (ch *CommandHandler) runShowIndexes() func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		conn, err := ch.Database(ctx)
		if err != nil {
			return err
		}
		defer conn.Close()
		nrow := int64(0)
		ListIndexesWalk(ctx, conn, func(ii *IndexInfo) bool {
			nrow++
			return ch.ShowIndexes(ii, nrow)
		})
		return nil
	}
}

func (ch *CommandHandler) runShowIndex(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	conn, err := ch.Database(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	ii, err := DescribeIndex(ctx, conn, args[0])
	if err != nil {
		return err
	}
	ch.ShowIndex(ii)
	return nil
}

func (ch *CommandHandler) runShowLsm(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	conn, err := ch.Database(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	rownum := int64(0)
	ListLsmIndexesWalk(ctx, conn, func(lii *LsmIndexInfo) bool {
		if err = lii.err; err != nil {
			return false
		}
		rownum++
		return ch.ShowLsmIndexes(lii, rownum)
	})
	return err
}

func (ch *CommandHandler) runShowStorage(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	conn, err := ch.Database(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	rownum := int64(0)
	ListStorageWalk(ctx, conn, func(si *StorageInfo) bool {
		if err = si.err; err != nil {
			return false
		}
		rownum++
		return ch.ShowStorage(si, rownum)
	})
	return err
}

func (ch *CommandHandler) runShowTableUsage(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	conn, err := ch.Database(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	rownum := int64(0)
	ListTableUsageWalk(ctx, conn, func(tui *TableUsageInfo) bool {
		if err = tui.err; err != nil {
			return false
		}
		rownum++
		return ch.ShowTableUsage(tui, rownum)
	})
	return err
}

func (ch *CommandHandler) runDescTable(showAll *bool) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		var conn Conn
		if c, err := ch.Database(ctx); err != nil {
			return err
		} else {
			conn = c
		}
		defer conn.Close()

		desc, err := DescribeTable(ctx, conn, args[0], *showAll)
		if err != nil {
			return err
		}
		ch.DescribeTable(desc)
		return nil
	}
}

func (ch *CommandHandler) runShowTags(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	var conn Conn
	if c, err := ch.Database(ctx); err != nil {
		return err
	} else {
		conn = c
	}
	defer conn.Close()

	tableName := strings.ToUpper(args[0])
	desc, err := DescribeTable(ctx, conn, tableName, false)
	if err != nil {
		return err
	}
	if desc.Type != TableTypeTag {
		return fmt.Errorf("table '%s' is not a tag table", tableName)
	}
	summarized := false
	for _, c := range desc.Columns {
		if c.Flag&ColumnFlagSummarized > 0 {
			summarized = true
			break
		}
	}
	nrow := int64(0)
	ListTagsWalk(ctx, conn, tableName, func(tag *TagInfo) bool {
		if err = tag.Err; err != nil {
			return false
		}
		tag.Summarized = summarized
		if stat, err := TagStat(ctx, conn, tableName, tag.Name); err != nil {
			// some tags may not have stat
			// the err may be 'no rows in result set'
			// ignore the error, for processing the next tag
		} else {
			tag.Stat = stat
		}
		nrow++
		return ch.ShowTags(tag, nrow)
	})
	return err
}

func (ch *CommandHandler) runShowIndexGap(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	conn, err := ch.Database(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	nrow := int64(0)
	ListIndexGapWalk(ctx, conn, func(gap *IndexGapInfo) bool {
		if err = gap.err; err != nil {
			return false
		}
		nrow++
		return ch.ShowIndexGap(gap, nrow)
	})
	return err
}

func (ch *CommandHandler) runShowTagIndexGap(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	conn, err := ch.Database(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	nrow := int64(0)
	ListTagIndexGapWalk(ctx, conn, func(gap *IndexGapInfo) bool {
		if err = gap.err; err != nil {
			return false
		}
		nrow++
		return ch.ShowTagIndexGap(gap, nrow)
	})
	return err
}

func (ch *CommandHandler) runShowRollupGap(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	conn, err := ch.Database(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	rownum := int64(0)
	ListRollupGapWalk(ctx, conn, func(gap *RollupGapInfo) bool {
		if err = gap.err; err != nil {
			return false
		}
		rownum++
		return ch.ShowRollupGap(gap, rownum)
	})
	return err
}

func (ch *CommandHandler) runShowSessions(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	conn, err := ch.Database(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	nrow := int64(0)
	ListSessionsWalk(ctx, conn, func(si *SessionInfo) bool {
		if err = si.err; err != nil {
			return false
		}
		nrow++
		return ch.ShowSessions(si, nrow)
	})
	return err
}

func (ch *CommandHandler) runShowStatements(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	conn, err := ch.Database(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	nrow := int64(0)
	ListStatementsWalk(ctx, conn, func(nfo *StatementInfo) bool {
		if err = nfo.err; err != nil {
			return false
		}
		nrow++
		return ch.ShowStatements(nfo, nrow)
	})
	return err
}

func (ch *CommandHandler) runExplain() func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		if ch.Explain == nil {
			return errors.New("handler .Explain not set")
		}
		ctx := cmd.Context()
		var conn Conn
		if c, err := ch.Database(ctx); err != nil {
			return err
		} else {
			conn = c
		}
		defer conn.Close()

		var fullRegex = regexp.MustCompile(`(?i)^full\s+`)
		var full = false
		if fullRegex.MatchString(args[0]) {
			// it allows to use 'explain full select...' as well
			args[0] = args[0][len("full"):]
			full = true
		}
		plan, err := conn.Explain(ctx, strings.Join(args, " "), full)
		ch.Explain(plan, err)
		return nil
	}
}

type SqlCommandOptions struct {
	pBridge *string
	// options for sink
	pOutput     *string
	pFormat     *string
	pCompress   *string
	pDelimiter  *string
	pRownum     *bool
	pTimeformat *string
	pTz         *string
	pHeading    *bool
	pFooter     *bool
	pPrecision  *int
}

func (ch *CommandHandler) runSql(opt SqlCommandOptions) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		var conn Conn
		if *opt.pBridge == "" {
			if c, err := ch.Database(ctx); err != nil {
				return err
			} else {
				conn = c
			}
		} else {
			return errors.New("bridge not supported")
		}
		defer conn.Close()
		query := &Query{
			Begin: func(q *Query) {
				ch.SqlQuery(q, 0)
			},
			End: func(q *Query) {
				ch.SqlQuery(q, -1)
			},
			Next: func(q *Query, nrow int64) bool {
				return ch.SqlQuery(q, nrow)
			},
		}
		sqlText := args[len(args)-1]
		if err := query.Execute(ctx, conn, sqlText, ch.params...); err != nil {
			return err
		}
		return nil
	}
}

func (ch *CommandHandler) runShowLicense(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	var conn Conn
	if c, err := ch.Database(ctx); err != nil {
		return err
	} else {
		conn = c
	}
	defer conn.Close()
	nfo, err := GetLicenseInfo(ctx, conn)
	if err != nil {
		return err
	}
	ch.ShowLicense(nfo)
	return nil
}
