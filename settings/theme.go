package settings

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// fixedVariant wraps the default theme and pins its light/dark variant,
// ignoring the OS preference. It is what the "light" and "dark" settings
// select; "system" uses the default theme as-is.
type fixedVariant struct {
	fyne.Theme
	variant fyne.ThemeVariant
}

func (t *fixedVariant) Color(name fyne.ThemeColorName, _ fyne.ThemeVariant) color.Color {
	return t.Theme.Color(name, t.variant)
}

// ApplyTheme applies the specified theme to the application. The "system"
// variant defers to Fyne's default theme, which in turn follows the OS
// light/dark preference (e.g. GNOME color-scheme, macOS appearance).
func ApplyTheme(app fyne.App, themeName string) {
	switch themeName {
	case "dark":
		app.Settings().SetTheme(&fixedVariant{theme.DefaultTheme(), theme.VariantDark})
	case "light":
		app.Settings().SetTheme(&fixedVariant{theme.DefaultTheme(), theme.VariantLight})
	default:
		// "system" and anything unknown: follow the OS preference.
		app.Settings().SetTheme(theme.DefaultTheme())
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
		return "Dark"
	case "light":
		return "Light"
	default:
		return "Follow system"
	}
}
