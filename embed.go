package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// OpenAI embedding request structure
type openAIRequest struct {
	Input string `json:"input"`
	Model string `json:"model"`
}

// OpenAI embedding response structures
type openAIData struct {
	Embedding []float32 `json:"embedding"`
}

type openAIResponse struct {
	Data []openAIData `json:"data"`
}

// Ollama embedding request structure
type ollamaRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

// Ollama embedding response structure
type ollamaResponse struct {
	Embedding []float32 `json:"embedding"`
}

// GetEmbedding encodes plain text into a float32 vector using the configured provider.
func GetEmbedding(cfg *Config, text string) ([]float32, error) {
	provider := strings.ToLower(cfg.EmbeddingProvider)
	if provider == "" {
		return nil, errors.New("embedding provider is not configured. Please set 'embedding_provider' to 'openai' or 'ollama' in config.json")
	}

	switch provider {
	case "openai":
		return getOpenAIEmbedding(cfg, text)
	case "ollama":
		return getOllamaEmbedding(cfg, text)
	default:
		return nil, fmt.Errorf("unsupported embedding provider: %s. Supported: openai, ollama", cfg.EmbeddingProvider)
	}
}

func getOpenAIEmbedding(cfg *Config, text string) ([]float32, error) {
	apiURL := cfg.EmbeddingAPIURL
	if apiURL == "" {
		apiURL = "https://api.openai.com/v1/embeddings"
	} else {
		// Ensure correct endpoints if they just passed host
		if !strings.HasSuffix(apiURL, "/embeddings") {
			apiURL = strings.TrimRight(apiURL, "/") + "/embeddings"
		}
	}

	model := cfg.EmbeddingModel
	if model == "" {
		model = "text-embedding-3-small"
	}

	reqBody := openAIRequest{
		Input: text,
		Model: model,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	if cfg.EmbeddingAPIKey != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.EmbeddingAPIKey)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errorDetail map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errorDetail)
		return nil, fmt.Errorf("API returned status %s: %v", resp.Status, errorDetail)
	}

	var apiResp openAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, err
	}

	if len(apiResp.Data) == 0 {
		return nil, errors.New("no embedding data returned from API")
	}

	return apiResp.Data[0].Embedding, nil
}

func getOllamaEmbedding(cfg *Config, text string) ([]float32, error) {
	apiURL := cfg.EmbeddingAPIURL
	if apiURL == "" {
		apiURL = "http://localhost:11434/api/embeddings"
	} else {
		if !strings.HasSuffix(apiURL, "/api/embeddings") {
			apiURL = strings.TrimRight(apiURL, "/") + "/api/embeddings"
		}
	}

	model := cfg.EmbeddingModel
	if model == "" {
		return nil, errors.New("ollama model must be specified in 'embedding_model' (e.g. mxbai-embed-large)")
	}

	reqBody := ollamaRequest{
		Model:  model,
		Prompt: text,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Ollama: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama returned status: %s", resp.Status)
	}

	var apiResp ollamaResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, err
	}

	return apiResp.Embedding, nil
}
