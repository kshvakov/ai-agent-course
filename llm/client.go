package llm

import (
	"errors"
	"os"
)

// Config is the connection settings for labs' main.go, read from the environment.
type Config struct {
	APIKey  string
	BaseURL string
	Model   string
}

// LoadConfig reads LLM_API_KEY (required), LLM_BASE_URL and LLM_MODEL
// (default glm-5.3). Values can be exported by hand or sourced from a
// .env file in the repo root (relative path depends on where the lab
// lives: labs/ or translations/ru/labs/).
func LoadConfig() (Config, error) {
	cfg := Config{
		APIKey:  os.Getenv("LLM_API_KEY"),
		BaseURL: os.Getenv("LLM_BASE_URL"),
		Model:   os.Getenv("LLM_MODEL"),
	}
	if cfg.APIKey == "" {
		return Config{}, errors.New("LLM_API_KEY is not set: export it (or source your .env file from the repo root, e.g. set -a; source ../../.env; set +a from labs/<lab>)")
	}
	if cfg.Model == "" {
		cfg.Model = DefaultModel
	}
	return cfg, nil
}
