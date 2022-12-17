package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"sync"

	"cloud.google.com/go/storage"
	"github.com/cockroachdb/cockroach/pkg/util/envutil"
	"golang.org/x/sync/errgroup"
	"google.golang.org/api/googleapi"
)

type Lockfile = map[string]LockfileEntry

type LockfileEntry struct {
	Version   string
	Resolved  *url.URL
	Integrity string
}

func (lfe *LockfileEntry) UnmarshalJSON(in []byte) error {
	type IntermediateEntry struct {
		Version   string `json:"version"`
		Resolved  string `json:"resolved"`
		Integrity string `json:"integrity"`
	}
	ie := new(IntermediateEntry)
	if err := json.Unmarshal(in, &ie); err != nil {
		return err
	}

	lfe.Version = ie.Version
	lfe.Integrity = ie.Integrity

	if ie.Resolved != "" {
		resolvedUrl, err := url.Parse(ie.Resolved)
		if err != nil {
			return err
		}
		lfe.Resolved = resolvedUrl
	}
	return nil
}

func canMirror() bool {
	return envutil.EnvOrDefaultBool("COCKROACH_BAZEL_CAN_MIRROR", false)
}

func parseLockfiles(jsonLockfiles []string) (map[string][]LockfileEntry, error) {
	entries := map[string][]LockfileEntry{}
	for _, path := range jsonLockfiles {
		lf := new(Lockfile)
		contents, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("unable to read lockfile %q: %v", path, err)
		}
		if err := json.Unmarshal(contents, &lf); err != nil {
			return nil, fmt.Errorf("unable to parse contents of %q as JSON: %v", path, err)
		}

		pathEntries := []LockfileEntry{}
		for name, entry := range *lf {
			if entry.Resolved != nil && entry.Resolved.String() == "" {
				return nil, fmt.Errorf("Something's weird with entry %q: %+v", name, entry)
			}
			pathEntries = append(pathEntries, entry)
		}
		entries[path] = pathEntries
	}

	return entries, nil
}

func mirrorDependencies(ctx context.Context, lockfiles map[string][]LockfileEntry) error {
	workdir, err := os.MkdirTemp("", "crdb-mirror-npm")
	if err != nil {
		return fmt.Errorf("unable top create temporary directory: %v", err)
	}

	client, err := storage.NewClient(ctx)
	if err != nil {
		return fmt.Errorf("unable to create Google Cloud Storage client: %v", err)
	}

	bucket := client.Bucket("barag-sandbox-crdb-mirror-npm")

	g, ctx := errgroup.WithContext(context.Background())
	g.SetLimit(runtime.GOMAXPROCS(0))
	for _, entries := range lockfiles {
		for _, entry := range entries {
			entry := entry
			g.Go(func() error {
				return mirrorLockfileEntry(ctx, bucket, workdir, entry)
			})
		}
	}
	return g.Wait()
}

var stderrLock sync.Mutex

func dbgLog(args ...interface{}) {
	stderrLock.Lock()
	defer stderrLock.Unlock()

	fmt.Fprintln(os.Stderr, args...)
}

const yarnRegistry = "registry.yarnpkg.com"
const npmjsComRegistry = "registry.npmjs.com"
const npmjsOrgRegistry = "registry.npmjs.org"

func mirrorLockfileEntry(ctx context.Context, bucket *storage.BucketHandle, tmpdir string, entry LockfileEntry) error {
	if entry.Resolved == nil {
		return nil
	}

	hostname := entry.Resolved.Hostname()
	if hostname != yarnRegistry && hostname != npmjsComRegistry && hostname != npmjsOrgRegistry {
		dbgLog("Skipping mirror for entry", entry)
		return nil
	}

	tgzUrl := entry.Resolved.String()
	dbgLog("Downloading file:", tgzUrl)

	// Download the file
	res, err := http.DefaultClient.Get(tgzUrl)
	if err != nil {
		return fmt.Errorf("unable to request file %q: %v", tgzUrl, err)
	}

	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(res.Body)
		defer res.Body.Close()
		return fmt.Errorf("received non-200 status code %q from %q. body = %s", res.Status, tgzUrl, body)
	}

	defer res.Body.Close()

	// Then upload it
	dbgLog("Uploading file:", entry.Resolved.Path)
	upload := bucket.Object(entry.Resolved.Path).If(storage.Conditions{DoesNotExist: true}).NewWriter(ctx)
	if _, err := io.Copy(upload, res.Body); err != nil {
		return fmt.Errorf("unexpected error while uploading %q: %v", entry.Resolved.Path, err)
	}

	if err := upload.Close(); err != nil {
		var gerr *googleapi.Error
		if errors.As(err, &gerr) {
			if gerr.Code == http.StatusPreconditionFailed {
				// In this case the "DoesNotExist" precondition
				// failed, i.e., the object does already exist.
				return nil
			}
			return gerr
		}
		return err
	}
	return nil
}

func main() {
	if !canMirror() {
		fmt.Println("Exiting without doing anything, since COCKROACH_BAZEL_CAN_MIRROR isn't truthy")
		os.Exit(0)
	}

	lockfiles, err := parseLockfiles(
		os.Args[1:],
	)
	if err != nil {
		fmt.Println("ERROR: ", err)
		os.Exit(1)
	}

	if err := mirrorDependencies(context.Background(), lockfiles); err != nil {
		fmt.Println("ERROR: ", err)
		os.Exit(1)
	}
}
