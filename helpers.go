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

// persistedConfig is what we serialize to disk. We don't write the legacy
// fields back out — once we migrate them, they are gone.
type persistedConfig struct {
	Origins  []Origin `toml:"Origins"`
	Username string   `toml:"Username"`
	Password string   `toml:"Password"`
}

func SaveConfig(cfg Config) error {
	path, err := configFilePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("failed to create config folder: %v", err)
	}
	out := persistedConfig{
		Origins:  cfg.Origins,
		Username: cfg.Username,
		Password: cfg.Password,
	}
	b, err := toml.Marshal(out)
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

	migrated := migrateLegacy(&cfg)
	ensureIDs(&cfg)
	credChanged := ensureCredentials(&cfg)
	if migrated || credChanged {
		if err := SaveConfig(cfg); err != nil {
			log.Printf("failed to persist updated config: %v", err)
		}
	}
	return cfg, nil
}

// migrateLegacy folds the old single-origin TOML schema into the new
// Origins slice. Returns true if a write-back is needed.
func migrateLegacy(cfg *Config) bool {
	if len(cfg.Origins) > 0 {
		cfg.LegacyOrigin = ""
		cfg.LegacyDestinations = nil
		cfg.LegacyKeys = nil
		cfg.LegacyEnabled = nil
		return false
	}
	if cfg.LegacyOrigin == "" && len(cfg.LegacyDestinations) == 0 {
		return false
	}
	o := Origin{
		ID:  newOriginID(),
		URL: cfg.LegacyOrigin,
	}
	for i, rtmp := range cfg.LegacyDestinations {
		key := ""
		if i < len(cfg.LegacyKeys) {
			key = cfg.LegacyKeys[i]
		}
		enabled := true
		if i < len(cfg.LegacyEnabled) {
			enabled = cfg.LegacyEnabled[i]
		}
		o.Destinations = append(o.Destinations, Destination{
			RTMP:    rtmp,
			Key:     key,
			Enabled: enabled,
		})
	}
	cfg.Origins = []Origin{o}
	cfg.LegacyOrigin = ""
	cfg.LegacyDestinations = nil
	cfg.LegacyKeys = nil
	cfg.LegacyEnabled = nil
	log.Println("Migrated legacy single-origin config to multi-origin schema.")
	return true
}

func ensureIDs(cfg *Config) {
	seen := make(map[string]bool, len(cfg.Origins))
	for i := range cfg.Origins {
		id := cfg.Origins[i].ID
		if id == "" || seen[id] {
			cfg.Origins[i].ID = newOriginID()
		}
		seen[cfg.Origins[i].ID] = true
	}
}

func newOriginID() string {
	return randomHex(8)
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

func CreateConfig() error {
	path, err := configFilePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("failed to create config folder: %v", err)
	}

	cfg := persistedConfig{
		Origins: []Origin{{
			ID:  newOriginID(),
			URL: "rtmp://0.0.0.0:1935/test",
			Destinations: []Destination{
				{RTMP: "rtmp://a.rtmp.youtube.com/live2", Key: "youtube_key", Enabled: true},
				{RTMP: "rtmp://live.twitch.tv/app", Key: "twitch_key", Enabled: true},
			},
		}},
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
