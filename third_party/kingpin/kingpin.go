package kingpin

import (
	"flag"
	"fmt"
	"os"
)

type Application struct {
	fs      *flag.FlagSet
	version string
	post    []func()
}

type FlagClause struct {
	app          *Application
	name         string
	help         string
	defaultValue string
}

type stringFlag struct {
	ptr *string
}

type boolFlag struct {
	ptr *bool
}

var CommandLine = &Application{fs: flag.CommandLine}

var versionFlag bool

var HelpFlag = &helpClause{}

type helpClause struct{}

func (h *helpClause) Short(r rune) *helpClause {
	// The standard library already supports -h automatically.
	return h
}

func Flag(name, help string) *FlagClause {
	return CommandLine.Flag(name, help)
}

func (a *Application) Flag(name, help string) *FlagClause {
	return &FlagClause{app: a, name: name, help: help}
}

func (c *FlagClause) Default(value string) *FlagClause {
	c.defaultValue = value
	return c
}

func (c *FlagClause) String() *string {
	if c.app.fs == nil {
		c.app.fs = flag.CommandLine
	}
	ptr := c.app.fs.String(c.name, c.defaultValue, c.help)
	return ptr
}

func (c *FlagClause) Bool() *bool {
	if c.app.fs == nil {
		c.app.fs = flag.CommandLine
	}
	def := false
	if c.defaultValue != "" {
		if c.defaultValue == "true" || c.defaultValue == "1" {
			def = true
		}
	}
	ptr := c.app.fs.Bool(c.name, def, c.help)
	return ptr
}

func Version(v string) {
	CommandLine.Version(v)
}

func (a *Application) Version(v string) {
	a.version = v
	a.fs.BoolVar(&versionFlag, "version", false, "print version and exit")
}

func Parse() {
	CommandLine.Parse()
}

func AfterParse(fn func()) {
	CommandLine.AfterParse(fn)
}

func (a *Application) Parse() {
	if a.fs == nil {
		a.fs = flag.CommandLine
	}
	a.fs.Parse(os.Args[1:])
	if versionFlag {
		fmt.Println(a.version)
		os.Exit(0)
	}
	for _, fn := range a.post {
		fn()
	}
}

func (a *Application) AfterParse(fn func()) {
	a.post = append(a.post, fn)
}
