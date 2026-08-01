// Command dmsn is the damson CLI: an AXI-native client for a damson library,
// built for agents first and people second.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sort"
	"strings"
	"time"

	"github.com/thomas-elliott/damson/internal/axi"
	"github.com/thomas-elliott/damson/internal/toon"
)

const (
	binaryName     = "dmsn"
	defaultTimeout = 30 * time.Second
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	os.Exit(run(ctx, os.Args[1:], os.Stdout))
}

// run is main's testable core: argv in, exit code out, everything written to
// w. Errors are rendered onto the same stream as results — an agent reads one
// stream, and a failure is as much a result as a table.
func run(ctx context.Context, args []string, w io.Writer) int {
	name, rest := split(args)

	if name == "" {
		return dispatch(ctx, homeCommand, rest, w)
	}
	if name == "help" || name == "--help" || name == "-h" {
		writeRootHelp(w)
		return axi.ExitOK
	}
	cmd, ok := lookup(name)
	if !ok {
		return axi.RenderError(w, unknownCommand(name), axi.Options{})
	}
	return dispatch(ctx, cmd, rest, w)
}

func dispatch(ctx context.Context, cmd *command, args []string, w io.Writer) int {
	g := &globals{}
	fs := flag.NewFlagSet(binaryName+" "+cmd.name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {}
	g.register(fs)
	if cmd.flags != nil {
		cmd.flags(fs)
	}

	if err := fs.Parse(permute(fs, args)); err != nil {
		// -h / --help is a request, not a failure: one screen, exit 0.
		if err == flag.ErrHelp {
			writeCommandHelp(w, cmd)
			return axi.ExitOK
		}
		return axi.RenderError(w, axi.Usagef(
			[]string{fmt.Sprintf("%s %s --help", binaryName, cmd.name)},
			"%v", err), g.options())
	}

	env := &Env{Out: w, In: os.Stdin, Globals: g, Args: fs.Args(), FlagSet: fs}
	if err := cmd.run(ctx, env); err != nil {
		return axi.RenderError(w, err, g.options())
	}
	return axi.ExitOK
}

// permute hoists flags ahead of positional arguments. stdlib flag stops
// parsing at the first non-flag argument, which would make `dmsn asset 511
// --json` treat --json as a second id — and putting the id last is the
// natural way to type it. Everything after a bare `--` is left alone.
func permute(fs *flag.FlagSet, args []string) []string {
	var flags, positional []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			positional = append(positional, args[i+1:]...)
			break
		}
		if len(a) < 2 || a[0] != '-' {
			positional = append(positional, a)
			continue
		}
		flags = append(flags, a)
		// A flag with an inline value, or a boolean, is self-contained.
		// Anything else consumes the argument that follows it.
		if strings.Contains(a, "=") {
			continue
		}
		if takesValue(fs, a) && i+1 < len(args) {
			i++
			flags = append(flags, args[i])
		}
	}
	return append(flags, positional...)
}

// takesValue reports whether a flag consumes the next argument. Unknown flags
// are treated as self-contained so flag.Parse reports them, rather than this
// function swallowing an argument on their behalf.
func takesValue(fs *flag.FlagSet, arg string) bool {
	f := fs.Lookup(strings.TrimLeft(arg, "-"))
	if f == nil {
		return false
	}
	b, ok := f.Value.(interface{ IsBoolFlag() bool })
	return !ok || !b.IsBoolFlag()
}

func split(args []string) (string, []string) {
	if len(args) == 0 {
		return "", nil
	}
	// A leading flag means no subcommand was given: `dmsn --json` is the home
	// view with an option, not a command called "--json".
	if strings.HasPrefix(args[0], "-") && args[0] != "-h" && args[0] != "--help" {
		return "", args
	}
	return args[0], args[1:]
}

// Env is what a command receives: where to write, the resolved global flags,
// and the positional arguments left after flag parsing.
type Env struct {
	Out     io.Writer
	In      io.Reader
	Globals *globals
	Args    []string
	// FlagSet is the parsed set, so a command can distinguish "flag set to
	// the empty string" from "flag not given" — the difference between
	// clearing a field and leaving it alone.
	FlagSet *flag.FlagSet
}

// flagWasSet reports whether the named flag appeared on the command line.
func (e *Env) flagWasSet(name string) bool {
	if e.FlagSet == nil {
		return false
	}
	seen := false
	e.FlagSet.Visit(func(f *flag.Flag) {
		if f.Name == name {
			seen = true
		}
	})
	return seen
}

// Record renders a single-entity result through the shared AXI layer, so the
// encoding switch, field projection and help block behave identically on
// every command rather than each one hand-rolling TOON.
func (e *Env) Record(key string, fields []toon.KV, help []string) error {
	return axi.RenderRecord(e.Out, axi.Record{Key: key, Fields: fields}, help, e.Globals.options())
}

