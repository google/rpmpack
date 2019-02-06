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
	"sort"

	"github.com/pkg/errors"
)

// FromFiles reads files from the filesystem and given filenames,
// and creates an rpm. The paths are relative to the current working directory.
func FromFiles(files []string, md RPMMetaData, opts Opts) (*RPM, error) {

	r, err := NewRPM(md)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create RPM structure")
	}
	sort.Strings(files)
	for _, f := range files {
		fs, err := os.Lstat(f)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to stat file (%q)", f)
		}
		var mode uint
		var body []byte
		switch {
		case fs.Mode().IsDir():
			mode = 040000
			if opts.DirMode != 0 {
				mode |= opts.DirMode
			} else {
				mode |= uint(fs.Mode().Perm())
			}
		case fs.Mode()&os.ModeSymlink != 0:
			mode = 0120777
			s, err := os.Readlink(f)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to read link (%q)", f)
			}
			body = []byte(s)
		default:
			if opts.FileMode != 0 {
				mode |= opts.FileMode
			} else {
				mode |= uint(fs.Mode().Perm())
			}
			b, err := ioutil.ReadFile(f)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to read file (%q)", f)
			}
			body = b
		}

		if err := r.AddFile(
			RPMFile{
				Name:  path.Join("/", f),
				Body:  body,
				Mode:  mode,
				Owner: opts.Owner,
				Group: opts.Group,
			}); err != nil {
			return nil, errors.Wrapf(err, "failed to add file (%q)", f)
		}
	}
	return r, nil
}
