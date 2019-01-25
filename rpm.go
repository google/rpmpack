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
	Mode  int
	Owner string
	Group string
	MTime int32
}

type rpm struct {
	RPMMetaData
	di          *DirIndex
	payload     *bytes.Buffer
	payloadSize int
	cpio        *cpio.Writer
	basenames   []string
	dirindexes  []int32
	filesizes   []int32
	filemodes   []int16
	fileowners  []string
	filegroups  []string
	filemtimes  []int32
	closed      bool
	gz_payload  *gzip.Writer
}

func NewRPM(m RPMMetaData) (*rpm, error) {
	p := &bytes.Buffer{}
	z, err := gzip.NewWriterLevel(p, 9)
	if err != nil {
		return nil, err
	}
	return &rpm{
		RPMMetaData: m,
		di:          NewDirIndex(),
		payload:     p,
		gz_payload:  z,
		cpio:        cpio.NewWriter(z),
	}, nil
}

// Write closes the rpm and writes the whole rpm to an io.Writer
func (r *rpm) Write(w io.Writer) error {
	if r.closed {
		return ErrWriteAfterClose
	}
	if err := r.cpio.Close(); err != nil {
		return err
	}
	if err := r.gz_payload.Close(); err != nil {
		return err
	}

	if _, err := w.Write(Lead(r.Name, r.Version, r.Release)); err != nil {
		return err
	}
	// Write the regular header.
	h := NewIndex(immutable)
	r.writeGenIndexes(h)
	r.writeFileIndexes(h)
	hb, err := h.Bytes()
	if err != nil {
		return err
	}
	// Write the signatures
	s := NewIndex(signatures)
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
func (r *rpm) writeSignatures(sigHeader *index, regHeader []byte) error {
	sigHeader.Add(sigSize, Int32Entry([]int32{int32(r.payload.Len() + len(regHeader))}))
	sigHeader.Add(sigSHA1, StringEntry(fmt.Sprintf("%x", sha1.Sum(regHeader))))
	sigHeader.Add(sigSHA256, StringEntry(fmt.Sprintf("%x", sha256.Sum256(regHeader))))
	sigHeader.Add(sigPayloadSize, Int32Entry([]int32{int32(r.payloadSize)}))
	return nil
}

func (r *rpm) writeGenIndexes(h *index) error {
	h.Add(tagHeaderI18NTable, StringEntry("C"))
	h.Add(tagSize, Int32Entry([]int32{int32(r.payloadSize)}))
	h.Add(tagName, StringEntry(r.Name))
	h.Add(tagVersion, StringEntry(r.Version))
	h.Add(tagRelease, StringEntry(r.Release))
	h.Add(tagPayloadFormat, StringEntry("cpio"))
	h.Add(tagPayloadCompressor, StringEntry("gzip"))
	h.Add(tagPayloadFlags, StringEntry("9"))
	h.Add(tagOS, StringEntry("linux"))
	h.Add(tagArch, StringEntry("noarch"))
	h.Add(tagProvides, StringEntry(r.Name))
	return nil
}

// WriteFileIndexes writes file related index headers to the header
func (r *rpm) writeFileIndexes(h *index) error {
	h.Add(tagBasenames, StringArrayEntry(r.basenames))
	h.Add(tagDirindexes, Int32Entry(r.dirindexes))
	h.Add(tagDirnames, StringArrayEntry(r.di.AllDirs()))
	h.Add(tagFileSizes, Int32Entry(r.filesizes))
	h.Add(tagFileModes, Int16Entry(r.filemodes))
	h.Add(tagFileUserName, StringArrayEntry(r.fileowners))
	h.Add(tagFileGroupName, StringArrayEntry(r.filegroups))
	h.Add(tagFileMTimes, Int32Entry(r.filemtimes))

	inodes := make([]int32, len(r.dirindexes))
	for ii := range inodes {
		inodes[ii] = int32(ii + 1)
	}
	h.Add(tagFileINodes, Int32Entry(inodes))
	return nil
}

func (r *rpm) AddFile(f RPMFile) error {
	dir, file := path.Split(f.Name)
	r.dirindexes = append(r.dirindexes, r.di.Get(dir))
	r.basenames = append(r.basenames, file)
	r.filesizes = append(r.filesizes, int32(len(f.Body)))
	r.filemodes = append(r.filemodes, int16(f.Mode))
	r.fileowners = append(r.fileowners, f.Group)
	r.filegroups = append(r.filegroups, f.Owner)
	r.filemtimes = append(r.filemtimes, f.MTime)
	r.writePayload(f)
	return nil
}

func (r *rpm) writePayload(f RPMFile) error {
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
	r.payloadSize += len(f.Body)
	return nil
}
