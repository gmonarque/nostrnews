package config

import (
	_ "embed"
	"encoding/json"
)

//go:embed feeds.json
var feedsJSON []byte

type Feed struct {
	URL      string   `json:"url"`
	Name     string   `json:"name"`
	Country  string   `json:"country"`
	Language string   `json:"language"`
	Category string   `json:"category"`
	Paywall  string   `json:"paywall"`
	Tags     []string `json:"tags"`
}

type Config struct {
	Feeds []Feed `json:"feeds"`
}

func Load() (*Config, error) {
	var cfg Config
	if err := json.Unmarshal(feedsJSON, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
