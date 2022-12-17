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
type Lockfiles = map[string][]LockfileEntry

type LockfileEntry struct {
	Name      string
	Version   string
	Resolved  *url.URL
	Integrity string
}

type IntermediateEntry struct {
	Version   string `json:"version,omitempty"`
	Resolved  string `json:"resolved,omitempty"`
	Integrity string `json:"integrity,omitempty"`
}

func (lfe *LockfileEntry) UnmarshalJSON(in []byte) error {
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

func (lfe *LockfileEntry) MarshalJSON() ([]byte, error) {
	ie := new(IntermediateEntry)
	ie.Version = lfe.Version
	ie.Integrity = lfe.Integrity
	if lfe.Resolved != nil {
		ie.Resolved = lfe.Resolved.String()
	}

	return json.Marshal(ie)
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
			entry.Name = name
			pathEntries = append(pathEntries, entry)
		}
		entries[path] = pathEntries
	}

	return entries, nil
}

const mirrorBucketName = "barag-sandbox-crdb-mirror-npm"

func mirrorDependencies(ctx context.Context, lockfiles Lockfiles) error {
	client, err := storage.NewClient(ctx)
	if err != nil {
		return fmt.Errorf("unable to create Google Cloud Storage client: %v", err)
	}

	bucket := client.Bucket(mirrorBucketName)

	g, ctx := errgroup.WithContext(context.Background())
	g.SetLimit(runtime.GOMAXPROCS(0))
	for _, entries := range lockfiles {
		for _, entry := range entries {
			entry := entry
			g.Go(func() error {
				return mirrorLockfileEntry(ctx, bucket, entry)
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

func mirrorLockfileEntry(ctx context.Context, bucket *storage.BucketHandle, entry LockfileEntry) error {
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

func updateLockfileUrls(ctx context.Context, lockfiles Lockfiles) error {
	for _, entries := range lockfiles {
		for _, entry := range entries {
			if entry.Resolved == nil {
				continue
			}
			newPath, err := url.JoinPath(mirrorBucketName, entry.Resolved.Path)
			if err != nil {
				return fmt.Errorf("unable to rewrite URL %q: %v", entry.Resolved.String(), err)
			}
			entry.Resolved.Path = newPath
			entry.Resolved.Host = "storage.googleapis.com"
		}
	}
	return nil
}

func writeNewLockfileJsons(ctx context.Context, lockfiles Lockfiles) error {
	for filename, entries := range lockfiles {
		fmt.Fprintf(os.Stderr, "generating new json for %q", filename)
		out := Lockfile{}
		for _, entry := range entries {
			out[entry.Name] = entry
		}
		asJson, err := json.Marshal(out)
		if err != nil {
			return fmt.Errorf("unable to marshal new lockfile to JSON: %v", err)
		}
		outname := filename + ".new"
		fmt.Fprintf(os.Stderr, "writing new file %q", outname)
		if err := os.WriteFile(filename+".new", asJson, os.ModePerm); err != nil {
			return fmt.Errorf("unable to write new file %q: %v", outname, err)
		}
	}

	return nil
}

func main() {
	if !canMirror() {
		fmt.Println("Exiting without doing anything, since COCKROACH_BAZEL_CAN_MIRROR isn't truthy")
		os.Exit(0)
	}

	fmt.Fprintf(os.Stderr, "len(os.Args) = %d\n", len(os.Args))
	fmt.Fprintf(os.Stderr, "os.Args = %v\n", os.Args)

	lockfiles, err := parseLockfiles(os.Args[1:])
	if err != nil {
		fmt.Println("ERROR: ", err)
		os.Exit(1)
	}

	ctx := context.Background()
	if err := mirrorDependencies(ctx, lockfiles); err != nil {
		fmt.Println("ERROR: ", err)
		os.Exit(1)
	}

	if err := updateLockfileUrls(ctx, lockfiles); err != nil {
		fmt.Println("ERROR: ", err)
		os.Exit(1)
	}

	if err := writeNewLockfileJsons(ctx, lockfiles); err != nil {
		fmt.Println("ERROR: ", err)
		os.Exit(1)
	}
}
