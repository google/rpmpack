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
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// create some files in a tempdir, and switch to there.
func createFileStructure(t *testing.T) func() {
	t.Helper()
	d, err := ioutil.TempDir("", "rpmpack")
	if err != nil {
		t.Errorf("failed to create tempdir: %v", err)
	}
	if err := os.Chdir(d); err != nil {
		t.Errorf("failed to switch to tempdir: %v", err)
	}
	if err := ioutil.WriteFile("testfile1.txt", []byte("content1"), os.FileMode(0644)); err != nil {
		t.Errorf("failed to write testfile1.txt: %v", err)
	}
	if err := os.Symlink("testfile1.txt", "symlink.txt"); err != nil {
		t.Errorf("failed to create symlink.txt: %v", err)
	}
	if err := os.Mkdir("dir1", os.FileMode(0755)); err != nil {
		t.Errorf("failed to create dir1: %v", err)
	}
	if err := ioutil.WriteFile(path.Join("dir1", "testfile2.txt"), []byte("content2"), os.FileMode(0755)); err != nil {
		t.Errorf("failed to create testfile2.txt: %v", err)
	}
	return func() {
		os.RemoveAll(d)
	}

}

func TestFromFiles(t *testing.T) {
	cleanUp := createFileStructure(t)
	defer cleanUp()

	testCases := []struct {
		name          string
		files         []string
		opts          Opts
		wantBasenames []string
		wantFileModes []uint16
	}{{
		name:          "just a file",
		files:         []string{"testfile1.txt"},
		wantBasenames: []string{"testfile1.txt"},
		wantFileModes: []uint16{0100644},
	}, {
		name:          "just a dir",
		files:         []string{"dir1"},
		wantBasenames: []string{"dir1"},
		wantFileModes: []uint16{040755},
	}, {
		name:          "symlink",
		files:         []string{"symlink.txt"},
		wantBasenames: []string{"symlink.txt"},
		wantFileModes: []uint16{0120777},
	}}
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			r, err := FromFiles(tc.files, RPMMetaData{}, tc.opts)
			if err != nil {
				t.Errorf("FromFiles returned err: %v", err)
			}
			if r == nil {
				t.Fatalf("FromFiles returned nil pointer")
			}
			if d := cmp.Diff(tc.wantBasenames, r.basenames); d != "" {
				t.Errorf("FromFiles basenames differs (want->got):\n%v", d)
			}
			if d := cmp.Diff(tc.wantFileModes, r.filemodes); d != "" {
				t.Errorf("FromFiles filemodes differs (want->got):\n%v", d)
			}
		})
	}
}
