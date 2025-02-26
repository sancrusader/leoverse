package main

import (
	"automation/leonai"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime/debug"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"
)

// Build flags
var version = ""
var commit = ""
var date = ""

func main() {
	// Create signal based context
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	// Launch command
	cmd := newCommand()
	if err := cmd.ParseAndRun(ctx, os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}

func newCommand() *ffcli.Command {
	fs := flag.NewFlagSet("leonai", flag.ExitOnError)

	var (
		cookie string
		debug  bool
		proxy  string
	)

	fs.StringVar(&cookie, "cookie", "", "Leonardo.ai cookie")
	fs.BoolVar(&debug, "debug", false, "Enable debug mode")
	fs.StringVar(&proxy, "proxy", "", "Proxy URL")

	return &ffcli.Command{
		ShortUsage: "leonai [flags] <subcommand>",
		FlagSet:    fs,
		Subcommands: []*ffcli.Command{
			newVersionCommand(),
			newGenerateCommand(cookie, debug, proxy),
		},
	}
}

func newGenerateCommand(cookie string, debug bool, proxy string) *ffcli.Command {
	fs := flag.NewFlagSet("generate", flag.ExitOnError)

	var (
		genCookie string
		genDebug  bool
		genProxy  string
	)

	fs.StringVar(&genCookie, "cookie", cookie, "Leonardo.ai cookie")
	fs.BoolVar(&genDebug, "debug", debug, "Enable debug mode")
	fs.StringVar(&genProxy, "proxy", proxy, "Proxy URL")

	return &ffcli.Command{
		Name:       "generate",
		ShortUsage: "leonai generate [flags] <prompt>",
		ShortHelp:  "Generate image using Leonardo.ai",
		FlagSet:    fs,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) < 1 {
				return fmt.Errorf("prompt is required")
			}

			// If no cookie provided, try to read from cookie.txt
			if genCookie == "" {
				data, err := os.ReadFile("cookie.txt")
				if err == nil {
					var session struct {
						AccessToken string `json:"accessToken"`
					}
					if err := json.Unmarshal(data, &session); err == nil && session.AccessToken != "" {
						genCookie = fmt.Sprintf("__Secure-next-auth.session-token=%s", session.AccessToken)
					} else {
						genCookie = strings.TrimSpace(string(data))
					}
				}
			}

			cfg := &leonai.Config{
				Cookie: genCookie,
				Debug:  genDebug,
				Proxy:  genProxy,
			}

			return leonai.GenerateImage(ctx, cfg, args[0])
		},
	}
}

func newVersionCommand() *ffcli.Command {
	return &ffcli.Command{
		Name:       "version",
		ShortUsage: "leonai version",
		ShortHelp:  "print version",
		Exec: func(ctx context.Context, args []string) error {
			v := version
			if v == "" {
				if buildInfo, ok := debug.ReadBuildInfo(); ok {
					v = buildInfo.Main.Version
				}
			}
			if v == "" {
				v = "dev"
			}
			versionFields := []string{v}
			if commit != "" {
				versionFields = append(versionFields, commit)
			}
			if date != "" {
				versionFields = append(versionFields, date)
			}
			fmt.Println(strings.Join(versionFields, " "))
			return nil
		},
	}
}
