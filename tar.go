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
	"fmt"
	"io"
	"path"
)

// FromTar reads a tar file and creates an rpm stuct.
func FromTar(inp io.Reader, md RPMMetaData) (*RPM, error) {
	r, err := NewRPM(md)
	if err != nil {
		return nil, fmt.Errorf("failed to create RPM structure: %w", err)
	}
	t := tar.NewReader(inp)
	for {
		h, err := t.Next()
		if err == io.EOF {
			return r, nil
		} else if err != nil {
			return nil, fmt.Errorf("failed to read tar file: %w", err)
		}
		var body []byte
		switch h.Typeflag {
		case tar.TypeDir:
			h.Mode |= 040000
		case tar.TypeSymlink:
			body = []byte(h.Linkname)
			h.Mode |= 0120000
		case tar.TypeReg:
			b, err := io.ReadAll(t)
			if err != nil {
				return nil, fmt.Errorf("failed to read file (%q): %w", h.Name, err)
			}
			body = b
		default:
			return nil, fmt.Errorf("unknown tar type: %d, (%q)", h.Typeflag, h.Name)
		}
		mtime := uint32(h.ModTime.Unix())

		// Sometimes the tar has no uname and gname. RPM expects these to always exist.
		owner := h.Uname
		if owner == "" {
			owner = "root"
		}
		group := h.Gname
		if group == "" {
			group = "root"
		}

		r.AddFile(
			RPMFile{
				Name:  path.Join("/", h.Name),
				Body:  body,
				Mode:  uint(h.Mode),
				Owner: owner,
				Group: group,
				MTime: mtime,
			})
	}
}
