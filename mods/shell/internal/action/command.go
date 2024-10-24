package action

import (
	"strings"
)

type Cmd struct {
	Name  string
	Desc  string
	Usage string
	Spec  *CmdSpec

	PcFunc func() PrefixCompleterInterface
	Action func(ctx *ActionContext)
	// if the Cmd is the client side action
	ClientAction bool
	// if the Cmd is an experimental feature
	Experimental bool
	// Deprecated
	Deprecated        bool
	DeprecatedMessage string
}

type CmdSpec struct {
	Syntax      string     `json:"syntax"`
	SubCmds     []*CmdSpec `json:"commands"`
	Args        []*CmdArg  `json:"args"`
	Flags       []*CmdFlag `json:"options"`
	Description string     `json:"description"`
	Detail      string     `json:"detail"`
}

func (cmd *CmdSpec) String() string {
	return cmd.Format(2, 0)
}

func (cmd *CmdSpec) fitHead(leadWidth int) int {
	maxHead := 12
	for _, itm := range cmd.Args {
		width := len(itm.FormatHead(leadWidth))
		if width > maxHead {
			maxHead = width
		}
	}
	for _, itm := range cmd.Flags {
		width := len(itm.FormatHead(leadWidth))
		if width > maxHead {
			maxHead = width
		}
	}
	return maxHead
}

func (cmd *CmdSpec) Format(leadWidth int, headWidth int) string {
	leadWidth += 2
	if w := cmd.fitHead(leadWidth + 4); w > headWidth {
		headWidth = w
	}

	sb := &strings.Builder{}

	sb.WriteString(strings.Repeat(" ", leadWidth))
	sb.WriteString(cmd.Syntax)

	if cmd.Description != "" {
		lines := strings.Split(cmd.Description, "\n")
		for i, l := range lines {
			if i == 0 {
				if headWidth-sb.Len() > 0 {
					sb.WriteString(strings.Repeat(" ", headWidth-sb.Len()))
				} else {
					sb.WriteString("\n")
					sb.WriteString(strings.Repeat(" ", headWidth))
				}
			} else {
				sb.WriteString(strings.Repeat(" ", headWidth))
			}
			sb.WriteString(l)
			sb.WriteString("\n")
		}
	} else {
		sb.WriteString("\n")
	}

	if len(cmd.SubCmds) > 0 {
		sb.WriteString(strings.Repeat(" ", leadWidth))
		sb.WriteString("  commands:\n")
	}
	for _, s := range cmd.SubCmds {
		sb.WriteString(s.Format(leadWidth+2, headWidth))
	}

	if len(cmd.Args) > 0 {
		sb.WriteString(strings.Repeat(" ", leadWidth))
		sb.WriteString("  arguments:\n")
	}
	for _, itm := range cmd.Args {
		sb.WriteString(itm.Format(leadWidth+4, headWidth))
		sb.WriteString("\n")
	}

	if len(cmd.Flags) > 0 {
		sb.WriteString(strings.Repeat(" ", leadWidth))
		sb.WriteString("  options:\n")
	}
	for _, itm := range cmd.Flags {
		sb.WriteString(itm.Format(leadWidth+4, headWidth))
		sb.WriteString("\n")
	}
	if cmd.Detail != "" {
		lines := strings.Split(TrimMultiLines(cmd.Detail), "\n")
		for i, l := range lines {
			if i == len(lines)-1 && len(l) == 0 {
				break
			}
			sb.WriteString(strings.Repeat(" ", leadWidth+2))
			sb.WriteString(l)
			if i < len(lines)-1 || strings.HasSuffix(cmd.Detail, "\n") {
				sb.WriteString("\n")
			}
		}
	}
	return sb.String()
}

type CmdArg struct {
	Name        string `json:"name"`
	Default     string `json:"default"`
	Description string `json:"description"`
}

func (arg *CmdArg) FormatHead(leadWidth int) string {
	sb := &strings.Builder{}
	if leadWidth > 0 {
		sb.WriteString(strings.Repeat(" ", leadWidth))
	}
	sb.WriteString(arg.Name)
	sb.WriteString("  ")
	return sb.String()
}

func (arg *CmdArg) Format(leadWidth int, headWidth int) string {
	sb := &strings.Builder{}
	sb.WriteString(arg.FormatHead(leadWidth))
	if sb.Len() < headWidth {
		sb.WriteString(strings.Repeat(" ", headWidth-sb.Len()))
	}
	if arg.Description != "" {
		lines := strings.Split(arg.Description, "\n")
		offset := sb.Len()
		for i, l := range lines {
			if i > 0 {
				sb.WriteString(strings.Repeat(" ", offset))
			}
			sb.WriteString(l)
			if i < len(lines)-1 {
				sb.WriteString("\n")
			}
		}
	}
	if arg.Default != "" {
		if arg.Description != "" {
			sb.WriteString(" ")
		}
		sb.WriteString("(default:" + arg.Default + ")")
	}
	return sb.String()
}

type CmdFlag struct {
	Long        string `json:"long"`
	Short       string `json:"short"`
	Placeholder string `json:"placeholder"`
	Default     string `json:"default"`
	Description string `json:"description"`
}

func (flag *CmdFlag) FormatHead(leadWidth int) string {
	sb := &strings.Builder{}
	if leadWidth > 0 {
		sb.WriteString(strings.Repeat(" ", leadWidth))
	}
	if flag.Short != "" {
		sb.WriteString("-" + flag.Short)
		if flag.Long != "" {
			sb.WriteString(",")
		}
	} else {
		sb.WriteString("   ")
	}
	if flag.Long != "" {
		sb.WriteString("--" + flag.Long + " ")
	} else {
		sb.WriteString(" ")
	}
	if flag.Placeholder != "" {
		sb.WriteString("<" + flag.Placeholder + "> ")
	}
	sb.WriteString("  ")
	return sb.String()
}

func (flag *CmdFlag) Format(leadWidth int, headWidth int) string {
	sb := &strings.Builder{}
	sb.WriteString(flag.FormatHead(leadWidth))
	if sb.Len() < headWidth {
		sb.WriteString(strings.Repeat(" ", headWidth-sb.Len()))
	}
	if flag.Description != "" {
		lines := strings.Split(flag.Description, "\n")
		offset := sb.Len()
		for i, l := range lines {
			if i > 0 {
				sb.WriteString(strings.Repeat(" ", offset))
			}
			sb.WriteString(l)
			if i < len(lines)-1 {
				sb.WriteString("\n")
			}
		}
	}
	if flag.Default != "" {
		if flag.Description != "" {
			sb.WriteString(" ")
		}
		sb.WriteString("(default:" + flag.Default + ")")
	}
	return sb.String()
}

func TrimMultiLines(str string) string {
	sb := &strings.Builder{}
	for _, line := range strings.Split(str, "\n") {
		idx := strings.Index(line, "|")
		if idx >= 0 {
			line = strings.ReplaceAll(line[idx+1:], "\t", "    ")
		} else {
			line = strings.ReplaceAll(line, "\t", "    ")
		}
		sb.WriteString(line)
		sb.WriteString("\n")
	}
	return sb.String()
}
