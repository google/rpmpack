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
	"strings"

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
	Name,
	Description,
	Version,
	Release,
	Arch,
	OS,
	Vendor,
	URL,
	Packager,
	Licence string
	Provides,
	Obsoletes,
	Suggests,
	Recommends,
	Requires,
	Conflicts []string
}

type pkg struct {
	Name    string
	Version string
	Sense   rpmSense
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
	prein       string
	postin      string
	preun       string
	postun      string
	parsedProvides,
	parsedObsoletes,
	parseSuggests,
	parsedRecommends,
	parsedRequires,
	parsedConflicts []*pkg
}

// NewRPM creates and returns a new RPM struct.
func NewRPM(m RPMMetaData) (rpm *RPM, err error) {
	if m.OS == "" {
		m.OS = "linux"
	}

	var z *gzip.Writer
	p := &bytes.Buffer{}
	if z, err = gzip.NewWriterLevel(p, 9); err != nil {
		return nil, errors.Wrap(err, "failed to create gzip writer")
	}
	rpm = &RPM{
		RPMMetaData: m,
		di:          newDirIndex(),
		payload:     p,
		gzPayload:   z,
		cpio:        cpio.NewWriter(z),
		files:       make(map[string]RPMFile),
	}
	err = rpm.ParsePackages()
	return rpm, err
}

// ParsePackages parses the various package related strings into their name, version and sense fields
func (r *RPM) ParsePackages() (err error) {
	if r.parsedProvides, err = parsePkg(r.Provides); err != nil {
		return err
	}

	var selfFound bool
	for idx := range r.parsedProvides {
		if r.parsedProvides[idx].Name == r.Name {
			selfFound = true
			r.parsedProvides[idx].Version = r.Version
			r.parsedProvides[idx].Sense = SenseEqual
		}
	}
	if !selfFound {
		r.parsedProvides = append(r.parsedProvides, &pkg{
			Name:    r.Name,
			Version: r.Version,
			Sense:   SenseEqual,
		})
	}

	if r.parsedObsoletes, err = parsePkg(r.Obsoletes); err != nil {
		return err
	}

	if r.parseSuggests, err = parsePkg(r.Suggests); err != nil {
		return err
	}

	if r.parsedRecommends, err = parsePkg(r.Recommends); err != nil {
		return err
	}

	if r.parsedRequires, err = parsePkg(r.Requires); err != nil {
		return err
	}

	if r.parsedConflicts, err = parsePkg(r.Conflicts); err != nil {
		return err
	}

	return nil
}

