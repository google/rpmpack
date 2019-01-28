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
	"bytes"
	"compress/gzip"
	"crypto/sha1"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"path"

	cpio "github.com/cavaliercoder/go-cpio"
)

var (
	ErrWriteAfterClose = errors.New("rpm write after close")
)

type RPMMetaData struct {
	Name    string
	Version string
	Release string
}

type RPMFile struct {
	Name  string
	Body  []byte
	Mode  uint
	Owner string
	Group string
	MTime uint32
}

// Opts is used to specify global options for all files in an rpm,
// to be used in functions that accept a list of file names.
type Opts struct {
	Owner string
	Group string
	Mode  uint
	Mtime uint
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
	closed      bool
	gz_payload  *gzip.Writer
}

// Create and return a new RPM struct.
func NewRPM(m RPMMetaData) (*RPM, error) {
	p := &bytes.Buffer{}
	z, err := gzip.NewWriterLevel(p, 9)
	if err != nil {
		return nil, err
	}
	return &RPM{
		RPMMetaData: m,
		di:          newDirIndex(),
		payload:     p,
		gz_payload:  z,
		cpio:        cpio.NewWriter(z),
	}, nil
}

// Write closes the rpm and writes the whole rpm to an io.Writer
func (r *RPM) Write(w io.Writer) error {
	if r.closed {
		return ErrWriteAfterClose
	}
	if err := r.cpio.Close(); err != nil {
		return err
	}
	if err := r.gz_payload.Close(); err != nil {
		return err
	}

	if _, err := w.Write(lead(r.Name, r.Version, r.Release)); err != nil {
		return err
	}
	// Write the regular header.
	h := newIndex(immutable)
	r.writeGenIndexes(h)
	r.writeFileIndexes(h)
	hb, err := h.Bytes()
	if err != nil {
		return err
	}
	// Write the signatures
	s := newIndex(signatures)
	r.writeSignatures(s, hb)
	sb, err := s.Bytes()
	if err != nil {
		return err
	}

	w.Write(sb)
	//Signatures are padded to 8-byte boundaries
	w.Write(make([]byte, (8-len(sb)%8)%8))
	w.Write(hb)
	if _, err := w.Write(r.payload.Bytes()); err != nil {
		return err
	}
	return nil

}

// Only call this after the payload and header were written.
func (r *RPM) writeSignatures(sigHeader *index, regHeader []byte) error {
	sigHeader.Add(sigSize, entry([]int32{int32(r.payload.Len() + len(regHeader))}))
	sigHeader.Add(sigSHA1, entry(fmt.Sprintf("%x", sha1.Sum(regHeader))))
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
	h.Add(tagProvides, entry(r.Name))
	h.Add(tagProvideVersion, entry(r.Version+"-"+r.Release))
	h.Add(tagProvideFlags, entry([]uint32{uint32(1 << 3)})) // means "="
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

	// is inodes just a range from 1..len(dirindexes)? maybe different with symlinks or dirs..
	inodes := make([]int32, len(r.dirindexes))
	for ii := range inodes {
		inodes[ii] = int32(ii + 1)
	}
	h.Add(tagFileINodes, entry(inodes))

	// We only use the sha256 digest algo, tag=8
	digestAlgo := make([]int32, len(r.dirindexes))
	for ii := range digestAlgo {
		digestAlgo[ii] = int32(8)
	}
	h.Add(tagFileDigestAlgo, entry(digestAlgo))
	//With regular files, it seems like we can always enable all of the veriy flags
	verifyFlags := make([]int32, len(r.dirindexes))
	for ii := range verifyFlags {
		verifyFlags[ii] = int32(-1)
	}
	h.Add(tagFileVerifyFlags, entry(verifyFlags))

	return nil
}

// AddFile adds an RPMFile to an existing rpm.
func (r *RPM) AddFile(f RPMFile) error {
	dir, file := path.Split(f.Name)
	r.dirindexes = append(r.dirindexes, r.di.Get(dir))
	r.basenames = append(r.basenames, file)
	r.filesizes = append(r.filesizes, uint32(len(f.Body)))
	r.filemodes = append(r.filemodes, uint16(f.Mode))
	r.fileowners = append(r.fileowners, f.Group)
	r.filegroups = append(r.filegroups, f.Owner)
	r.filemtimes = append(r.filemtimes, f.MTime)
	r.filedigests = append(r.filedigests, fmt.Sprintf("%x", sha256.Sum256(f.Body)))
	r.writePayload(f)
	return nil
}

func (r *RPM) writePayload(f RPMFile) error {
	chash := cpio.NewHash()
	chash.Write(f.Body)
	hdr := &cpio.Header{
		Name:     f.Name,
		Mode:     cpio.FileMode(f.Mode),
		Size:     int64(len(f.Body)),
		Checksum: cpio.Checksum(chash.Sum32()),
	}
	if err := r.cpio.WriteHeader(hdr); err != nil {
		return err
	}
	if _, err := r.cpio.Write(f.Body); err != nil {
		return err
	}
	r.payloadSize += uint(len(f.Body))
	return nil
}
