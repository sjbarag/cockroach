package main

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	// "golang.org/x/sync/errgroup"
	"github.com/cockroachdb/cockroach/pkg/build/bazel"
	"os/exec"
)

type Lockfile = map[string]LockfileEntry

type LockfileEntry struct {
	Version   string `json:"version"`
	Resolved  string `json:"resolved"`
	Integrity string `json:"integrity"`
}

func parseLockfiles(ctx context.Context, lockfiles ...string) ([]LockfileEntry, error) {
	// g, ctx := errgroup.WithContext(ctx)
	runfiles, err := bazel.RunfilesPath()
	if err != nil {
		return nil, fmt.Errorf("unable to get bazel runfiles path: %v", err)
	}

	indexJs := filepath.Join(runfiles, "pkg/cmd/mirror/ui/index.js")

	parsed := []LockfileEntry{}
	for _, path := range lockfiles {
		lockfilePath := filepath.Join(runfiles, path)
		out, err := exec.CommandContext(ctx, "bazel", "run", "@nodejs_host//:node", indexJs, lockfilePath).CombinedOutput()
		if err != nil {
			fmt.Printf("combined output: %s\n", string(out))
			return nil, fmt.Errorf("error running 'bazel run @nodejs_host//:node %s': %v", lockfilePath, err)
		}
		lf := new(Lockfile)
		if err := json.Unmarshal(out, lf); err != nil {
			return nil, fmt.Errorf("unable to unmarshal JSON for file %q: %v", lockfilePath, err)
		}
		for _, entry := range *lf {
			parsed = append(parsed, entry)
		}
	}

	return parsed, nil
}

func MustBazelRunfile(path string) string {
	out, err := bazel.Runfile(path)
	if err != nil {
		panic(err)
	}
	return out
}

func main() {
	lockfiles, err := parseLockfiles(
		context.Background(),
		"pkg/cmd/mirror/ui/yarn.lock",
		// MustBazelRunfile("//pkg/cmd/mirror/ui/yarn.lock"),
		// MustBazelRunfile("//pkg/ui/workspaces/cluster-ui:yarn.lock"),
	)
	if err != nil {
		fmt.Println("ERROR: ", err)
		return
	}
	fmt.Println(lockfiles)
}
