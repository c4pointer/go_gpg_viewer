package assets

import (
	"embed"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

//go:embed icon.svg sync.svg
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

// SyncIcon returns the toolbar icon for git synchronization (two circular
// arrows, Material "sync"). It is themed so it follows the light/dark
// foreground color like the built-in toolbar icons.
func SyncIcon() fyne.Resource {
	iconData, err := iconFS.ReadFile("sync.svg")
	if err != nil {
		return theme.ViewRefreshIcon()
	}
	return theme.NewThemedResource(fyne.NewStaticResource("sync.svg", iconData))
}
