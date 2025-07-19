package settings

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// ApplyTheme applies the specified theme to the application
func ApplyTheme(app fyne.App, themeName string) {
	switch themeName {
	case "dark":
		app.Settings().SetTheme(theme.DarkTheme())
	case "light":
		app.Settings().SetTheme(theme.LightTheme())
	default:
		// Default to light theme
		app.Settings().SetTheme(theme.LightTheme())
	}
}

// GetAvailableThemes returns a list of available themes
func GetAvailableThemes() []string {
	return []string{"light", "dark"}
}

// GetThemeDisplayName returns a user-friendly name for the theme
func GetThemeDisplayName(themeName string) string {
	switch themeName {
	case "dark":
		return "Dark Theme"
	case "light":
		return "Light Theme"
	default:
		return "Light Theme"
	}
}
