// Copyright 2017 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/alessio/shellescape"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/cockroachdb/cockroach/pkg/release"
	"github.com/cockroachdb/cockroach/pkg/testutils"
	"github.com/cockroachdb/errors"
	"github.com/stretchr/testify/require"
)

type mockS3 struct {
	puts []string
}

var _ s3putter = (*mockS3)(nil)

func (s *mockS3) PutObject(i *s3.PutObjectInput) (*s3.PutObjectOutput, error) {
	url := fmt.Sprintf(`s3://%s/%s`, *i.Bucket, *i.Key)
	if i.CacheControl != nil {
		url += `/` + *i.CacheControl
	}
	if i.Body != nil {
		bytes, err := ioutil.ReadAll(i.Body)
		if err != nil {
			return nil, err
		}
		if strings.HasSuffix(*i.Key, release.ChecksumSuffix) {
			// Unfortunately the archive tarball checksum changes every time,
			// because we generate tarballs and the copy file modification time from the generated files.
			// This makes the checksum not reproducible.
			s.puts = append(s.puts, fmt.Sprintf("%s CONTENTS <sha256sum>", url))
		} else if utf8.Valid(bytes) {
			s.puts = append(s.puts, fmt.Sprintf("%s CONTENTS %s", url, bytes))
		} else {
			s.puts = append(s.puts, fmt.Sprintf("%s CONTENTS <binary stuff>", url))
		}
	} else if i.WebsiteRedirectLocation != nil {
		s.puts = append(s.puts, fmt.Sprintf("%s REDIRECT %s", url, *i.WebsiteRedirectLocation))
	}
	return &s3.PutObjectOutput{}, nil
}

type mockExecRunner struct {
	fakeBazelBin string
	cmds         []string
}

func (r *mockExecRunner) run(c *exec.Cmd) ([]byte, error) {
	if r.fakeBazelBin == "" {
		panic("r.fakeBazelBin not set")
	}
	if c.Dir == `` {
		return nil, errors.Errorf(`Dir must be specified`)
	}
	cmd := fmt.Sprintf("env=%s args=%s", c.Env, shellescape.QuoteCommand(c.Args))
	r.cmds = append(r.cmds, cmd)

	var paths []string
	if c.Args[0] == "bazel" && c.Args[1] == "info" && c.Args[2] == "bazel-bin" {
		return []byte(r.fakeBazelBin), nil
	}
	if c.Args[0] == "bazel" && c.Args[1] == "build" && c.Args[2] == "//pkg/cmd/workload" {
		paths = append(paths, filepath.Join(r.fakeBazelBin, "pkg", "cmd", "workload", "workload_", "workload"))
	} else if c.Args[0] == "bazel" && c.Args[1] == "build" {
		path := filepath.Join(r.fakeBazelBin, "pkg", "cmd", "cockroach", "cockroach_", "cockroach")
		pathSQL := filepath.Join(r.fakeBazelBin, "pkg", "cmd", "cockroach-sql", "cockroach-sql_", "cockroach-sql")
		var platform release.Platform
		for _, arg := range c.Args {
			if strings.HasPrefix(arg, `--config=`) {
				switch strings.TrimPrefix(arg, `--config=`) {
				case "crosslinuxbase":
					platform = release.PlatformLinux
				case "crosslinuxarmbase":
					platform = release.PlatformLinuxArm
				case "crossmacosbase":
					platform = release.PlatformMacOS
				case "crosswindowsbase":
					platform = release.PlatformWindows
					path += ".exe"
					pathSQL += ".exe"
				case "ci", "force_build_cdeps", "with_ui":
				default:
					panic(fmt.Sprintf("Unexpected configuration %s", arg))
				}
			}
		}
		paths = append(paths, path, pathSQL)
		ext := release.SharedLibraryExtensionFromPlatform(platform)
		for _, lib := range release.CRDBSharedLibraries {
			libDir := "lib"
			if platform == release.PlatformWindows {
				libDir = "bin"
			}
			paths = append(paths, filepath.Join(r.fakeBazelBin, "c-deps", "libgeos_foreign", libDir, lib+ext))
		}
	}

	for _, path := range paths {
		if err := os.MkdirAll(filepath.Dir(path), 0777); err != nil {
			return nil, err
		}
		if err := ioutil.WriteFile(path, []byte(cmd), 0666); err != nil {
			return nil, err
		}
	}

	var output []byte
	return output, nil
}

