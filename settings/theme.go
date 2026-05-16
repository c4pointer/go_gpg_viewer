package settings

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// ApplyTheme applies the specified theme to the application. The "system"
// variant defers to Fyne's default theme, which in turn follows the OS
// light/dark preference (e.g. GNOME color-scheme, macOS appearance).
func ApplyTheme(app fyne.App, themeName string) {
	switch themeName {
	case "dark":
		app.Settings().SetTheme(theme.DarkTheme())
	case "light":
		app.Settings().SetTheme(theme.LightTheme())
	case "system":
		app.Settings().SetTheme(theme.DefaultTheme())
	default:
		// Unknown setting — fall back to light to keep behaviour predictable.
		app.Settings().SetTheme(theme.LightTheme())
	}
}

// GetAvailableThemes returns the list of theme keys recognised by
// ApplyTheme, in the order they should be shown to the user.
func GetAvailableThemes() []string {
	return []string{"system", "light", "dark"}
}

// GetThemeDisplayName returns a user-friendly name for the theme
func GetThemeDisplayName(themeName string) string {
	switch themeName {
	case "dark":
		return "Dark Theme"
	case "light":
		return "Light Theme"
	case "system":
		return "System (Auto)"
	default:
		return "Light Theme"
	}
}
