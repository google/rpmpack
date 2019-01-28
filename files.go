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
	"io"
	"io/ioutil"
	"os"
	"path"
	"sort"

	"github.com/pkg/errors"
)

func FromFiles(w io.Writer, files []string, md RPMMetaData, opts Opts) error {

	r, err := NewRPM(md)
	if err != nil {
		return err
	}
	sort.Strings(files)
	for _, f := range files {
		fmode := opts.Mode
		// Deduce mode from file
		if fmode == 0 {
			fs, err := os.Stat(f)
			if err != nil {
				return err
			}
			fmode = uint(fs.Mode().Perm())
		}

		b, err := ioutil.ReadFile(f)
		if err != nil {
			return err
		}
		if err := r.AddFile(
			RPMFile{
				Name:  path.Join("/", f),
				Body:  b,
				Mode:  fmode,
				Owner: opts.Owner,
				Group: opts.Group,
			}); err != nil {
			return errors.Wrapf(err, "failed to add file (%q)", f)
		}

	}
	return r.Write(w)
}
