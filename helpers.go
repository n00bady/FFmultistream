package main

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

func configFilePath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("failed to find config directory: %v", err)
	}
	return filepath.Join(configDir, "FFmultistream", "config.toml"), nil
}

func SaveConfig(cfg Config) error {
	path, err := configFilePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("failed to create config folder: %v", err)
	}
	b, err := toml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling config failed: %v", err)
	}
	if err := os.WriteFile(path, b, 0o600); err != nil {
		return fmt.Errorf("failed to write config file: %v", err)
	}
	return nil
}

func LoadConfig() (Config, error) {
	var cfg Config
	path, err := configFilePath()
	if err != nil {
		return cfg, err
	}

	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		log.Println("Cannot find config file, creating the default one...")
		if err := CreateConfig(); err != nil {
			return cfg, err
		}
		data, err = os.ReadFile(path)
		if err != nil {
			return cfg, fmt.Errorf("failed to read newly created config: %v", err)
		}
	} else if err != nil {
		return cfg, fmt.Errorf("failed to read config file: %v", err)
	}

	if err := toml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("failed to unmarshal config file: %v", err)
	}
	normalizeEnabled(&cfg)
	if ensureCredentials(&cfg) {
		if err := SaveConfig(cfg); err != nil {
			log.Printf("failed to persist generated credentials: %v", err)
		}
	}
	return cfg, nil
}

func ensureCredentials(cfg *Config) bool {
	changed := false
	if cfg.Username == "" {
		cfg.Username = "admin"
		changed = true
	}
	if cfg.Password == "" {
		cfg.Password = randomHex(20)
		changed = true
	}
	if changed {
		log.Println("Generated UI credentials (stored in config.toml):")
		log.Printf("  username: %s", cfg.Username)
		log.Printf("  password: %s", cfg.Password)
	}
	return changed
}

func randomHex(n int) string {
	b := make([]byte, (n+1)/2)
	if _, err := rand.Read(b); err != nil {
		log.Fatalf("failed to read random bytes: %v", err)
	}
	return hex.EncodeToString(b)[:n]
}

func normalizeEnabled(cfg *Config) {
	if len(cfg.Enabled) == len(cfg.Destinations) {
		return
	}
	enabled := make([]bool, len(cfg.Destinations))
	for i := range enabled {
		if i < len(cfg.Enabled) {
			enabled[i] = cfg.Enabled[i]
		} else {
			enabled[i] = true
		}
	}
	cfg.Enabled = enabled
}

func CreateConfig() error {
	path, err := configFilePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("failed to create config folder: %v", err)
	}

	cfg := Config{
		Origin:       "rtmp://0.0.0.0:1935/test",
		Destinations: []string{"rtmp://a.rtmp.youtube.com/live2", "rtmp://live.twitch.tv/app"},
		Keys:         []string{"youtube_key", "twitch_key"},
		Enabled:      []bool{true, true},
	}

	b, err := toml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling default config failed: %v", err)
	}
	if err := os.WriteFile(path, b, 0o600); err != nil {
		return fmt.Errorf("failed to write toml file: %v", err)
	}

	log.Printf("Default config file created at: %s\n", path)
	return nil
}

func IsvalidRTMP(s string) bool {
	lower := strings.ToLower(s)
	if !strings.HasPrefix(lower, "rtmp://") && !strings.HasPrefix(lower, "rtmps://") {
		log.Println("RTMP entry doesn't have prefix rtmp:// or rtmps://")
		return false
	}
	if _, err := url.Parse(s); err != nil {
		log.Println("RTMP entry is not a valid url.")
		return false
	}
	return true
}

func IsvalidKEY(s string) bool {
	if s == "" {
		log.Println("Key entry is empty.")
		return false
	}
	return true
}

func maskKey(s string) string {
	const dot = "•"
	n := len([]rune(s))
	if n == 0 {
		return ""
	}
	if n <= 4 {
		return strings.Repeat(dot, n)
	}
	runes := []rune(s)
	return string(runes[:2]) + strings.Repeat(dot, n-4) + string(runes[n-2:])
}
