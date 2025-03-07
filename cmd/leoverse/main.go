package main

import (
	"automation/leoverse"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"runtime/debug"
	"strings"
	"syscall"

	"automation/leoverse/pkg/airtable"

	"github.com/joho/godotenv"

	"github.com/peterbourgon/ff/v3/ffcli"
)

// Build flags
var version = ""
var commit = ""
var date = ""

func mainCmd() {
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
	fs := flag.NewFlagSet("leoverse", flag.ExitOnError)

	var (
		cookie string
		debug  bool
		proxy  string
	)

	fs.StringVar(&cookie, "cookie", "", "Leonardo.ai cookie")
	fs.BoolVar(&debug, "debug", false, "Enable debug mode")
	fs.StringVar(&proxy, "proxy", "", "Proxy URL")

	return &ffcli.Command{
		ShortUsage: "leoverse [flags] <subcommand>",
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
		ShortUsage: "leoverse generate [flags] <prompt>",
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

			cfg := &leoverse.Config{
				Cookie: genCookie,
				Debug:  genDebug,
				Proxy:  genProxy,
			}

			return leoverse.GenerateImage(ctx, cfg, args[0])
		},
	}
}

func newVersionCommand() *ffcli.Command {
	return &ffcli.Command{
		Name:       "version",
		ShortUsage: "leoverse version",
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

func main() {
	// Disable non-essential logging
	log.SetOutput(io.Discard)

	// Load environment variables from .env file
	if err := godotenv.Load(); err != nil {
		fmt.Printf("Warning: Error loading .env file: %v\n", err)
	}

	generateCmd := flag.NewFlagSet("generate", flag.ExitOnError)
	prompt := generateCmd.String("prompt", "", "Prompt for image generation")
	debug := generateCmd.Bool("debug", false, "Enable debug mode")
	proxy := generateCmd.String("proxy", "", "Proxy URL")

	airtableCmd := flag.NewFlagSet("airtable", flag.ExitOnError)
	debugAirtable := airtableCmd.Bool("debug", false, "Enable debug mode")
	proxyAirtable := airtableCmd.String("proxy", "", "Proxy URL")

	if len(os.Args) < 2 {
		fmt.Println("expected 'generate' or 'airtable' subcommands")
		os.Exit(1)
	}

	// Read cookie from file
	cookie, err := os.ReadFile("cmd/leoverse/cookie.txt")
	if err != nil {
		fmt.Printf("Error reading cookie file: %v\n", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle interrupt signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	switch os.Args[1] {
	case "generate":
		generateCmd.Parse(os.Args[2:])
		if *prompt == "" {
			fmt.Println("please provide a prompt")
			os.Exit(1)
		}

		cfg := &leoverse.Config{
			Cookie: string(cookie),
			Debug:  *debug,
			Proxy:  *proxy,
		}

		if err := leoverse.GenerateImage(ctx, cfg, *prompt); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

	case "airtable":
		airtableCmd.Parse(os.Args[2:])
		// Get Airtable configuration from environment variables
		apiKey := os.Getenv("AIRTABLE_API_KEY")
		baseID := os.Getenv("AIRTABLE_BASE_ID")
		tableName := os.Getenv("AIRTABLE_TABLE_NAME")

		if apiKey == "" || baseID == "" || tableName == "" {
			fmt.Println("please set AIRTABLE_API_KEY, AIRTABLE_BASE_ID, and AIRTABLE_TABLE_NAME environment variables")
			os.Exit(1)
		}

		cfg := &leoverse.Config{
			Cookie: string(cookie),
			Debug:  *debugAirtable,
			Proxy:  *proxyAirtable,
		}

		// Initialize Airtable client
		airtableClient := airtable.NewClient(apiKey, baseID, tableName)
		log.Printf("Initialized Airtable client for base %s, table %s", baseID, tableName)

		// Process prompts from Airtable
		processFunc := func(prompt string) (string, error) {
			// Create temporary directory for each prompt
			tempDir, err := os.MkdirTemp("", "leoverse-*")
			if err != nil {
				log.Printf("Error creating temp directory: %v", err)
				return "", fmt.Errorf("couldn't create temp directory: %w", err)
			}
			log.Printf("Created temporary directory: %s", tempDir)

			// Set output directory to temp directory
			os.Setenv("OUTPUT_DIR", tempDir)
			log.Printf("Processing prompt: %q", prompt)

			// Generate image
			if err := leoverse.GenerateImage(ctx, cfg, prompt); err != nil {
				log.Printf("Error generating image: %v", err)
				os.RemoveAll(tempDir)
				return "", fmt.Errorf("generation failed: %w", err)
			}
			log.Printf("Successfully generated image for prompt: %q", prompt)

			// Process all generated images
			for i := 1; i <= 4; i++ {
				imagePath := fmt.Sprintf("%s/image_%d.png", tempDir, i)
				log.Printf("Processing image: %s", imagePath)

				// Upload each image to Airtable
				if err := airtableClient.UploadImage(prompt, imagePath); err != nil {
					log.Printf("Error uploading image %d: %v", i, err)
					continue
				}
				log.Printf("Successfully uploaded image %d to Airtable", i)
			}

			// Return success even if some uploads failed
			return tempDir, nil
		}

		log.Println("Starting to process prompts from Airtable...")
		if err := airtableClient.ProcessPrompts(processFunc); err != nil {
			log.Printf("Error processing prompts: %v", err)
			fmt.Printf("Error processing prompts: %v\n", err)
			os.Exit(1)
		}
		log.Println("Successfully completed processing all prompts")

	default:
		fmt.Println("expected 'generate' or 'airtable' subcommands")
		os.Exit(1)
	}
}
