package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

var tourCommitHash = regexp.MustCompile(`^[a-f0-9]{40}([a-f0-9]{24})?$`)

type tourOutput struct {
	bytes.Buffer
	limit int
}

func (output *tourOutput) Write(data []byte) (int, error) {
	if len(data) > output.limit-output.Len() {
		return 0, fmt.Errorf("Git output exceeds tour source budget")
	}
	return output.Buffer.Write(data)
}

// Resolve refs once and read immutable objects. Textconv/external diffs and
// partial-clone lazy fetching are disabled; presentation creation executes no
// repository code or network operation.
func readGitTourSnapshot(ctx context.Context, repository, revision, path string) ([]byte, string, error) {
	if len(revision) > 256 || strings.ContainsAny(revision, "\x00\r\n") || !filepath.IsLocal(filepath.FromSlash(path)) || strings.ContainsAny(path, `\\:`) || !strings.HasSuffix(path, ".sir") {
		return nil, "", fmt.Errorf("Git tours need bounded revisions and relative .sir paths")
	}
	run := func(limit int, args ...string) ([]byte, error) {
		cmd := exec.CommandContext(ctx, "git", append([]string{"--no-optional-locks", "-c", "core.fsmonitor=false", "-C", repository}, args...)...)
		var env []string
		for _, entry := range os.Environ() {
			name, _, _ := strings.Cut(entry, "=")
			if strings.HasPrefix(strings.ToUpper(name), "GIT_") {
				continue
			}
			env = append(env, entry)
		}
		// The empty protocol whitelist also prevents lazy fetches with older
		// Git versions that do not recognize GIT_NO_LAZY_FETCH.
		cmd.Env = append(env, "GIT_NO_LAZY_FETCH=1", "GIT_OPTIONAL_LOCKS=0", "GIT_ALLOW_PROTOCOL=", "GIT_NO_REPLACE_OBJECTS=1", "GIT_LITERAL_PATHSPECS=1")
		output := &tourOutput{limit: limit}
		cmd.Stdout = output
		// Diagnostic text is not part of the generated source or report.
		cmd.Stderr = &tourOutput{limit: 8192}
		if err := cmd.Run(); err != nil {
			return nil, fmt.Errorf("read local Git snapshot: %w", err)
		}
		return append([]byte(nil), output.Bytes()...), nil
	}
	data, err := run(256, "rev-parse", "--verify", "--end-of-options", revision+"^{commit}")
	if err != nil {
		return nil, "", err
	}
	commit := strings.TrimSpace(string(data))
	if !tourCommitHash.MatchString(commit) {
		return nil, "", fmt.Errorf("Git did not return a canonical commit hash")
	}
	cleanPath := filepath.ToSlash(filepath.Clean(path))
	entry, err := run(4096, "ls-tree", "-z", commit, "--", cleanPath)
	if err != nil {
		return nil, "", err
	}
	metadata, entryPath, ok := strings.Cut(strings.TrimSuffix(string(entry), "\x00"), "\t")
	fields := strings.Fields(metadata)
	if !ok || entryPath != cleanPath || len(fields) != 3 || (fields[0] != "100644" && fields[0] != "100755") || fields[1] != "blob" {
		return nil, "", fmt.Errorf("Git tour sources must be regular committed files")
	}
	data, err = run(1<<20, "show", "--no-ext-diff", "--no-textconv", commit+":"+cleanPath)
	return data, commit, err
}
