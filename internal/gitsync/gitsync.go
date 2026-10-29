// Package gitsync synchronizes the password store's git repository with its
// remote. Sync figures out on its own what is needed (commit, pull, push or
// nothing at all) instead of blindly running every step.
package gitsync

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// defaultTimeout caps a single git invocation so an unreachable remote or a
// hung credential helper cannot keep the progress dialog open forever.
const defaultTimeout = 2 * time.Minute

// ErrConflict is returned by Sync when local and remote changes touch the
// same lines. The rebase is aborted, so the repository is left exactly as it
// was before the pull (local commits included).
var ErrConflict = errors.New("local and remote changes conflict")

// ErrUnfinished is returned by Sync when the repository is in the middle of a
// rebase or merge that has to be finished or aborted by hand first.
var ErrUnfinished = errors.New("the repository has an unfinished operation")

// Result describes what Sync did.
type Result struct {
	Committed   bool   // local changes were committed
	Pulled      int    // commits taken from the remote
	Pushed      int    // commits sent to the remote
	PublishedTo string // "<remote>/<branch>" when the branch had no upstream yet
}

// Changed reports whether Sync modified the working tree, i.e. whether the
// store has to be rescanned.
func (r Result) Changed() bool {
	return r.Pulled > 0
}

// Summary returns a human-readable description of r.
func (r Result) Summary() string {
	var parts []string
	if r.Committed {
		parts = append(parts, "committed local changes")
	}
	if r.Pulled > 0 {
		parts = append(parts, fmt.Sprintf("pulled %s", commits(r.Pulled)))
	}
	if r.Pushed > 0 {
		parts = append(parts, fmt.Sprintf("pushed %s", commits(r.Pushed)))
	}
	if r.PublishedTo != "" {
		parts = append(parts, "published the branch to "+r.PublishedTo)
	}
	if len(parts) == 0 {
		return "Already up to date."
	}
	s := strings.Join(parts, ", ") + "."
	return strings.ToUpper(s[:1]) + s[1:]
}

func commits(n int) string {
	if n == 1 {
		return "1 commit"
	}
	return fmt.Sprintf("%d commits", n)
}

// run runs git with args inside dir and returns its trimmed stdout. On
// failure git's output is folded into the error so the cause is visible in
// the UI.
func run(dir string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	// The app has no terminal: fail fast instead of waiting for a prompt
	// nobody can answer.
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	output, err := cmd.CombinedOutput()
	out := strings.TrimSpace(string(output))
	if err != nil {
		if ctx.Err() != nil {
			return out, fmt.Errorf("git %s timed out after %s", args[0], defaultTimeout)
		}
		return out, fmt.Errorf("git %s failed: %v\n%s", args[0], err, out)
	}
	return out, nil
}

// HasChanges reports whether the working tree at dir has uncommitted
// changes, including untracked files.
func HasChanges(dir string) (bool, error) {
	out, err := run(dir, "status", "--porcelain")
	if err != nil {
		return false, err
	}
	return out != "", nil
}

// CommitAll stages every change in the repository at dir and commits it
// with message. It refuses to run during an unfinished rebase or merge,
// where "add everything" would silently mark conflicts as resolved.
func CommitAll(dir, message string) error {
	if err := checkNoOperation(dir); err != nil {
		return err
	}
	if _, err := run(dir, "add", "-A"); err != nil {
		return err
	}
	_, err := run(dir, "commit", "-m", message)
	return err
}

