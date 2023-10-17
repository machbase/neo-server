package client

import (
	"fmt"
	"os"
)

func (cli *client) Write(p []byte) (int, error) {
	return os.Stdout.Write(p)
}

func (cli *client) Print(args ...any) {
	fmt.Fprint(os.Stdout, args...)
}

func (cli *client) Printf(format string, args ...any) {
	str := fmt.Sprintf(format, args...)
	fmt.Fprint(os.Stdout, str)
}

func (cli *client) Println(args ...any) {
	fmt.Fprintln(os.Stdout, args...)
}

func (cli *client) Printfln(format string, args ...any) {
	fmt.Fprintf(os.Stdout, format+"\r\n", args...)
}
