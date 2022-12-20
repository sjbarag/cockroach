package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"cloud.google.com/go/storage"
	"golang.org/x/sync/errgroup"
	"google.golang.org/api/googleapi"
)

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
	uploadPath, err := filepath.Rel("/", entry.Resolved.Path)
	if err != nil {
		return fmt.Errorf("could not relativize path %q", entry.Resolved.Path)
	}
	dbgLog("Uploading file:", entry.Resolved.Path)
	upload := bucket.Object(uploadPath).If(storage.Conditions{DoesNotExist: true}).NewWriter(ctx)
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
			if entry.Resolved == nil || entry.Resolved.Host == "storage.googleapis.com" {
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
		fmt.Fprintf(os.Stderr, "generating new json for %q\n", filename)
		out := Lockfile{}
		for _, entry := range entries {
			out[entry.Name] = entry
		}
		asJson, err := json.Marshal(out)
		if err != nil {
			return fmt.Errorf("unable to marshal new lockfile to JSON: %v", err)
		}
		outname := filename + ".new"
		fmt.Fprintf(os.Stderr, "writing new file %q\n", outname)
		if err := os.WriteFile(outname, asJson, os.ModePerm); err != nil {
			return fmt.Errorf("unable to write new file %q: %v", outname, err)
		}
	}

	return nil
}

func main() {
	var shouldMirror = flag.Bool("mirror", false, "mirrors dependencies to GCS instead of regenerate yarn.lock files.")
	flag.Parse()

	fmt.Fprintf(os.Stderr, "len(os.Args) = %d\n", flag.NArg())
	fmt.Fprintf(os.Stderr, "os.Args = %v\n", flag.Args())

	lockfiles, err := parseLockfiles(flag.Args())
	if err != nil {
		fmt.Println("ERROR: ", err)
		os.Exit(1)
	}

	ctx := context.Background()
	if shouldMirror != nil && *shouldMirror {
		fmt.Fprintln(os.Stderr, "INFO: mirroring dependencies to GCS")
		if err := mirrorDependencies(ctx, lockfiles); err != nil {
			fmt.Println("ERROR: ", err)
			os.Exit(1)
		}
	} else {
		fmt.Fprintln(os.Stderr, "INFO: regenerating *.yarn.json files with GCS URLs")
		if err := updateLockfileUrls(ctx, lockfiles); err != nil {
			fmt.Println("ERROR: ", err)
			os.Exit(1)
		}

		if err := writeNewLockfileJsons(ctx, lockfiles); err != nil {
			fmt.Println("ERROR: ", err)
			os.Exit(1)
		}
	}
}