// FullVersion properly combines version and release fields to a version string
func (r *RPM) FullVersion() string {
	if r.Release != "" {
		return fmt.Sprintf("%s-%s", r.Version, r.Release)
	}

	return r.Version
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

	if _, err := w.Write(lead(r.Name, r.FullVersion())); err != nil {
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

	if _, err := w.Write(sb); err != nil {
		return errors.Wrap(err, "failed to write signature bytes")
	}
	//Signatures are padded to 8-byte boundaries
	if _, err := w.Write(make([]byte, (8-len(sb)%8)%8)); err != nil {
		return errors.Wrap(err, "failed to write signature padding")
	}
	if _, err := w.Write(hb); err != nil {
		return errors.Wrap(err, "failed to write header body")
	}
	_, err = w.Write(r.payload.Bytes())
	return errors.Wrap(err, "failed to write payload")

}

// Only call this after the payload and header were written.
func (r *RPM) writeSignatures(sigHeader *index, regHeader []byte) {
	sigHeader.Add(sigSize, entry([]int32{int32(r.payload.Len() + len(regHeader))}))
	sigHeader.Add(sigSHA256, entry(fmt.Sprintf("%x", sha256.Sum256(regHeader))))
	sigHeader.Add(sigPayloadSize, entry([]int32{int32(r.payloadSize)}))
}

func (r *RPM) writeGenIndexes(h *index) {
	h.Add(tagHeaderI18NTable, entry("C"))
	h.Add(tagSize, entry([]int32{int32(r.payloadSize)}))
	h.Add(tagName, entry(r.Name))
	h.Add(tagDescription, entry(r.Description))
	h.Add(tagVersion, entry(r.Version))
	h.Add(tagRelease, entry(r.Release))
	h.Add(tagPayloadFormat, entry("cpio"))
	h.Add(tagPayloadCompressor, entry("gzip"))
	h.Add(tagPayloadFlags, entry("9"))
	h.Add(tagOS, entry(r.OS))
	h.Add(tagArch, entry(r.Arch))
	h.Add(tagVendor, entry(r.Vendor))
	h.Add(tagLicence, entry(r.Licence))
	h.Add(tagPackager, entry(r.Packager))
	h.Add(tagURL, entry(r.URL))

	if len(r.parsedRequires) > 0 {
		names := make([]string, len(r.parsedRequires))
		versions := make([]string, len(r.parsedRequires))
		flags := make([]uint32, len(r.parsedRequires))

		for idx := range r.parsedRequires {
			names[idx] = r.parsedRequires[idx].Name
			versions[idx] = r.parsedRequires[idx].Version
			flags[idx] = uint32(r.parsedRequires[idx].Sense)
		}
		h.Add(tagRequire, entry(names))
		h.Add(tagRequireVersion, entry(versions))
		h.Add(tagRequireFlags, entry(flags))
	}

	if len(r.parsedObsoletes) > 0 {
		names := make([]string, len(r.parsedObsoletes))
		versions := make([]string, len(r.parsedObsoletes))
		flags := make([]uint32, len(r.parsedObsoletes))

		for idx := range r.parsedObsoletes {
			names[idx] = r.parsedObsoletes[idx].Name
			versions[idx] = r.parsedObsoletes[idx].Version
			flags[idx] = uint32(r.parsedObsoletes[idx].Sense)
		}
		h.Add(tagObsolete, entry(names))
		h.Add(tagObsoleteVersion, entry(versions))
		h.Add(tagObsoleteFlags, entry(flags))
	}

	if len(r.parsedConflicts) > 0 {
		names := make([]string, len(r.parsedConflicts))
		versions := make([]string, len(r.parsedConflicts))
		flags := make([]uint32, len(r.parsedConflicts))

		for idx := range r.parsedConflicts {
			names[idx] = r.parsedConflicts[idx].Name
			versions[idx] = r.parsedConflicts[idx].Version
			flags[idx] = uint32(r.parsedConflicts[idx].Sense)
		}
		h.Add(tagConflicts, entry(names))
		h.Add(tagConflictVersion, entry(versions))
		h.Add(tagConflictFlags, entry(flags))
	}

	if len(r.parsedRecommends) > 0 {
		names := make([]string, len(r.parsedRecommends))
		versions := make([]string, len(r.parsedRecommends))
		flags := make([]uint32, len(r.parsedRecommends))

		for idx := range r.parsedRecommends {
			names[idx] = r.parsedRecommends[idx].Name
			versions[idx] = r.parsedRecommends[idx].Version
			flags[idx] = uint32(r.parsedRecommends[idx].Sense)
		}
		h.Add(tagRecommends, entry(names))
		h.Add(tagRecommendVersion, entry(versions))
		h.Add(tagRecommendFlags, entry(flags))
	}

	if len(r.parseSuggests) > 0 {
		names := make([]string, len(r.parseSuggests))
		versions := make([]string, len(r.parseSuggests))
		flags := make([]uint32, len(r.parseSuggests))

		for idx := range r.parseSuggests {
			names[idx] = r.parseSuggests[idx].Name
			versions[idx] = r.parseSuggests[idx].Version
			flags[idx] = uint32(r.parseSuggests[idx].Sense)
		}
		h.Add(tagSuggests, entry(names))
		h.Add(tagSuggestVersion, entry(versions))
		h.Add(tagSuggestFlags, entry(flags))
	}

	if len(r.parsedProvides) > 0 {
		names := make([]string, len(r.parsedProvides))
		versions := make([]string, len(r.parsedProvides))
		flags := make([]uint32, len(r.parsedProvides))

		for idx := range r.parsedProvides {
			names[idx] = r.parsedProvides[idx].Name
			versions[idx] = r.parsedProvides[idx].Version
			flags[idx] = uint32(r.parsedProvides[idx].Sense)
		}
		h.Add(tagProvides, entry(names))
		h.Add(tagProvideVersion, entry(versions))
		h.Add(tagProvideFlags, entry(flags))
	}

	// rpm utilities look for the sourcerpm tag to deduce if this is not a source rpm (if it has a sourcerpm,
	// it is NOT a source rpm).
	h.Add(tagSourceRPM, entry(fmt.Sprintf("%s-%s.src.rpm", r.Name, r.FullVersion())))
	if r.prein != "" {
		h.Add(tagPrein, entry(r.prein))
		h.Add(tagPreinProg, entry("/bin/sh"))
	}
	if r.postin != "" {
		h.Add(tagPostin, entry(r.postin))
		h.Add(tagPostinProg, entry("/bin/sh"))
	}
	if r.preun != "" {
		h.Add(tagPreun, entry(r.preun))
		h.Add(tagPreunProg, entry("/bin/sh"))
	}
	if r.postun != "" {
		h.Add(tagPostun, entry(r.postun))
		h.Add(tagPostunProg, entry("/bin/sh"))
	}
}

// WriteFileIndexes writes file related index headers to the header
func (r *RPM) writeFileIndexes(h *index) {
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
}

// AddPrein adds a prein sciptlet
func (r *RPM) AddPrein(s string) {
	r.prein = s
}

// AddPostin adds a postin sciptlet
func (r *RPM) AddPostin(s string) {
	r.postin = s
}

// AddPreun adds a preun sciptlet
func (r *RPM) AddPreun(s string) {
	r.preun = s
}

// AddPostun adds a postun sciptlet
func (r *RPM) AddPostun(s string) {
	r.postun = s
}

// AddFile adds an RPMFile to an existing rpm.
func (r *RPM) AddFile(f RPMFile) {
	if f.Name == "/" { // rpm does not allow the root dir to be included.
		return
	}
	r.files[f.Name] = f
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
	return r.writePayload(f, links)
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

func parsePkg(pkgList []string) ([]*pkg, error) {
	parsed := make([]*pkg, len(pkgList))
	for idx, pkgString := range pkgList {
		var (
			pkgInfo  []string
			pkgSense = SenseAny
		)

		switch {
		case strings.Contains(pkgString, cmpgte):
			pkgInfo = strings.Split(pkgString, cmpgte)
			if len(pkgInfo) != 2 {
				return nil, fmt.Errorf("%v did not parse the package name and version properly", pkgString)
			}
			pkgSense = SenseEqual | SenseGreater
		case strings.Contains(pkgString, cmplte):
			pkgInfo = strings.Split(pkgString, cmplte)
			if len(pkgInfo) != 2 {
				return nil, fmt.Errorf("%v did not parse the package name and version properly", pkgString)
			}
			pkgSense = SenseEqual | SenseLess
		case strings.Contains(pkgString, cmpeq):
			pkgInfo = strings.Split(pkgString, cmpeq)
			if len(pkgInfo) != 2 {
				return nil, fmt.Errorf("%v did not parse the package name and version properly", pkgString)
			}
			pkgSense = SenseEqual
		case strings.Contains(pkgString, cmpgt):
			pkgInfo = strings.Split(pkgString, cmpgt)
			if len(pkgInfo) != 2 {
				return nil, fmt.Errorf("%v did not parse the package name and version properly", pkgString)
			}
			pkgSense = SenseGreater
		case strings.Contains(pkgString, cmplt):
			pkgInfo = strings.Split(pkgString, cmplt)
			if len(pkgInfo) != 2 {
				return nil, fmt.Errorf("%v did not parse the package name and version properly", pkgString)
			}
			pkgSense = SenseLess
		default:
			pkgInfo = []string{pkgString, ""}
		}
		parsed[idx] = &pkg{
			Name:    strings.Trim(pkgInfo[0], " "),
			Version: strings.Trim(pkgInfo[1], " "),
			Sense:   pkgSense,
		}
	}

	return parsed, nil
}