// Sync brings the repository at dir in line with its upstream:
//
//  1. uncommitted changes are committed with commitMessage;
//  2. a branch without an upstream is pushed to the default remote and
//     starts tracking it;
//  3. otherwise the remote is fetched and the ahead/behind counts decide
//     what happens next: remote commits are rebased in, local commits are
//     pushed, and nothing is done when both sides already match.
//
// progress, if non-nil, receives a short description and the completed
// fraction before each step.
func Sync(dir, commitMessage string, progress func(step string, fraction float64)) (Result, error) {
	var res Result
	report := func(step string, fraction float64) {
		if progress != nil {
			progress(step, fraction)
		}
	}

	if _, err := run(dir, "rev-parse", "--is-inside-work-tree"); err != nil {
		return res, fmt.Errorf("%s is not a git repository", dir)
	}
	if err := checkNoOperation(dir); err != nil {
		return res, err
	}

	report("Checking local changes...", 0.1)
	changed, err := HasChanges(dir)
	if err != nil {
		return res, err
	}
	if changed {
		report("Committing local changes...", 0.2)
		if err := CommitAll(dir, commitMessage); err != nil {
			return res, err
		}
		res.Committed = true
	}

	if _, err := run(dir, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}"); err != nil {
		report("Publishing branch...", 0.6)
		return publish(dir, res)
	}

	report("Fetching from remote...", 0.4)
	if _, err := run(dir, "fetch", "--prune"); err != nil {
		return res, err
	}

	ahead, behind, err := aheadBehind(dir)
	if err != nil {
		return res, err
	}

	if behind > 0 {
		report("Pulling remote changes...", 0.6)
		// Rebase rather than merge so the history stays linear, like
		// `pass git pull --rebase`. With no local commits this is a plain
		// fast-forward.
		if _, err := run(dir, "rebase", "@{u}"); err != nil {
			if _, abortErr := run(dir, "rebase", "--abort"); abortErr != nil {
				return res, fmt.Errorf("%w; aborting the rebase failed too, fix the repository by hand: %v", ErrConflict, abortErr)
			}
			return res, fmt.Errorf("%w: the pull was undone, resolve it in %s by hand\n%v", ErrConflict, dir, err)
		}
		res.Pulled = behind
	}

	if ahead > 0 {
		report("Pushing local changes...", 0.8)
		if _, err := run(dir, "push"); err != nil {
			return res, err
		}
		res.Pushed = ahead
	}

	report("Done", 1)
	return res, nil
}

// checkNoOperation returns an ErrUnfinished error when a rebase or merge is
// in progress in the repository at dir.
func checkNoOperation(dir string) error {
	op, err := operationInProgress(dir)
	if err != nil || op == "" {
		return err
	}
	return fmt.Errorf("%w: a %s is in progress in %s, probably left by an earlier failed sync; finish it with \"git %s --continue\" or undo it with \"git %s --abort\"", ErrUnfinished, op, dir, op, op)
}

// operationInProgress returns "rebase" or "merge" when one of them was
// started in the repository at dir and not finished, and "" otherwise.
func operationInProgress(dir string) (string, error) {
	checks := []struct{ op, path string }{
		{"rebase", "rebase-merge"},
		{"rebase", "rebase-apply"},
		{"merge", "MERGE_HEAD"},
	}
	for _, c := range checks {
		p, err := run(dir, "rev-parse", "--path-format=absolute", "--git-path", c.path)
		if err != nil {
			return "", err
		}
		if _, err := os.Stat(p); err == nil {
			return c.op, nil
		}
	}
	return "", nil
}

// aheadBehind returns how many commits HEAD has that its upstream lacks
// (ahead) and the other way round (behind).
func aheadBehind(dir string) (ahead, behind int, err error) {
	out, err := run(dir, "rev-list", "--left-right", "--count", "HEAD...@{u}")
	if err != nil {
		return 0, 0, err
	}
	fields := strings.Fields(out)
	if len(fields) != 2 {
		return 0, 0, fmt.Errorf("unexpected git rev-list output: %q", out)
	}
	if ahead, err = strconv.Atoi(fields[0]); err != nil {
		return 0, 0, fmt.Errorf("unexpected git rev-list output: %q", out)
	}
	if behind, err = strconv.Atoi(fields[1]); err != nil {
		return 0, 0, fmt.Errorf("unexpected git rev-list output: %q", out)
	}
	return ahead, behind, nil
}

// publish pushes a branch that has no upstream yet to the default remote
// and makes it track the pushed branch.
func publish(dir string, res Result) (Result, error) {
	branch, err := run(dir, "symbolic-ref", "--short", "HEAD")
	if err != nil {
		return res, errors.New("HEAD is detached: check out a branch before syncing")
	}
	remote, err := defaultRemote(dir)
	if err != nil {
		return res, err
	}
	if _, err := run(dir, "push", "--set-upstream", remote, branch); err != nil {
		return res, err
	}
	res.PublishedTo = remote + "/" + branch
	return res, nil
}

// defaultRemote returns "origin" when it exists, otherwise the only
// configured remote.
func defaultRemote(dir string) (string, error) {
	out, err := run(dir, "remote")
	if err != nil {
		return "", err
	}
	remotes := strings.Fields(out)
	switch {
	case len(remotes) == 0:
		return "", errors.New("no git remote is configured for the password store")
	case len(remotes) == 1:
		return remotes[0], nil
	}
	for _, r := range remotes {
		if r == "origin" {
			return r, nil
		}
	}
	return "", fmt.Errorf("several git remotes (%s) and none is called origin: set an upstream for the branch", strings.Join(remotes, ", "))
}
