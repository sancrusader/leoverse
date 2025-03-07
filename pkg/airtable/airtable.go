package airtable

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Client struct {
	APIKey     string
	BaseID     string
	TableName  string
	httpClient *http.Client
}

type Record struct {
	ID     string                 `json:"id,omitempty"`
	Fields map[string]interface{} `json:"fields"`
}

type ListResponse struct {
	Records []Record `json:"records"`
	Offset  string   `json:"offset,omitempty"`
}

type UpdateResponse struct {
	Records []Record `json:"records"`
}

func NewClient(apiKey, baseID, tableName string) *Client {
	return &Client{
		APIKey:    apiKey,
		BaseID:    baseID,
		TableName: tableName,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *Client) GetPrompts() ([]Record, error) {
	url := fmt.Sprintf("https://api.airtable.com/v0/%s/%s", c.BaseID, c.TableName)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+c.APIKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var listResp ListResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		return nil, err
	}

	return listResp.Records, nil
}

func (c *Client) UpdateRecord(recordID string, imageData []byte) error {
	// Validate input data
	if len(imageData) == 0 {
		return fmt.Errorf("empty image data provided")
	}

	// Check file size (max 5MB as per Airtable's limit)
	const maxSize = 5 * 1024 * 1024 // 5MB
	if len(imageData) > maxSize {
		return fmt.Errorf("image size exceeds maximum allowed size of 5MB (current size: %.2fMB)", float64(len(imageData))/1024/1024)
	}

	// Detect MIME type
	mimeType := http.DetectContentType(imageData)
	if !strings.HasPrefix(mimeType, "image/") {
		return fmt.Errorf("invalid image format: %s", mimeType)
	}

	// Convert image data to base64
	imageBase64 := base64.StdEncoding.EncodeToString(imageData)

	// Prepare the upload payload
	uploadPayload := struct {
		ContentType string `json:"contentType"`
		File        string `json:"file"`
		Filename    string `json:"filename"`
	}{
		ContentType: mimeType,
		File:        imageBase64,
		Filename:    fmt.Sprintf("generated_image.%s", getExtensionFromMIME(mimeType)),
	}

	payload, err := json.Marshal(uploadPayload)
	if err != nil {
		return fmt.Errorf("failed to marshal upload payload: %w", err)
	}

	// Use the dedicated attachment upload endpoint
	url := fmt.Sprintf("https://content.airtable.com/v0/%s/%s/Image/uploadAttachment", c.BaseID, recordID)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to upload attachment: status=%d, response=%s", resp.StatusCode, string(body))
	}

	// Update the record to mark it as generated
	update := UpdateResponse{
		Records: []Record{
			{
				ID: recordID,
				Fields: map[string]interface{}{
					"Generated": true,
				},
			},
		},
	}

	payload, err = json.Marshal(update)
	if err != nil {
		return fmt.Errorf("failed to marshal update payload: %w", err)
	}

	url = fmt.Sprintf("https://api.airtable.com/v0/%s/%s", c.BaseID, c.TableName)
	req, err = http.NewRequest("PATCH", url, bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err = c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to update record: status=%d, response=%s", resp.StatusCode, string(body))
	}

	return nil
}

func (c *Client) ProcessPrompts(processFunc func(prompt string) (string, error)) error {
	records, err := c.GetPrompts()
	if err != nil {
		return fmt.Errorf("failed to get prompts: %w", err)
	}

	if len(records) == 0 {
		fmt.Println("No prompts found in Airtable")
		return nil
	}

	processedCount := 0
	skippedCount := 0

	for _, record := range records {
		// Skip if already generated
		if generated, ok := record.Fields["Generated"].(bool); ok && generated {
			skippedCount++
			fmt.Printf("Skipping already processed prompt ID: %s\n", record.ID)
			continue
		}

		prompt, ok := record.Fields["Prompt"].(string)
		if !ok || prompt == "" {
			fmt.Printf("Warning: Record %s has no valid prompt field\n", record.ID)
			continue
		}

		fmt.Printf("Processing prompt ID %s: %q\n", record.ID, prompt)

		// Process the prompt
		imageFile, err := processFunc(prompt)
		if err != nil {
			fmt.Printf("Error processing prompt '%s': %v\n", prompt, err)
			continue
		}

		// Verify the image file exists
		fileInfo, err := os.Stat(imageFile)
		if err != nil {
			fmt.Printf("Error: Image file '%s' does not exist: %v\n", imageFile, err)
			continue
		}

		// Check if the path is a directory and handle accordingly
		if fileInfo.IsDir() {
			// Try to find the image file in the directory
			files, err := os.ReadDir(imageFile)
			if err != nil {
				fmt.Printf("Error reading directory '%s': %v\n", imageFile, err)
				continue
			}

			// Look for image files in the directory
			var found bool
			for _, file := range files {
				if !file.IsDir() && strings.HasPrefix(file.Name(), "image_") {
					imageFile = filepath.Join(imageFile, file.Name())
					found = true
					break
				}
			}

			if !found {
				fmt.Printf("Error: No valid image file found in directory '%s'\n", imageFile)
				continue
			}
		}

		// Read the generated image
		imageData, err := os.ReadFile(imageFile)
		if err != nil {
			fmt.Printf("Error reading image file '%s': %v\n", imageFile, err)
			continue
		}

		// Verify we have valid image data
		if len(imageData) == 0 {
			fmt.Printf("Error: Image file '%s' is empty\n", imageFile)
			continue
		}

		fmt.Printf("Attempting to update record %s with image (size: %d bytes)\n", record.ID, len(imageData))

		// Update the record with the generated image
		if err := c.UpdateRecord(record.ID, imageData); err != nil {
			fmt.Printf("Error updating record for prompt '%s': %v\n", prompt, err)
			continue
		}

		processedCount++
		fmt.Printf("Successfully processed prompt ID %s: %q\n", record.ID, prompt)
	}

	fmt.Printf("Processing completed. Total records: %d, Processed: %d, Skipped: %d\n",
		len(records), processedCount, skippedCount)

	return nil
}

func (c *Client) UploadImage(prompt string, imagePath string) error {
	// Read the image file
	imageData, err := os.ReadFile(imagePath)
	if err != nil {
		return fmt.Errorf("failed to read image file: %w", err)
	}

	// Get records to find the matching prompt
	records, err := c.GetPrompts()
	if err != nil {
		return fmt.Errorf("failed to get records: %w", err)
	}

	// Find the record with matching prompt
	var recordID string
	for _, record := range records {
		if p, ok := record.Fields["Prompt"].(string); ok && p == prompt {
			recordID = record.ID
			break
		}
	}

	if recordID == "" {
		return fmt.Errorf("no record found for prompt: %s", prompt)
	}

	// Update the record with the image
	return c.UpdateRecord(recordID, imageData)
}

func getExtensionFromMIME(mimeType string) string {
	switch mimeType {
	case "image/jpeg", "image/jpg":
		return "jpg"
	case "image/png":
		return "png"
	case "image/gif":
		return "gif"
	case "image/webp":
		return "webp"
	default:
		return "png" // Default to png if unknown
	}
}
