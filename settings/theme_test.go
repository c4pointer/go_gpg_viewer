package settings

import (
	"testing"

	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplyTheme(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()

	ApplyTheme(app, "dark")
	dark, ok := app.Settings().Theme().(*fixedVariant)
	require.True(t, ok)
	assert.Equal(t, theme.VariantDark, dark.variant)
	// The pinned variant must win over whatever the OS asks for.
	assert.Equal(t, theme.DefaultTheme().Color(theme.ColorNameBackground, theme.VariantDark),
		dark.Color(theme.ColorNameBackground, theme.VariantLight))

	ApplyTheme(app, "light")
	light, ok := app.Settings().Theme().(*fixedVariant)
	require.True(t, ok)
	assert.Equal(t, theme.VariantLight, light.variant)

	ApplyTheme(app, "system")
	_, pinned := app.Settings().Theme().(*fixedVariant)
	assert.False(t, pinned, "system must follow the OS preference")

	ApplyTheme(app, "unknown")
	_, pinned = app.Settings().Theme().(*fixedVariant)
	assert.False(t, pinned)
}

func TestGetAvailableThemes(t *testing.T) {
	themes := GetAvailableThemes()
	assert.ElementsMatch(t, []string{"system", "light", "dark"}, themes)
	// system must come first so the settings dialog presents it as the
	// recommended default.
	assert.Equal(t, "system", themes[0])
}

func TestGetThemeDisplayName(t *testing.T) {
	assert.Equal(t, "Dark", GetThemeDisplayName("dark"))
	assert.Equal(t, "Light", GetThemeDisplayName("light"))
	assert.Equal(t, "Follow system", GetThemeDisplayName("system"))
	assert.Equal(t, "Follow system", GetThemeDisplayName("unknown"))
}
