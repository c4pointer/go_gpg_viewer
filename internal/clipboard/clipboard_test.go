package clipboard

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeClipboard is a goroutine-safe in-memory Clipboard double so the
// auto-clear timer (which fires on a separate goroutine) doesn't race
// the test assertions.
type fakeClipboard struct {
	mu      sync.Mutex
	content string
}

func (f *fakeClipboard) Content() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.content
}

func (f *fakeClipboard) SetContent(s string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.content = s
}

func TestCopyImmediatelyPopulatesClipboard(t *testing.T) {
	clip := &fakeClipboard{}
	CopyWithAutoClear(clip, "hunter2", 0)
	assert.Equal(t, "hunter2", clip.Content())
}

func TestAutoClearAfterTTL(t *testing.T) {
	clip := &fakeClipboard{}
	timer := CopyWithAutoClear(clip, "hunter2", 30*time.Millisecond)
	require.NotNil(t, timer)
	t.Cleanup(func() { timer.Stop() })

	assert.Equal(t, "hunter2", clip.Content())

	time.Sleep(80 * time.Millisecond)
	assert.Equal(t, "", clip.Content(), "clipboard should be cleared after TTL")
}

func TestAutoClearDoesNotOverwriteNewerContent(t *testing.T) {
	clip := &fakeClipboard{}
	timer := CopyWithAutoClear(clip, "hunter2", 30*time.Millisecond)
	require.NotNil(t, timer)
	t.Cleanup(func() { timer.Stop() })

	// User (or another app) copies something else before TTL fires.
	clip.SetContent("user picked this")

	time.Sleep(80 * time.Millisecond)
	assert.Equal(t, "user picked this", clip.Content(),
		"the user's newer selection must be preserved")
}

func TestZeroTTLSkipsScheduling(t *testing.T) {
	clip := &fakeClipboard{}
	timer := CopyWithAutoClear(clip, "hunter2", 0)
	assert.Nil(t, timer, "ttl=0 must not schedule a timer")
	assert.Equal(t, "hunter2", clip.Content())
}

func TestStoppedTimerLeavesClipboardAlone(t *testing.T) {
	clip := &fakeClipboard{}
	timer := CopyWithAutoClear(clip, "hunter2", 30*time.Millisecond)
	require.NotNil(t, timer)
	timer.Stop()

	time.Sleep(60 * time.Millisecond)
	assert.Equal(t, "hunter2", clip.Content(),
		"after Stop, the auto-clear must not fire")
}
