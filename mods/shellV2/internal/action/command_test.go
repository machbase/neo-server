package action

import (
	"fmt"
	"testing"
)

func TestFlagSpec(t *testing.T) {
	tests := []struct {
		flag   *CmdFlag
		expect string
	}{
		{
			&CmdFlag{Long: "name", Short: "n", Placeholder: "tag", Default: "value", Description: "tag name"},
			"  -n,--name <tag>        tag name (default:value)",
		},
		{
			&CmdFlag{Long: "name", Short: "n", Placeholder: "tag", Default: "value", Description: "multi\nlined\ndesc"},
			"  -n,--name <tag>        multi\n                         lined\n                         desc (default:value)",
		},
	}

	for _, tt := range tests {
		result := tt.flag.Format(2, 25)
		if result != tt.expect {
			t.Logf("wrong flag format %q, expect %q", result, tt.expect)
			t.Fail()
		}
	}
}

func TestArgSpec(t *testing.T) {
	tests := []struct {
		arg    *CmdArg
		expect string
	}{
		{
			&CmdArg{Name: "name", Default: "value", Description: "tag name"},
			"  name                   tag name (default:value)",
		},
		{
			&CmdArg{Name: "name", Default: "value", Description: "multi\nlined\ndesc"},
			"  name                   multi\n                         lined\n                         desc (default:value)",
		},
	}

	for _, tt := range tests {
		result := tt.arg.Format(2, 25)
		if result != tt.expect {
			t.Logf("wrong arg format %q, expect %q", result, tt.expect)
			t.Fail()
		}
	}
}

func TestCmdSpec(t *testing.T) {
	tests := []struct {
		cmd    *CmdSpec
		expect string
	}{
		{
			&CmdSpec{
				Syntax:      "hello commands [options] <name>",
				Description: "Say hello to system.\nThis is demo",
				Detail: `Some
				        |Detail examples`,
				SubCmds: []*CmdSpec{
					{Syntax: "list", Description: "show all greeting messages"},
				},
				Args: []*CmdArg{
					{Name: "name", Default: `"value"`, Description: "tag name"},
				},
				Flags: []*CmdFlag{
					{Long: "name", Short: "n", Placeholder: "tag", Default: "value", Description: "tag name"},
					{Short: "f", Description: "force"},
				},
			},
			`|    hello commands [options] <name>
			 |                          Say hello to system.
			 |                          This is demo
			 |      commands:
			 |        list              show all greeting messages
			 |      arguments:
			 |        name              tag name (default:"value")
			 |      options:
			 |        -n,--name <tag>   tag name (default:value)
			 |        -f                force
			 |      Some
			 |      Detail examples`,
		},
	}

	for _, tt := range tests {
		result := tt.cmd.String()
		expect := TrimMultiLines(tt.expect)
		if result != expect {
			t.Logf("wrong cmd format")
			fmt.Println("===result====")
			fmt.Println(result)
			fmt.Println("===expect====")
			fmt.Println(expect)

			t.Fail()
		}
	}
}
