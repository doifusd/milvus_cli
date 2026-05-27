package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Config holds all connection parameters for SSH and Milvus.
type Config struct {
	SSHHost     string `json:"ssh_host"`     // SSH Host (ip:port, e.g. 192.168.1.100:22)
	SSHUser     string `json:"ssh_user"`     // SSH Username (e.g. root)
	SSHPassword string `json:"ssh_password"` // SSH Password (optional, will prompt if empty and key isn't provided)
	SSHKeyPath  string `json:"ssh_key_path"`  // SSH Private Key Path (e.g. ~/.ssh/id_rsa)
	SSHKeyPass  string `json:"ssh_key_pass"`  // Passphrase for encrypted private key (optional)
	MilvusAddr  string `json:"milvus_addr"`  // Milvus Host (e.g. localhost:19530)
	MilvusUser        string `json:"milvus_user"`        // Milvus Username (optional)
	MilvusPass        string `json:"milvus_pass"`        // Milvus Password (optional)
	MilvusDB          string `json:"milvus_db"`          // Milvus Database Name (default "default")
	EmbeddingProvider string `json:"embedding_provider"` // "openai", "ollama", or ""
	EmbeddingAPIKey   string `json:"embedding_api_key"`   // OpenAI API Key (optional)
	EmbeddingModel    string `json:"embedding_model"`    // Model name (default "text-embedding-3-small")
	EmbeddingAPIURL   string `json:"embedding_api_url"`   // Proxy or local endpoint URL (optional)
}

// DefaultConfig returns a configuration with default values populated.
func DefaultConfig() *Config {
	home, err := os.UserHomeDir()
	var defaultKeyPath string
	if err == nil {
		defaultKeyPath = filepath.Join(home, ".ssh", "id_rsa")
	}

	return &Config{
		SSHHost:           "",
		SSHUser:           "root",
		SSHKeyPath:        defaultKeyPath,
		MilvusAddr:        "localhost:19530",
		MilvusDB:          "default",
		EmbeddingProvider: "",
		EmbeddingAPIKey:   "",
		EmbeddingModel:    "text-embedding-3-small",
		EmbeddingAPIURL:   "",
	}
}

// ExpandPath expands tilde (~) in paths to the user home directory.
func ExpandPath(path string) string {
	if path == "" {
		return ""
	}
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		if path == "~" {
			return home
		}
		return filepath.Join(home, path[2:])
	}
	return path
}

// LoadConfig tries to load configuration from the specified JSON file.
func LoadConfig(filename string) (*Config, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	cfg := DefaultConfig()
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config JSON: %w", err)
	}

	cfg.SSHKeyPath = ExpandPath(cfg.SSHKeyPath)
	return cfg, nil
}

// SaveConfig saves the configuration to the specified JSON file.
func SaveConfig(filename string, cfg *Config) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(cfg); err != nil {
		return fmt.Errorf("failed to encode config JSON: %w", err)
	}
	return nil
}
