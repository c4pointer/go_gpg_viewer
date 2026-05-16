// Package clipboard copies sensitive strings (passwords) to the system
// clipboard and schedules their removal after a TTL, so a credential
// pasted once does not stay reachable for the rest of the session.
package clipboard

import "time"

// Clipboard is the minimal subset of fyne.Clipboard this package needs.
// Declaring it here keeps the package unit-testable without dragging
// the Fyne event loop into the test binary.
type Clipboard interface {
	Content() string
	SetContent(string)
}

// CopyWithAutoClear places text on the clipboard and schedules its
// removal after ttl. The clearing fires only if the clipboard is still
// holding text at that point — if the user copied something else in the
// meantime, their newer selection is preserved.
//
// Returns the scheduled *time.Timer so callers can cancel the auto-clear
// (Stop) if, e.g., the user explicitly clears the clipboard themselves.
// A zero or negative ttl skips scheduling entirely.
func CopyWithAutoClear(clip Clipboard, text string, ttl time.Duration) *time.Timer {
	clip.SetContent(text)
	if ttl <= 0 {
		return nil
	}
	return time.AfterFunc(ttl, func() {
		if clip.Content() == text {
			clip.SetContent("")
		}
	})
}
