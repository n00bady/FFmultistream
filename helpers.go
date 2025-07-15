package main

import (
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

func SaveConfig(cfg Config) error {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return fmt.Errorf("failed to find config directory: %v", err)
	}
	configPath := filepath.Join(configDir, "FFmultistream")

	if !checkFileExist(configPath) {
		err := os.Mkdir(configPath, os.ModePerm)
		if err != nil {
			return fmt.Errorf("failed to create config folder: %v", err)
		}
	}
	b, err := toml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("Marshaling new config failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(configPath, "config.toml"), b, os.ModePerm); err != nil {
		return fmt.Errorf("Failed to write config file: %v", err)
	}

	return nil
}

func LoadConfig() (Config, error) {
	var cfg Config

	configPath, _ := os.UserConfigDir()
	data, err := os.ReadFile(filepath.Join(configPath, "FFmultistream", "config.toml"))
	if errors.Is(err, os.ErrNotExist) {
		log.Println("Cannot find config file, creating the default one...")
		CreateConfig()
	} else if err != nil {
		return cfg, fmt.Errorf("failed to read config file: %v", err)
	}
	
	err = toml.Unmarshal(data, &cfg)
	if err != nil {
		return cfg, fmt.Errorf("failed to Unmarshal config file: %v", err)
	}

	return cfg, nil
}

func CreateConfig() error {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return fmt.Errorf("failed to find config directory: %v", err)
	}
	configPath := filepath.Join(configDir, "FFmultistream")

	if !checkFileExist(configPath) {
		err := os.Mkdir(configPath, os.ModePerm)
		if err != nil {
			return fmt.Errorf("failed to create config folder: %v", err)
		}
	}

	cfg := Config{
		Origin: "rtmp://127.0.0.1:1935/test",
		Destinations: []string{"rtmp://a.rtmp.youtube.com/live2", "rtmp://live.twitch.tv/app"},
		Keys: []string{"youtube_key", "twitch_key"},
	}

	b, err := toml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("Marshaling default config failed: %v", err)
	}

	err = os.WriteFile(filepath.Join(configPath, "config.toml"), b, os.ModePerm)
	if err != nil {
		return fmt.Errorf("failed to write toml file: %v", err)
	}

	log.Printf("Default config file created at: %s\n", filepath.Join(configPath, "config.toml"))

	return nil
}

func checkFileExist(path string) bool {
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return false
	}

	return true
}

func IsvalidRTMP(s string) bool {
	if !strings.HasPrefix(strings.ToLower(s), "rtmp://") {
		log.Println("RTMP entry doesn't have prefix rtmp://")
		return false
	}

	_, err := url.Parse(s)
	if err != nil {
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
