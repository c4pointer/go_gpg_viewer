package assets

import (
	"embed"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

//go:embed icon.svg
var iconFS embed.FS

// GetAppIcon returns the custom application icon
func GetAppIcon() fyne.Resource {
	iconData, err := iconFS.ReadFile("icon.svg")
	if err != nil {
		// Fallback to default theme icon if custom icon fails to load
		return theme.ComputerIcon()
	}

	return fyne.NewStaticResource("app-icon", iconData)
}
