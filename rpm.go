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

// Package rpmpack packs files to rpm files.
// It is designed to be simple to use and deploy, not requiring any filesystem access
// to create rpm files.
package rpmpack

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"io"
	"path"
	"sort"

	cpio "github.com/cavaliercoder/go-cpio"
	"github.com/pkg/errors"
)

var (
	// ErrWriteAfterClose is returned when a user calls Write() on a closed rpm.
	ErrWriteAfterClose = errors.New("rpm write after close")
	// ErrWrongFileOrder is returned when files are not sorted by name.
	ErrWrongFileOrder = errors.New("wrong file addition order")
)

// RPMMetaData contains meta info about the whole package.
type RPMMetaData struct {
	Name    string
	Version string
	Release string
}

// RPMFile contains a particular file's entry and data.
type RPMFile struct {
	Name  string
	Body  []byte
	Mode  uint
	Owner string
	Group string
	MTime uint32
}

// RPM holds the state of a particular rpm file. Please use NewRPM to instantiate it.
type RPM struct {
	RPMMetaData
	di          *dirIndex
	payload     *bytes.Buffer
	payloadSize uint
	cpio        *cpio.Writer
	basenames   []string
	dirindexes  []uint32
	filesizes   []uint32
	filemodes   []uint16
	fileowners  []string
	filegroups  []string
	filemtimes  []uint32
	filedigests []string
	filelinktos []string
	closed      bool
	gzPayload   *gzip.Writer
	files       map[string]RPMFile
}

// NewRPM creates and returns a new RPM struct.
func NewRPM(m RPMMetaData) (*RPM, error) {
	p := &bytes.Buffer{}
	z, err := gzip.NewWriterLevel(p, 9)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create gzip writer")
	}
	return &RPM{
		RPMMetaData: m,
		di:          newDirIndex(),
		payload:     p,
		gzPayload:   z,
		cpio:        cpio.NewWriter(z),
		files:       make(map[string]RPMFile),
	}, nil
}

// Write closes the rpm and writes the whole rpm to an io.Writer
func (r *RPM) Write(w io.Writer) error {
	if r.closed {
		return ErrWriteAfterClose
	}
	// Add all of the files, sorted alphabetically.
	fnames := []string{}
	for fn := range r.files {
		fnames = append(fnames, fn)
	}
	sort.Strings(fnames)
	for _, fn := range fnames {
		if err := r.writeFile(r.files[fn]); err != nil {
			return errors.Wrapf(err, "failed to write file %q", fn)
		}
	}
	if err := r.cpio.Close(); err != nil {
		return errors.Wrap(err, "failed to close cpio payload")
	}
	if err := r.gzPayload.Close(); err != nil {
		return errors.Wrap(err, "failed to close gzip payload")
	}

	if _, err := w.Write(lead(r.Name, r.Version, r.Release)); err != nil {
		return errors.Wrap(err, "failed to write lead")
	}
	// Write the regular header.
	h := newIndex(immutable)
	r.writeGenIndexes(h)
	r.writeFileIndexes(h)
	hb, err := h.Bytes()
	if err != nil {
		return errors.Wrap(err, "failed to retrieve header")
	}
	// Write the signatures
	s := newIndex(signatures)
	r.writeSignatures(s, hb)
	sb, err := s.Bytes()
	if err != nil {
		return errors.Wrap(err, "failed to retrieve signatures header")
	}

	w.Write(sb)
	//Signatures are padded to 8-byte boundaries
	w.Write(make([]byte, (8-len(sb)%8)%8))
	w.Write(hb)
	_, err = w.Write(r.payload.Bytes())
	return errors.Wrap(err, "failed to write payload")

}

// Only call this after the payload and header were written.
func (r *RPM) writeSignatures(sigHeader *index, regHeader []byte) error {
	sigHeader.Add(sigSize, entry([]int32{int32(r.payload.Len() + len(regHeader))}))
	sigHeader.Add(sigSHA256, entry(fmt.Sprintf("%x", sha256.Sum256(regHeader))))
	sigHeader.Add(sigPayloadSize, entry([]int32{int32(r.payloadSize)}))
	return nil
}

func (r *RPM) writeGenIndexes(h *index) error {
	h.Add(tagHeaderI18NTable, entry("C"))
	h.Add(tagSize, entry([]int32{int32(r.payloadSize)}))
	h.Add(tagName, entry(r.Name))
	h.Add(tagVersion, entry(r.Version))
	h.Add(tagRelease, entry(r.Release))
	h.Add(tagPayloadFormat, entry("cpio"))
	h.Add(tagPayloadCompressor, entry("gzip"))
	h.Add(tagPayloadFlags, entry("9"))
	h.Add(tagOS, entry("linux"))
	h.Add(tagArch, entry("noarch"))
	// A package must provide itself...
	h.Add(tagProvides, entry([]string{r.Name}))
	h.Add(tagProvideVersion, entry([]string{r.Version + "-" + r.Release}))
	h.Add(tagProvideFlags, entry([]uint32{uint32(1 << 3)})) // means "="
	// rpm utilities look for the sourcerpm tag to deduce if this is not a source rpm (if it has a sourcerpm,
	// it is NOT a source rpm).
	h.Add(tagSourceRPM, entry(fmt.Sprintf("%s-%s-%s.src.rpm", r.Name, r.Version, r.Release)))
	return nil
}

