package settings

import (
	"net/url"
	"testing"

	"fyne.io/fyne/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockApp struct {
	mock.Mock
	settings fyne.Settings
}

func (m *mockApp) Settings() fyne.Settings {
	m.Called()
	return m.settings
}

func (m *mockApp) SetSettings(s fyne.Settings) {
	m.settings = s
}

// Only implement the methods needed for the test
func (m *mockApp) Driver() fyne.Driver                          { return nil }
func (m *mockApp) Run()                                         {}
func (m *mockApp) Quit()                                        {}
func (m *mockApp) NewWindow(title string) fyne.Window           { return nil }
func (m *mockApp) OpenURL(url *url.URL) error                   { return nil }
func (m *mockApp) SendNotification(n *fyne.Notification)        {}
func (m *mockApp) Icon() fyne.Resource                          { return nil }
func (m *mockApp) SetIcon(icon fyne.Resource)                   {}
func (m *mockApp) UniqueID() string                             { return "mock" }
func (m *mockApp) Lifecycle() fyne.Lifecycle                    { return nil }
func (m *mockApp) Clipboard() fyne.Clipboard                    { return nil }
func (m *mockApp) CloudProvider() fyne.CloudProvider            { return nil }
func (m *mockApp) Metadata() fyne.AppMetadata                   { return fyne.AppMetadata{} }
func (m *mockApp) Preferences() fyne.Preferences                { return nil }
func (m *mockApp) SetCloudProvider(provider fyne.CloudProvider) {}
func (m *mockApp) Storage() fyne.Storage                        { return nil }

// MockSettings implements fyne.Settings
// Only implement SetTheme for test
type mockSettings struct {
	mock.Mock
	lastTheme fyne.Theme
}

func (m *mockSettings) SetTheme(theme fyne.Theme) {
	m.lastTheme = theme
	m.Called(theme)
}
func (m *mockSettings) Theme() fyne.Theme                    { return m.lastTheme }
func (m *mockSettings) PrimaryColor() string                 { return "" }
func (m *mockSettings) Scale() float32                       { return 1.0 }
func (m *mockSettings) SetScale(float32)                     {}
func (m *mockSettings) SetThemeVariant(fyne.ThemeVariant)    {}
func (m *mockSettings) ThemeVariant() fyne.ThemeVariant      { return 0 }
func (m *mockSettings) AddChangeListener(chan fyne.Settings) {}
func (m *mockSettings) AddListener(func(fyne.Settings))      {}
func (m *mockSettings) BuildType() fyne.BuildType            { return fyne.BuildRelease }
func (m *mockSettings) ShowAnimations() bool                 { return true }

func TestApplyTheme(t *testing.T) {
	app := new(mockApp)
	settings := new(mockSettings)
	app.SetSettings(settings)
	app.On("Settings").Return(settings)

	settings.On("SetTheme", mock.Anything).Return().Once()
	ApplyTheme(app, "dark")
	settings.AssertCalled(t, "SetTheme", mock.Anything)

	settings.On("SetTheme", mock.Anything).Return().Once()
	ApplyTheme(app, "light")
	settings.AssertCalled(t, "SetTheme", mock.Anything)

	settings.On("SetTheme", mock.Anything).Return().Once()
	ApplyTheme(app, "unknown")
	settings.AssertCalled(t, "SetTheme", mock.Anything)
}

func TestGetAvailableThemes(t *testing.T) {
	themes := GetAvailableThemes()
	assert.ElementsMatch(t, []string{"light", "dark"}, themes)
}

func TestGetThemeDisplayName(t *testing.T) {
	assert.Equal(t, "Dark Theme", GetThemeDisplayName("dark"))
	assert.Equal(t, "Light Theme", GetThemeDisplayName("light"))
	assert.Equal(t, "Light Theme", GetThemeDisplayName("unknown"))
}
