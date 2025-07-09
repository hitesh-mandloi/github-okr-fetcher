package litellm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github-okr-fetcher/internal/domain/entity"
)

// Client represents a LiteLLM API client
type Client struct {
	baseURL    string
	token      string
	model      string
	httpClient *http.Client
}

// NewClient creates a new LiteLLM API client
func NewClient(config entity.LiteLLMConfig, token string) *Client {
	timeoutSec := 60
	if config.TimeoutSec > 0 {
		timeoutSec = config.TimeoutSec
	}

	return &Client{
		baseURL: config.BaseURL,
		token:   token,
		model:   config.Model,
		httpClient: &http.Client{
			Timeout: time.Duration(timeoutSec) * time.Second,
		},
	}
}

// ChatRequest represents a chat completion request
type ChatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
}

// Message represents a chat message
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatResponse represents a chat completion response
type ChatResponse struct {
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

// Choice represents a response choice
type Choice struct {
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

// Usage represents token usage information
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// AnalyzeOKRs sends OKR data to LiteLLM for analysis
func (c *Client) AnalyzeOKRs(okrData string) (string, error) {
	prompt := fmt.Sprintf(`
Analyze the following OKR (Objectives and Key Results) data and provide a short summary (100 words in bullet points) focusing on:

1. **Success & Achievements**: List completed issues, key milestones reached, and notable impactful business achievements that are clearly visible
2. **Business Impact**: Provide quantitative and qualitative metrics showing business value, developer productivity improvements, and strategic outcomes

Please format your response in markdown with clear sections and keep it concise.

OKR Data:
%s

Provide a brief analysis focused on achievements and business impact.`, okrData)

	request := ChatRequest{
		Model: c.model,
		Messages: []Message{
			{
				Role:    "user",
				Content: prompt,
			},
		},
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", c.baseURL+"/v1/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var chatResp ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("no response choices returned")
	}

	return chatResp.Choices[0].Message.Content, nil
}
