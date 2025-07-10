package main

import (
	"context"

	"fyne.io/fyne/v2"
)

type Config struct {
	Origin       string   `toml:"Origin"`
	Destinations []string `toml:"Destinations"`
	Keys         []string `toml:"Keys"`
}

type AppState struct {
	config Config
	ctx    context.Context
	window fyne.Window
}
