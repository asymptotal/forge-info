// Command forge-info-mcp serves a file-backed knowledge store over MCP.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime/debug"

	"github.com/asymptotal/forge-info/internal/server"
	"github.com/asymptotal/forge-info/internal/store"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// version is set by release builds with -ldflags "-X main.version=…".
var version = ""

// getVersion returns the release version, else the module version recorded
// by go install, else "dev".
func getVersion() string {
	if version != "" {
		return version
	}
	if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		return bi.Main.Version
	}
	return "dev"
}

const usage = `usage:
  forge-info-mcp serve [--root <dir>]   serve the store over MCP on stdio (root: the .info dir)
  forge-info-mcp init [<dir>]           create <dir>/.info (default: current directory)
  forge-info-mcp version                print the version
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	var err error
	switch args[0] {
	case "serve":
		err = serve(args[1:], stdin, stdout, stderr)
	case "init":
		err = initCmd(args[1:], stderr)
	case "version":
		if len(args) > 1 {
			err = fmt.Errorf("version takes no arguments")
		} else {
			fmt.Fprintln(stdout, getVersion())
		}
	default:
		fmt.Fprintf(stderr, "forge-info-mcp: unknown command %q\n%s", args[0], usage)
		return 2
	}
	if err != nil {
		fmt.Fprintf(stderr, "forge-info-mcp: %v\n", err)
		return 2
	}
	return 0
}

func initCmd(args []string, stderr io.Writer) error {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 1 {
		return fmt.Errorf("init takes at most one directory")
	}
	dir := "."
	if fs.NArg() == 1 {
		dir = fs.Arg(0)
	}
	if err := store.Init(dir); err != nil {
		return err
	}
	fmt.Fprintf(stderr, "created %s\n", dir+string(os.PathSeparator)+".info")
	return nil
}

func serve(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("root", "", "the .info directory (default: nearest .info above the working directory)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("serve takes no arguments")
	}
	if *root == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		if *root, err = store.Find(cwd); err != nil {
			return err
		}
	}
	st, err := store.Open(*root)
	if err != nil {
		return err
	}
	srv, err := server.New(st, getVersion())
	if err != nil {
		return err
	}
	var t mcp.Transport = &mcp.IOTransport{Reader: io.NopCloser(stdin), Writer: nopWriteCloser{stdout}}
	if stdin == os.Stdin && stdout == os.Stdout {
		t = &mcp.StdioTransport{}
	}
	return srv.Run(context.Background(), t)
}

type nopWriteCloser struct{ io.Writer }

func (nopWriteCloser) Close() error { return nil }
