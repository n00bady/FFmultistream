package main

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

type myTheme struct{}

func (m myTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNameBackground:
		return color.RGBA{R: 29, G: 32, B: 33, A: 255}
	case theme.ColorNameForeground:
		return color.RGBA{R: 235, G: 219, B: 178, A: 255}
	case theme.ColorNamePrimary:
		return color.RGBA{R: 214, G: 93, B: 14, A: 255}
	case theme.ColorNameButton:
		return color.RGBA{R: 80, G: 73, B: 69, A: 255}
	case theme.ColorNameDisabled:
		return color.RGBA{R: 168, G: 153, B: 132, A: 255}
	case theme.ColorNameHover:
		return color.RGBA{R: 255, G: 255, B: 255, A: 65}
	case theme.ColorNameSelection:
		return color.RGBA{R: 214, G: 93, B: 14, A: 255}
	default:
		return theme.DefaultTheme().Color(name, variant)
	}
}

func (m myTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}

func (m myTheme) Font(style fyne.TextStyle) fyne.Resource {
	return theme.DefaultTheme().Font(style)
}

func (m myTheme) Size(name fyne.ThemeSizeName) float32 {
	return theme.DefaultTheme().Size(name)
}
