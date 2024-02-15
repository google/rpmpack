// Copyright 2019 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package rpmpack

import (
	"archive/tar"
	"bytes"
	"io"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// create a test tar file
func createTar(t *testing.T) io.Reader {
	t.Helper()
	b := &bytes.Buffer{}
	ta := tar.NewWriter(b)
	entries := []struct {
		hdr  *tar.Header
		body []byte
	}{{
		hdr: &tar.Header{
			Name: "dir1/",
			Mode: 0755,
		},
	}, {
		hdr: &tar.Header{
			Typeflag: tar.TypeSymlink,
			Name:     "dir1/symlink1",
			Linkname: "../symtarget",
		},
	}, {
		hdr: &tar.Header{
			Name: "dir1/testfile1.txt",
			Mode: 0644,
			Size: int64(len("content1")),
		},
		body: []byte("content1"),
	}}

	for _, e := range entries {
		if err := ta.WriteHeader(e.hdr); err != nil {
			t.Errorf("failed to write header %s: %v", e.hdr.Name, err)
		}
		if e.hdr.Size != 0 {
			if _, err := ta.Write(e.body); err != nil {
				t.Errorf("failed to write body %s: %v", e.hdr.Name, err)
			}
		}

	}
	return b
}

func TestFromTar(t *testing.T) {
	testCases := []struct {
		name          string
		input         io.Reader
		wantBasenames []string
		wantFileModes []uint16
	}{{
		name:          "simple tar",
		input:         createTar(t),
		wantBasenames: []string{"dir1", "symlink1", "testfile1.txt"},
		wantFileModes: []uint16{040755, 0120000, 0100644},
	}}
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			r, err := FromTar(tc.input, RPMMetaData{})
			if err != nil {
				t.Errorf("FromTar returned err: %v", err)
			}
			if err := r.Write(io.Discard); err != nil {
				t.Errorf("r.Write() returned err: %v", err)
			}
			if r == nil {
				t.Fatalf("FromTar returned nil pointer")
			}
			if d := cmp.Diff(tc.wantBasenames, r.basenames); d != "" {
				t.Errorf("FromTar basenames differs (want->got):\n%v", d)
			}
			if d := cmp.Diff(tc.wantFileModes, r.filemodes); d != "" {
				t.Errorf("FromTar filemodes differs (want->got):\n%v", d)
			}
		})
	}
}
