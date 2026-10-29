package gitsync

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// isolateGit keeps the developer's git config (signing, hooks, aliases) out
// of the tests and gives commits a fixed identity.
func isolateGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_AUTHOR_NAME", "Test")
	t.Setenv("GIT_AUTHOR_EMAIL", "test@example.invalid")
	t.Setenv("GIT_COMMITTER_NAME", "Test")
	t.Setenv("GIT_COMMITTER_EMAIL", "test@example.invalid")
}

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := run(dir, args...)
	require.NoError(t, err)
	return out
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0600))
}

// setup creates a bare remote with one commit and two clones tracking it.
func setup(t *testing.T) (remote, a, b string) {
	t.Helper()
	isolateGit(t)
	root := t.TempDir()
	remote = filepath.Join(root, "remote.git")
	a = filepath.Join(root, "a")
	b = filepath.Join(root, "b")

	git(t, root, "init", "--quiet", "--bare", "--initial-branch=main", remote)
	git(t, root, "clone", "--quiet", remote, a)
	git(t, a, "checkout", "--quiet", "-b", "main")
	writeFile(t, a, "first.gpg", "1")
	git(t, a, "add", "-A")
	git(t, a, "commit", "--quiet", "-m", "first")
	git(t, a, "push", "--quiet", "--set-upstream", "origin", "main")
	git(t, root, "clone", "--quiet", remote, b)
	return remote, a, b
}

func TestSyncUpToDateDoesNothing(t *testing.T) {
	_, a, _ := setup(t)
	before := git(t, a, "rev-parse", "HEAD")

	res, err := Sync(a, "auto", nil)
	require.NoError(t, err)
	assert.Equal(t, Result{}, res)
	assert.Equal(t, "Already up to date.", res.Summary())
	assert.Equal(t, before, git(t, a, "rev-parse", "HEAD"))
}

func TestSyncCommitsAndPushesLocalChanges(t *testing.T) {
	remote, a, _ := setup(t)
	writeFile(t, a, "Mail/gmail.gpg", "x")

	res, err := Sync(a, "auto", nil)
	require.NoError(t, err)
	assert.Equal(t, Result{Committed: true, Pushed: 1}, res)
	assert.False(t, res.Changed())
	assert.Equal(t, git(t, a, "rev-parse", "HEAD"), git(t, remote, "rev-parse", "main"))
	assert.Equal(t, "auto", git(t, a, "log", "-1", "--format=%s"))
}

func TestSyncPullsRemoteChanges(t *testing.T) {
	_, a, b := setup(t)
	writeFile(t, b, "from-b.gpg", "b")
	_, err := Sync(b, "from b", nil)
	require.NoError(t, err)

	progressSteps := 0
	res, err := Sync(a, "auto", func(string, float64) { progressSteps++ })
	require.NoError(t, err)
	assert.Equal(t, Result{Pulled: 1}, res)
	assert.True(t, res.Changed())
	assert.FileExists(t, filepath.Join(a, "from-b.gpg"))
	assert.Positive(t, progressSteps)
}

func TestSyncRebasesDivergedHistoryAndPushes(t *testing.T) {
	remote, a, b := setup(t)
	writeFile(t, b, "from-b.gpg", "b")
	_, err := Sync(b, "from b", nil)
	require.NoError(t, err)
	writeFile(t, a, "from-a.gpg", "a")

	res, err := Sync(a, "from a", nil)
	require.NoError(t, err)
	assert.Equal(t, Result{Committed: true, Pulled: 1, Pushed: 1}, res)
	assert.Equal(t, "Committed local changes, pulled 1 commit, pushed 1 commit.", res.Summary())
	assert.FileExists(t, filepath.Join(a, "from-b.gpg"))
	assert.Equal(t, git(t, a, "rev-parse", "HEAD"), git(t, remote, "rev-parse", "main"))
	// Linear history: no merge commit.
	assert.Empty(t, git(t, a, "rev-list", "--merges", "HEAD"))
}

func TestSyncConflictIsAbortedAndKeepsLocalWork(t *testing.T) {
	_, a, b := setup(t)
	writeFile(t, b, "first.gpg", "from b")
	_, err := Sync(b, "from b", nil)
	require.NoError(t, err)
	writeFile(t, a, "first.gpg", "from a")

	res, err := Sync(a, "from a", nil)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrConflict))
	assert.True(t, res.Committed)
	assert.Zero(t, res.Pushed)

	// The rebase was aborted: no rebase in progress and the local commit is intact.
	assert.NoDirExists(t, filepath.Join(a, ".git", "rebase-merge"))
	assert.NoDirExists(t, filepath.Join(a, ".git", "rebase-apply"))
	assert.Equal(t, "from a", git(t, a, "log", "-1", "--format=%s"))
	data, err := os.ReadFile(filepath.Join(a, "first.gpg"))
	require.NoError(t, err)
	assert.Equal(t, "from a", string(data))
}

func TestSyncPublishesBranchWithoutUpstream(t *testing.T) {
	remote, a, _ := setup(t)
	git(t, a, "checkout", "--quiet", "-b", "laptop")
	writeFile(t, a, "laptop.gpg", "l")

	res, err := Sync(a, "auto", nil)
	require.NoError(t, err)
	assert.Equal(t, Result{Committed: true, PublishedTo: "origin/laptop"}, res)
	assert.Equal(t, git(t, a, "rev-parse", "HEAD"), git(t, remote, "rev-parse", "laptop"))
	assert.Equal(t, "origin/laptop", git(t, a, "rev-parse", "--abbrev-ref", "@{u}"))
}

func TestSyncWithoutRemote(t *testing.T) {
	isolateGit(t)
	dir := t.TempDir()
	git(t, dir, "init", "--quiet")
	writeFile(t, dir, "x.gpg", "x")

	_, err := Sync(dir, "auto", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no git remote")
}

func TestSyncOutsideRepository(t *testing.T) {
	isolateGit(t)
	_, err := Sync(t.TempDir(), "auto", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a git repository")
}

func TestSyncRefusesUnfinishedRebase(t *testing.T) {
	_, a, b := setup(t)
	writeFile(t, b, "first.gpg", "from b")
	_, err := Sync(b, "from b", nil)
	require.NoError(t, err)
	writeFile(t, a, "first.gpg", "from a")
	git(t, a, "commit", "--quiet", "-am", "from a")
	git(t, a, "fetch", "--quiet")
	// Reproduce what the old sync left behind: a rebase stopped on a conflict.
	_, err = run(a, "rebase", "@{u}")
	require.Error(t, err)

	_, err = Sync(a, "auto", nil)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrUnfinished))
	assert.Contains(t, err.Error(), "git rebase --abort")
	// Sync must not have touched the stopped rebase.
	assert.DirExists(t, filepath.Join(a, ".git", "rebase-merge"))
}

func TestCommitAllRefusesUnfinishedRebase(t *testing.T) {
	_, a, b := setup(t)
	writeFile(t, b, "first.gpg", "from b")
	_, err := Sync(b, "from b", nil)
	require.NoError(t, err)
	writeFile(t, a, "first.gpg", "from a")
	git(t, a, "commit", "--quiet", "-am", "from a")
	git(t, a, "fetch", "--quiet")
	_, err = run(a, "rebase", "@{u}")
	require.Error(t, err)

	err = CommitAll(a, "manual")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrUnfinished))
	assert.NotEmpty(t, git(t, a, "diff", "--name-only", "--diff-filter=U"), "conflict must stay unresolved")
}
