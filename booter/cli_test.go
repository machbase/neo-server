package booter_test

import (
	"testing"

	"github.com/machbase/neo-server/v8/booter"
	"github.com/stretchr/testify/require"
)

func TestFlagParser(t *testing.T) {
	parser := booter.NewCommandLineParser([]string{
		"--strlongsep", `"strval"`,
		"--strlong='str val'",
		"-s", "strval",
		"--intval=1234",
		"--exists",
		"ARG1",
		"--no-daemon=true",
		"--no-reverse=false",
		"--flag=true",
		"ARG2",
		"--", "pass1", "pass2",
	})
	parser.AddHintBool("exists", "", false)
	parser.AddHintBool("daemon", "", true)
	parser.AddHintBool("reverse", "", true)
	ctx, err := parser.Parse()
	if err != nil {
		panic(err)
	}
	require.Equal(t, "strval", ctx.Flag("strlongsep", "").String(""))
	require.Equal(t, "str val", ctx.Flag("strlong", "").String(""))
	require.Equal(t, "strval", ctx.Flag("", "s").String(""))
	require.Equal(t, 1234, ctx.Flag("intval", "").Int(0))
	require.Equal(t, "true", ctx.Flag("flag", "").String(""))
	require.Equal(t, true, ctx.Flag("reverse", "").Bool(false))
	require.Equal(t, false, ctx.Flag("daemon", "").Bool(true))
	require.Equal(t, 2, len(ctx.Args()))
	require.Equal(t, true, ctx.Flag("exists", "").Bool(false))
	require.Equal(t, "ARG1", ctx.Args()[0])
	require.Equal(t, "ARG2", ctx.Args()[1])

	require.Equal(t, 2, len(ctx.Passthrough()))
}

func TestFlagParserDeamon(t *testing.T) {
	parser := booter.NewCommandLineParser([]string{
		"serve",
		"--daemon",
		"--pid", "./tmp/pid.txt",
		"--bootlog", "./tmp/boot.log",
		"--log-filename=./tmp/machbase.log",
	})
	parser.AddHintBool("daemon", "d", false)
	parser.AddHintBool("help", "h", false)
	ctx, err := parser.Parse()
	if err != nil {
		panic(err)
	}
	require.Equal(t, true, ctx.Flag("daemon", "").Bool(false))
	require.Equal(t, "./tmp/pid.txt", ctx.Flag("pid", "").String(""))
	require.Equal(t, "./tmp/boot.log", ctx.Flag("bootlog", "s").String(""))
	require.Equal(t, "./tmp/machbase.log", ctx.Flag("log-filename", "").String(""))
}

func TestFlagParserDeamon2(t *testing.T) {
	parser := booter.NewCommandLineParser([]string{
		"serve",
		"--daemon",
	})
	parser.AddHintBool("daemon", "d", false)
	parser.AddHintBool("help", "h", false)
	ctx, err := parser.Parse()
	if err != nil {
		panic(err)
	}
	require.Equal(t, true, ctx.Flag("daemon", "").Bool(false))
}
