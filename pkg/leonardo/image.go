package leonardo

import (
	"context"
	"fmt"
	"time"
)

const generateImageQuery = `mutation CreateSDGenerationJob($arg1: SDGenerationInput!) {
	sdGenerationJob(arg1: $arg1) {
		generationId
		__typename
	}
}`

type GenerateImageInput struct {
	Prompt         string
	NegativePrompt string
	ModelID        string
	Width          int
	Height         int
	NumImages      int
	GuidanceScale  float64
	PresetStyle    string
	Scheduler      string
	SDVersion      string
	Steps          int
	Public         bool
	HighContrast   bool
	PhotoReal      bool
	NSFW           bool
	Contrast       float64
	EnhancePrompt  bool
	Weighting      float64
}

func (c *Client) GenerateImage(ctx context.Context, input *GenerateImageInput) ([]string, error) {
	// Authenticate if necessary
	if err := c.Auth(ctx); err != nil {
		return nil, err
	}

	c.log("Creating generation job...")
	generationID, err := c.createGeneration(ctx, input)
	if err != nil {
		return nil, err
	}
	c.log("Generation job created with ID: %s", generationID)

	// Wait for generation to complete
	statusReq := &graphqlRequest{
		OperationName: "GetAIGenerationFeedStatuses",
		Variables: map[string]any{
			"where": map[string]any{
				"status": map[string]any{
					"_in": []string{"COMPLETE", "FAILED"},
				},
				"id": map[string]any{
					"_in": []string{generationID},
				},
			},
		},
		Query: statusQuery,
	}

	c.log("Waiting for generation to complete...")
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(5 * time.Second):
		}

		var statusResp statusResponse
		if _, err := c.do(ctx, "POST", "graphql", statusReq, &statusResp); err != nil {
			return nil, fmt.Errorf("couldn't get status: %w", err)
		}

		if len(statusResp.Data.Generations) > 0 {
			status := statusResp.Data.Generations[0]
			c.log("Generation status: %s", status.Status)

			if status.Status == "FAILED" {
				return nil, fmt.Errorf("generation failed")
			}
			if status.Status == "COMPLETE" {
				break
			}
		}
	}

	// Get generated images
	feedReq := &graphqlRequest{
		OperationName: "GetAIGenerationFeed",
		Variables: map[string]any{
			"where": map[string]any{
				"id": map[string]any{
					"_eq": generationID,
				},
			},
		},
		Query: feedQuery,
	}

	c.log("Fetching generated images...")
	var feedResp feedResponse
	if _, err := c.do(ctx, "POST", "graphql", feedReq, &feedResp); err != nil {
		return nil, fmt.Errorf("couldn't get feed: %w", err)
	}

	var urls []string
	if len(feedResp.Data.Generations) > 0 {
		gen := feedResp.Data.Generations[0]
		for _, img := range gen.GeneratedImages {
			urls = append(urls, img.URL)
		}
	}

	c.log("Found %d generated images", len(urls))
	return urls, nil
}

// Move existing GenerateImage implementation to this function
func (c *Client) createGeneration(ctx context.Context, input *GenerateImageInput) (string, error) {
    // Authenticate if necessary
    if err := c.Auth(ctx); err != nil {
        return "", err
    }

    // Prepare variables
    vars := map[string]any{
        "arg1": map[string]any{
            "prompt":              input.Prompt,
            "negative_prompt":     input.NegativePrompt,
            "modelId":             input.ModelID,
            "width":               input.Width,
            "height":              input.Height,
            "num_images":          input.NumImages,
            "guidance_scale":      input.GuidanceScale,
            "presetStyle":         input.PresetStyle,
            "scheduler":           input.Scheduler,
            "sd_version":          input.SDVersion,
            "num_inference_steps": input.Steps,
            "public":              input.Public,
            "highContrast":        input.HighContrast,
            "photoReal":           input.PhotoReal,
            "nsfw":                input.NSFW,
            "contrast":            input.Contrast,
            "enhancePrompt":       input.EnhancePrompt,
            "weighting":           input.Weighting,
        },
    }

    // Create GraphQL request
    req := &graphqlRequest{
        OperationName: "CreateSDGenerationJob",
        Variables:     vars,
        Query:         generateImageQuery,
    }

    // Execute request
    var resp createGenerationResponse
    if _, err := c.do(ctx, "POST", "graphql", req, &resp); err != nil {
        return "", fmt.Errorf("leonardo: couldn't create generation: %w", err)
    }

    generationID := resp.Data.SDGenerationJob.GenerationID
    if generationID == "" {
        c.log("leonardo: received empty generation ID from response: %+v", resp)
        return "", fmt.Errorf("leonardo: empty generation ID received")
    }

    c.log("leonardo: generation ID received: %s", generationID)
    return generationID, nil
}

func (c *Client) WaitForGeneration(ctx context.Context, generationID string) ([]GeneratedImage, error) {
	req := &graphqlRequest{
		OperationName: "GetAIGenerationFeed",
		Variables: map[string]any{
			"where": map[string]any{
				"id": map[string]any{
					"_eq": generationID,
				},
			},
		},
		Query: feedQuery,
	}

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(5 * time.Second):
		}

		var resp feedResponse
		if _, err := c.do(ctx, "POST", "graphql", req, &resp); err != nil {
			return nil, fmt.Errorf("couldn't get generation status: %w", err)
		}

		if len(resp.Data.Generations) == 0 {
			continue
		}

		gen := resp.Data.Generations[0]
		switch gen.Status {
		case "PENDING", "IN_PROGRESS":
			fmt.Printf("Generation status: %s\n", gen.Status)
			continue
		case "COMPLETE":
			images := make([]GeneratedImage, len(gen.GeneratedImages))
			for i, img := range gen.GeneratedImages {
				images[i] = GeneratedImage{
					ID:       img.ID,
					URL:      img.URL,
					NSFW:     img.Nsfw,
					Typename: img.Typename,
				}
			}
			return images, nil
		default:
			return nil, fmt.Errorf("generation failed with status: %s", gen.Status)
		}
	}
}

type GeneratedImage struct {
	ID       string `json:"id"`
	URL      string `json:"url"`
	NSFW     bool   `json:"nsfw"`
	Typename string `json:"__typename"`
}
