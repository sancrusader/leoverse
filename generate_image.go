package leoverse

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"automation/leoverse/pkg/leonardo"
)

type Config struct {
	Cookie string
	Wait   bool
	Debug  bool
	Proxy  string
}

func GenerateImage(ctx context.Context, cfg *Config, prompt string) error {
	httpClient := &http.Client{
		Timeout: 5 * time.Minute, // Increased timeout
	}
	if cfg.Proxy != "" {
		u, err := url.Parse(cfg.Proxy)
		if err != nil {
			return fmt.Errorf("invalid proxy URL: %w", err)
		}
		httpClient.Transport = &http.Transport{
			Proxy: http.ProxyURL(u),
		}
	}

	client := leonardo.New(&leonardo.Config{
		Wait:        10 * time.Second, // Reduced wait time
		Debug:       cfg.Debug,
		Client:      httpClient,
		CookieStore: leonardo.NewMemCookieStore(cfg.Cookie),
	})

	if err := client.Start(ctx); err != nil {
		return fmt.Errorf("couldn't start leonardo client: %w", err)
	}
	defer client.Stop(ctx)

	fmt.Printf("Generating image for prompt: %q\n", prompt)
	startTime := time.Now()

	input := &leonardo.GenerateImageInput{
		Prompt:        prompt,
		Width:         1472,
		Height:        832,
		NumImages:     4,
		Steps:         10,   // Reduced steps
		Public:        true, // Changed to true
		EnhancePrompt: true,
		ModelID:       "6b645e3a-d64f-4341-a6d8-7a3690fbf042", // Updated model ID
		GuidanceScale: 7.0,
		Scheduler:     "LEONARDO",
		SDVersion:     "PHOENIX",  // Added SD version
		PresetStyle:   "LEONARDO", // Added preset style
		Contrast:      3.5,        // Added contrast
		Weighting:     0.75,       // Added weighting
		NSFW:          true,       // Allow NSFW content
	}

	urls, err := client.GenerateImage(ctx, input)
	if err != nil {
		return fmt.Errorf("generation failed: %w", err)
	}

	elapsed := time.Since(startTime).Round(time.Second)
	fmt.Printf("\nGeneration completed in %s\n", elapsed)
	fmt.Printf("Generated %d images:\n", len(urls))

	for i, url := range urls {
		fmt.Printf("%d. %s\n", i+1, url)

		// Get output directory from environment variable, default to "output"
		outputDir := os.Getenv("OUTPUT_DIR")
		if outputDir == "" {
			outputDir = "output"
		}

		// Create output directory if it doesn't exist
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			return fmt.Errorf("couldn't create output directory: %w", err)
		}

		filename := fmt.Sprintf("%s/image_%d.png", outputDir, i+1)
		if err := downloadImage(url, filename); err != nil {
			return fmt.Errorf("couldn't download image %d: %w", i+1, err)
		}
		fmt.Printf("Downloaded to: %s\n", filename)
	}

	return nil
}

func downloadImage(url, filename string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	out, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}