// Records renders several named object blocks as one result.
func (e *Env) Records(blocks []Block, help []string) error {
	return axi.RenderBlocks(e.Out, blocks, help, e.Globals.options())
}

// Block is one named object inside a multi-block result.
type Block = axi.Block

// Client resolves credentials and returns a ready client. Resolution order is
// flag, then environment, then the config file — most explicit wins.
func (e *Env) Client() (*Client, error) {
	cfg, err := LoadConfig()
	if err != nil {
		return nil, axi.Wrap(axi.CodeIO, err, "%v", err)
	}
	server := firstNonEmpty(e.Globals.server, os.Getenv("DAMSON_SERVER"), cfg.Server)
	token := firstNonEmpty(e.Globals.token, os.Getenv("DAMSON_TOKEN"), cfg.Token)
	if token == "" {
		return nil, &axi.Error{
			Code:    axi.CodeAuth,
			Message: "no token configured",
			Help: []string{
				fmt.Sprintf("%s login --server <url> --token <token>", binaryName),
				"create a token in the damson SPA under Settings → Access tokens",
			},
		}
	}
	return NewClient(server, token, e.Globals.timeout)
}

// globals are the flags every command accepts, per AXI principle 10.
type globals struct {
	server  string
	token   string
	json    bool
	fields  string
	full    bool
	noHelp  bool
	quiet   bool
	timeout time.Duration
}

func (g *globals) register(fs *flag.FlagSet) {
	fs.StringVar(&g.server, "server", "", "damson server URL")
	fs.StringVar(&g.token, "token", "", "personal access token")
	fs.BoolVar(&g.json, "json", false, "emit JSON instead of TOON")
	fs.StringVar(&g.fields, "fields", "", "comma-separated fields to show")
	fs.BoolVar(&g.full, "full", false, "do not truncate long text")
	fs.BoolVar(&g.noHelp, "no-help", false, "suppress the trailing help block")
	fs.BoolVar(&g.quiet, "quiet", false, "data only: no help, no aggregates")
	fs.DurationVar(&g.timeout, "timeout", defaultTimeout, "per-request timeout")
}

func (g *globals) options() axi.Options {
	o := axi.Options{
		Fields: axi.ParseFields(g.fields),
		Full:   g.full,
		NoHelp: g.noHelp,
		Quiet:  g.quiet,
	}
	if g.json {
		o.Format = axi.FormatJSON
	}
	return o
}

// command is one subcommand. usage is a single screen; AXI principle 10 wants
// --help everywhere and wants it short.
type command struct {
	name    string
	summary string
	usage   string
	flags   func(*flag.FlagSet)
	run     func(context.Context, *Env) error
	// hidden keeps a command out of the root listing without disabling it.
	hidden bool
}

// commands is the dispatch table. Registered from each command file's init so
// a new file is self-contained.
var commands = map[string]*command{}

func register(c *command) { commands[c.name] = c }

func lookup(name string) (*command, bool) {
	c, ok := commands[name]
	return c, ok
}

func commandNames() []string {
	names := make([]string, 0, len(commands))
	for n, c := range commands {
		if c.hidden {
			continue
		}
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

func unknownCommand(name string) error {
	return axi.Usagef(
		[]string{
			fmt.Sprintf("%s help", binaryName),
			binaryName,
		},
		"unknown command %q", name)
}

func writeRootHelp(w io.Writer) {
	fmt.Fprintf(w, "%s — the damson CLI\n\n", binaryName)
	fmt.Fprintf(w, "usage: %s [command] [flags]\n\n", binaryName)
	fmt.Fprintf(w, "Run %s with no arguments for a live view of your library.\n\n", binaryName)
	fmt.Fprintln(w, "commands:")
	names := commandNames()
	width := 0
	for _, n := range names {
		if len(n) > width {
			width = len(n)
		}
	}
	for _, n := range names {
		fmt.Fprintf(w, "  %-*s  %s\n", width, n, commands[n].summary)
	}
	fmt.Fprintln(w, "\nglobal flags:")
	fmt.Fprintln(w, "  --server URL   --token TOKEN   --timeout DURATION")
	fmt.Fprintln(w, "  --json         --fields LIST   --full")
	fmt.Fprintln(w, "  --no-help      --quiet")
	fmt.Fprintf(w, "\nenvironment: DAMSON_SERVER, DAMSON_TOKEN, DAMSON_CONFIG\n")
	fmt.Fprintf(w, "\n%s <command> --help for one command.\n", binaryName)
}

func writeCommandHelp(w io.Writer, c *command) {
	fmt.Fprintf(w, "%s %s — %s\n\n", binaryName, c.name, c.summary)
	if c.usage != "" {
		fmt.Fprintln(w, strings.TrimRight(c.usage, "\n"))
		fmt.Fprintln(w)
	}
	fmt.Fprintln(w, "global flags: --server --token --json --fields --full --no-help --quiet --timeout")
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if s := strings.TrimSpace(v); s != "" {
			return s
		}
	}
	return ""
}