// WriteFileIndexes writes file related index headers to the header
func (r *RPM) writeFileIndexes(h *index) error {
	h.Add(tagBasenames, entry(r.basenames))
	h.Add(tagDirindexes, entry(r.dirindexes))
	h.Add(tagDirnames, entry(r.di.AllDirs()))
	h.Add(tagFileSizes, entry(r.filesizes))
	h.Add(tagFileModes, entry(r.filemodes))
	h.Add(tagFileUserName, entry(r.fileowners))
	h.Add(tagFileGroupName, entry(r.filegroups))
	h.Add(tagFileMTimes, entry(r.filemtimes))
	h.Add(tagFileDigests, entry(r.filedigests))
	h.Add(tagFileLinkTos, entry(r.filelinktos))

	inodes := make([]int32, len(r.dirindexes))
	digestAlgo := make([]int32, len(r.dirindexes))
	verifyFlags := make([]int32, len(r.dirindexes))
	fileFlags := make([]int32, len(r.dirindexes))
	fileRDevs := make([]int16, len(r.dirindexes))
	fileLangs := make([]string, len(r.dirindexes))

	for ii := range inodes {
		// is inodes just a range from 1..len(dirindexes)? maybe different with hard links
		inodes[ii] = int32(ii + 1)
		// We only use the sha256 digest algo, tag=8
		digestAlgo[ii] = int32(8)
		// With regular files, it seems like we can always enable all of the verify flags
		verifyFlags[ii] = int32(-1)
		fileFlags[ii] = int32(0)
		fileRDevs[ii] = int16(1)
	}
	h.Add(tagFileINodes, entry(inodes))
	h.Add(tagFileDigestAlgo, entry(digestAlgo))
	h.Add(tagFileVerifyFlags, entry(verifyFlags))
	h.Add(tagFileFlags, entry(fileFlags))
	h.Add(tagFileRDevs, entry(fileRDevs))
	h.Add(tagFileLangs, entry(fileLangs))

	return nil
}

// AddFile adds an RPMFile to an existing rpm.
func (r *RPM) AddFile(f RPMFile) error {
	if f.Name == "/" { // rpm does not allow the root dir to be included.
		return nil
	}
	r.files[f.Name] = f
	return nil
}

// writeFile writes the file to the indexes and cpio.
func (r *RPM) writeFile(f RPMFile) error {
	dir, file := path.Split(f.Name)
	r.dirindexes = append(r.dirindexes, r.di.Get(dir))
	r.basenames = append(r.basenames, file)
	r.fileowners = append(r.fileowners, f.Group)
	r.filegroups = append(r.filegroups, f.Owner)
	r.filemtimes = append(r.filemtimes, f.MTime)
	links := 1
	switch {
	case f.Mode&040000 != 0: // directory
		r.filesizes = append(r.filesizes, 4096)
		r.filedigests = append(r.filedigests, "")
		r.filelinktos = append(r.filelinktos, "")
		links = 2
	case f.Mode&0120000 != 0: //  symlink
		r.filesizes = append(r.filesizes, uint32(len(f.Body)))
		r.filedigests = append(r.filedigests, "")
		r.filelinktos = append(r.filelinktos, string(f.Body))
	default: // regular file
		f.Mode = f.Mode | 0100000
		r.filesizes = append(r.filesizes, uint32(len(f.Body)))
		r.filedigests = append(r.filedigests, fmt.Sprintf("%x", sha256.Sum256(f.Body)))
		r.filelinktos = append(r.filelinktos, "")
	}
	r.filemodes = append(r.filemodes, uint16(f.Mode))
	r.writePayload(f, links)
	return nil
}

func (r *RPM) writePayload(f RPMFile, links int) error {
	hdr := &cpio.Header{
		Name:  f.Name,
		Mode:  cpio.FileMode(f.Mode),
		Size:  int64(len(f.Body)),
		Links: links,
	}
	if err := r.cpio.WriteHeader(hdr); err != nil {
		return errors.Wrap(err, "failed to write payload file header")
	}
	if _, err := r.cpio.Write(f.Body); err != nil {
		return errors.Wrap(err, "failed to write payload file content")
	}
	r.payloadSize += uint(len(f.Body))
	return nil
}