func TestPublish(t *testing.T) {
	tests := []struct {
		name         string
		flags        runFlags
		expectedCmds []string
		expectedPuts []string
	}{
		{
			name: `release`,
			flags: runFlags{
				branch:     "master",
				sha:        "1234567890abcdef",
				bucketName: "cockroach",
			},
			expectedCmds: []string{
				"env=[] args=bazel build //pkg/cmd/cockroach //c-deps:libgeos //pkg/cmd/cockroach-sql " +
					"'--workspace_status_command=./build/bazelutil/stamp.sh x86_64-pc-linux-gnu official-binary' -c opt --config=ci --config=force_build_cdeps --config=with_ui --config=crosslinuxbase",
				"env=[] args=bazel info bazel-bin -c opt --config=ci --config=force_build_cdeps --config=with_ui --config=crosslinuxbase",
				"env=[MALLOC_CONF=prof:true] args=./cockroach.linux-2.6.32-gnu-amd64 version",
				"env=[] args=ldd ./cockroach.linux-2.6.32-gnu-amd64",
				"env=[] args=bazel build //pkg/cmd/cockroach //c-deps:libgeos //pkg/cmd/cockroach-sql " +
					"'--workspace_status_command=./build/bazelutil/stamp.sh x86_64-apple-darwin19 official-binary' -c opt --config=ci --config=force_build_cdeps --config=with_ui --config=crossmacosbase",
				"env=[] args=bazel info bazel-bin -c opt --config=ci --config=force_build_cdeps --config=with_ui --config=crossmacosbase",
				"env=[] args=bazel build //pkg/cmd/cockroach //c-deps:libgeos //pkg/cmd/cockroach-sql " +
					"'--workspace_status_command=." +
					"/build/bazelutil/stamp.sh x86_64-w64-mingw32 official-binary' -c opt --config=ci --config=force_build_cdeps --config=with_ui --config=crosswindowsbase",
				"env=[] args=bazel info bazel-bin -c opt --config=ci --config=force_build_cdeps --config=with_ui --config=crosswindowsbase",
				"env=[] args=bazel build //pkg/cmd/workload -c opt --config=crosslinux --config=ci",
				"env=[] args=bazel info bazel-bin -c opt --config=crosslinux --config=ci",
			},
			expectedPuts: []string{
				"s3://cockroach//cockroach/cockroach.linux-gnu-amd64.1234567890abcdef CONTENTS env=[] args=bazel build " +
					"//pkg/cmd/cockroach //c-deps:libgeos //pkg/cmd/cockroach-sql " +
					"'--workspace_status_command=./build/bazelutil/stamp.sh x86_64-pc-linux-gnu official-binary' " +
					"-c opt --config=ci --config=force_build_cdeps --config=with_ui --config=crosslinuxbase",
				"s3://cockroach/cockroach/cockroach.linux-gnu-amd64.LATEST/no-cache REDIRECT /cockroach/cockroach.linux-gnu-amd64.1234567890abcdef",
				"s3://cockroach//cockroach/cockroach-sql.linux-gnu-amd64.1234567890abcdef CONTENTS env=[] args=bazel build " +
					"//pkg/cmd/cockroach //c-deps:libgeos //pkg/cmd/cockroach-sql " +
					"'--workspace_status_command=./build/bazelutil/stamp.sh x86_64-pc-linux-gnu official-binary' " +
					"-c opt --config=ci --config=force_build_cdeps --config=with_ui --config=crosslinuxbase",
				"s3://cockroach/cockroach/cockroach-sql.linux-gnu-amd64.LATEST/no-cache REDIRECT /cockroach/cockroach-sql.linux-gnu-amd64.1234567890abcdef",
				"s3://cockroach//cockroach/lib/libgeos.linux-gnu-amd64.1234567890abcdef.so CONTENTS env=[] args=bazel build " +
					"//pkg/cmd/cockroach //c-deps:libgeos //pkg/cmd/cockroach-sql " +
					"'--workspace_status_command=./build/bazelutil/stamp.sh x86_64-pc-linux-gnu official-binary' " +
					"-c opt --config=ci --config=force_build_cdeps --config=with_ui --config=crosslinuxbase",
				"s3://cockroach/cockroach/lib/libgeos.linux-gnu-amd64.so.LATEST/no-cache REDIRECT /cockroach/lib/libgeos.linux-gnu-amd64.1234567890abcdef.so",
				"s3://cockroach//cockroach/lib/libgeos_c.linux-gnu-amd64.1234567890abcdef.so CONTENTS env=[] args=bazel build " +
					"//pkg/cmd/cockroach //c-deps:libgeos //pkg/cmd/cockroach-sql " +
					"'--workspace_status_command=./build/bazelutil/stamp.sh x86_64-pc-linux-gnu official-binary' " +
					"-c opt --config=ci --config=force_build_cdeps --config=with_ui --config=crosslinuxbase",
				"s3://cockroach/cockroach/lib/libgeos_c.linux-gnu-amd64.so.LATEST/no-cache REDIRECT /cockroach/lib/libgeos_c.linux-gnu-amd64.1234567890abcdef.so",
				"s3://cockroach//cockroach/cockroach.darwin-amd64.1234567890abcdef CONTENTS env=[] args=bazel build " +
					"//pkg/cmd/cockroach //c-deps:libgeos //pkg/cmd/cockroach-sql " +
					"'--workspace_status_command=./build/bazelutil/stamp.sh x86_64-apple-darwin19 official-binary' " +
					"-c opt --config=ci --config=force_build_cdeps --config=with_ui --config=crossmacosbase",
				"s3://cockroach/cockroach/cockroach.darwin-amd64.LATEST/no-cache REDIRECT /cockroach/cockroach.darwin-amd64.1234567890abcdef",
				"s3://cockroach//cockroach/cockroach-sql.darwin-amd64.1234567890abcdef CONTENTS env=[] args=bazel build " +
					"//pkg/cmd/cockroach //c-deps:libgeos //pkg/cmd/cockroach-sql " +
					"'--workspace_status_command=./build/bazelutil/stamp.sh x86_64-apple-darwin19 official-binary' " +
					"-c opt --config=ci --config=force_build_cdeps --config=with_ui --config=crossmacosbase",
				"s3://cockroach/cockroach/cockroach-sql.darwin-amd64.LATEST/no-cache REDIRECT /cockroach/cockroach-sql.darwin-amd64.1234567890abcdef",
				"s3://cockroach//cockroach/lib/libgeos.darwin-amd64.1234567890abcdef.dylib CONTENTS env=[] args=bazel build " +
					"//pkg/cmd/cockroach //c-deps:libgeos //pkg/cmd/cockroach-sql " +
					"'--workspace_status_command=./build/bazelutil/stamp.sh x86_64-apple-darwin19 official-binary' " +
					"-c opt --config=ci --config=force_build_cdeps --config=with_ui --config=crossmacosbase",
				"s3://cockroach/cockroach/lib/libgeos.darwin-amd64.dylib.LATEST/no-cache REDIRECT /cockroach/lib/libgeos.darwin-amd64.1234567890abcdef.dylib",
				"s3://cockroach//cockroach/lib/libgeos_c.darwin-amd64.1234567890abcdef.dylib CONTENTS env=[] args=bazel build " +
					"//pkg/cmd/cockroach //c-deps:libgeos //pkg/cmd/cockroach-sql " +
					"'--workspace_status_command=./build/bazelutil/stamp.sh x86_64-apple-darwin19 official-binary' " +
					"-c opt --config=ci --config=force_build_cdeps --config=with_ui --config=crossmacosbase",
				"s3://cockroach/cockroach/lib/libgeos_c.darwin-amd64.dylib.LATEST/no-cache REDIRECT /cockroach/lib/libgeos_c.darwin-amd64.1234567890abcdef.dylib",
				"s3://cockroach//cockroach/cockroach.windows-amd64.1234567890abcdef.exe CONTENTS env=[] args=bazel build " +
					"//pkg/cmd/cockroach //c-deps:libgeos //pkg/cmd/cockroach-sql " +
					"'--workspace_status_command=./build/bazelutil/stamp.sh x86_64-w64-mingw32 official-binary' " +
					"-c opt --config=ci --config=force_build_cdeps --config=with_ui --config=crosswindowsbase",
				"s3://cockroach/cockroach/cockroach.windows-amd64.LATEST/no-cache REDIRECT /cockroach/cockroach.windows-amd64.1234567890abcdef.exe",
				"s3://cockroach//cockroach/cockroach-sql.windows-amd64.1234567890abcdef.exe CONTENTS env=[] args=bazel build " +
					"//pkg/cmd/cockroach //c-deps:libgeos //pkg/cmd/cockroach-sql " +
					"'--workspace_status_command=./build/bazelutil/stamp.sh x86_64-w64-mingw32 official-binary' " +
					"-c opt --config=ci --config=force_build_cdeps --config=with_ui --config=crosswindowsbase",
				"s3://cockroach/cockroach/cockroach-sql.windows-amd64.LATEST/no-cache REDIRECT /cockroach/cockroach-sql.windows-amd64.1234567890abcdef.exe",
				"s3://cockroach//cockroach/lib/libgeos.windows-amd64.1234567890abcdef.dll CONTENTS env=[] args=bazel build " +
					"//pkg/cmd/cockroach //c-deps:libgeos //pkg/cmd/cockroach-sql " +
					"'--workspace_status_command=./build/bazelutil/stamp.sh x86_64-w64-mingw32 official-binary' " +
					"-c opt --config=ci --config=force_build_cdeps --config=with_ui --config=crosswindowsbase",
				"s3://cockroach/cockroach/lib/libgeos.windows-amd64.dll.LATEST/no-cache REDIRECT /cockroach/lib/libgeos.windows-amd64.1234567890abcdef.dll",
				"s3://cockroach//cockroach/lib/libgeos_c.windows-amd64.1234567890abcdef.dll CONTENTS env=[] args=bazel build " +
					"//pkg/cmd/cockroach //c-deps:libgeos //pkg/cmd/cockroach-sql " +
					"'--workspace_status_command=./build/bazelutil/stamp.sh x86_64-w64-mingw32 official-binary' " +
					"-c opt --config=ci --config=force_build_cdeps --config=with_ui --config=crosswindowsbase",
				"s3://cockroach/cockroach/lib/libgeos_c.windows-amd64.dll.LATEST/no-cache REDIRECT /cockroach/lib/libgeos_c.windows-amd64.1234567890abcdef.dll",
				"s3://cockroach//cockroach/workload.1234567890abcdef CONTENTS env=[] args=bazel build //pkg/cmd/workload -c opt --config=crosslinux --config=ci",
				"s3://cockroach/cockroach/workload.LATEST/no-cache REDIRECT /cockroach/workload.1234567890abcdef",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir, cleanup := testutils.TempDir(t)
			defer cleanup()

			var s3 mockS3
			var exec mockExecRunner
			fakeBazelBin, cleanup := testutils.TempDir(t)
			defer cleanup()
			exec.fakeBazelBin = fakeBazelBin
			flags := test.flags
			flags.pkgDir = dir
			execFn := release.ExecFn{MockExecFn: exec.run}
			run(&s3, flags, execFn)
			require.Equal(t, test.expectedCmds, exec.cmds)
			require.Equal(t, test.expectedPuts, s3.puts)
		})
	}
}
