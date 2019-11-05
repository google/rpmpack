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
	"github.com/ulikunitz/xz"
	"github.com/ulikunitz/xz/lzma"
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
	Group,
	Licence,
	Compressor string
	Provides,
	Obsoletes,
	Suggests,
	Recommends,
	Requires,
	Conflicts Relations
}

// RPM holds the state of a particular rpm file. Please use NewRPM to instantiate it.
type RPM struct {
	RPMMetaData
	di                *dirIndex
	payload           *bytes.Buffer
	payloadSize       uint
	cpio              *cpio.Writer
	basenames         []string
	dirindexes        []uint32
	filesizes         []uint32
	filemodes         []uint16
	fileowners        []string
	filegroups        []string
	filemtimes        []uint32
	filedigests       []string
	filelinktos       []string
	fileflags         []uint32
	closed            bool
	compressedPayload io.WriteCloser
	files             map[string]RPMFile
	prein             string
	postin            string
	preun             string
	postun            string
	sigIndex,
	normalIndex *index
}

// NewRPM creates and returns a new RPM struct.
func NewRPM(m RPMMetaData) (*RPM, error) {
	var err error

	if m.OS == "" {
		m.OS = "linux"
	}

	if m.Arch == "" {
		m.Arch = "noarch"
	}

	p := &bytes.Buffer{}
	var z io.WriteCloser
	switch m.Compressor {
	case "":
		m.Compressor = "gzip"
		fallthrough
	case "gzip":
		z, err = gzip.NewWriterLevel(p, 9)
	case "lzma":
		z, err = lzma.NewWriter(p)
	case "xz":
		z, err = xz.NewWriter(p)
	default:
		err = fmt.Errorf("unknown compressor type %s", m.Compressor)
	}
	if err != nil {
		return nil, errors.Wrap(err, "failed to create compression writer")
	}

	rpm := &RPM{
		RPMMetaData:       m,
		di:                newDirIndex(),
		payload:           p,
		compressedPayload: z,
		cpio:              cpio.NewWriter(z),
		files:             make(map[string]RPMFile),
		normalIndex:       newIndex(immutable),
		sigIndex:          newIndex(signatures),
	}

	// A package must provide itself...
	rpm.Provides.addIfMissing(&Relation{
		Name:    rpm.Name,
		Version: rpm.FullVersion(),
		Sense:   SenseEqual,
	})

	return rpm, nil
}

// FullVersion properly combines version and release fields to a version string
func (r *RPM) FullVersion() string {
	if r.Release != "" {
		return fmt.Sprintf("%s-%s", r.Version, r.Release)
	}

	return r.Version
}

// AddTag a tag to the normal index of the rpm
func (r *RPM) AddTag(rpmTag int, value *IndexEntry) {
	r.normalIndex.Add(rpmTag, value)
}

// AddSignatureTag a tag to the signature index of the rpm
func (r *RPM) AddSignatureTag(rpmTag int, value *IndexEntry) {
	r.sigIndex.Add(rpmTag, value)
}

func (r *RPM) DefaultTags() error {
	var err error

	// Write the regular header.
	if err = r.WriteGeneralIndexes(); err != nil {
		return err
	}
	if err = r.WriteRelationIndexes(); err != nil {
		return err
	}
	return nil
}

// Only call this after the payload and header were written.
func (r *RPM) WriteSignatures() error {
	var (
		err error
		sigSizeEntry,
		sigSHA256Entry,
		sigPayloadSizeEntry *IndexEntry
	)

	regHeader, err := r.normalIndex.Bytes()
	if err != nil {
		return errors.Wrap(err, "failed to retrieve header")
	}

	if sigSizeEntry, err = NewIndexEntry([]int32{int32(r.payload.Len() + len(regHeader))}); err != nil {
		return err
	}
	if sigSHA256Entry, err = NewIndexEntry(fmt.Sprintf("%x", sha256.Sum256(regHeader))); err != nil {
		return err
	}
	if sigPayloadSizeEntry, err = NewIndexEntry([]int32{int32(r.payloadSize)}); err != nil {
		return err
	}
	r.AddSignatureTag(sigSize, sigSizeEntry)
	r.AddSignatureTag(sigSHA256, sigSHA256Entry)
	r.AddSignatureTag(sigPayloadSize, sigPayloadSizeEntry)

	return nil
}

// Write closes the rpm and writes the whole rpm to an io.Writer
func (r *RPM) Write(w io.Writer) error {
	var err error
	if r.closed {
		return ErrWriteAfterClose
	}
	if err = r.DefaultTags(); err != nil {
		return err
	}
	if err = r.WriteFileIndexes(); err != nil {
		return err
	}

	if err = r.WritePayloadIndexes(); err != nil {
		return err
	}
	if err = r.WriteSignatures(); err != nil {
		return err
	}

	return r.WriteCustom(w)
}

// WriteCustom closes the rpm and writes the whole rpm to an io.Writer
// does NOT write the default indexes or signature indexes, it expects you to do that
func (r *RPM) WriteCustom(w io.Writer) error {
	if r.closed {
		return ErrWriteAfterClose
	}

	if _, err := w.Write(lead(r.Name, r.FullVersion())); err != nil {
		return errors.Wrap(err, "failed to write lead")
	}

	hb, err := r.normalIndex.Bytes()
	if err != nil {
		return errors.Wrap(err, "failed to retrieve header")
	}

	sb, err := r.sigIndex.Bytes()
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

// WriteRelationIndexes write the relation sense indexes
func (r *RPM) WriteRelationIndexes() error {
	// add all relation categories
	if err := r.Provides.AddToIndex(r.normalIndex, tagProvides, tagProvideVersion, tagProvideFlags); err != nil {
		return errors.Wrap(err, "failed to add provides")
	}
	if err := r.Obsoletes.AddToIndex(r.normalIndex, tagObsoletes, tagObsoleteVersion, tagObsoleteFlags); err != nil {
		return errors.Wrap(err, "failed to add obsoletes")
	}
	if err := r.Suggests.AddToIndex(r.normalIndex, tagSuggests, tagSuggestVersion, tagSuggestFlags); err != nil {
		return errors.Wrap(err, "failed to add suggests")
	}
	if err := r.Recommends.AddToIndex(r.normalIndex, tagRecommends, tagRecommendVersion, tagRecommendFlags); err != nil {
		return errors.Wrap(err, "failed to add recommends")
	}
	if err := r.Requires.AddToIndex(r.normalIndex, tagRequires, tagRequireVersion, tagRequireFlags); err != nil {
		return errors.Wrap(err, "failed to add requires")
	}
	if err := r.Conflicts.AddToIndex(r.normalIndex, tagConflicts, tagConflictVersion, tagConflictFlags); err != nil {
		return errors.Wrap(err, "failed to add conflicts")
	}

	return nil
}

func (r *RPM) WriteGeneralIndexes() error {
	var (
		err error
		headerI18NTableEntry,
		nameEntry,
		versionEntry,
		releaseEntry,
		archEntry,
		osEntry,
		vendorEntry,
		licenceEntry,
		packagerEntry,
		groupEntry,
		urlEntry,
		sourceRPMEntry,
		progEntry,
		preinEntry,
		postinEntry,
		preunEntry,
		postunEntry *IndexEntry
	)
	if headerI18NTableEntry, err = NewIndexEntry("C"); err != nil {
		return err
	}
	if nameEntry, err = NewIndexEntry(r.Name); err != nil {
		return err
	}
	if versionEntry, err = NewIndexEntry(r.Version); err != nil {
		return err
	}
	if releaseEntry, err = NewIndexEntry(r.Release); err != nil {
		return err
	}
	if archEntry, err = NewIndexEntry(r.Arch); err != nil {
		return err
	}
	if osEntry, err = NewIndexEntry(r.OS); err != nil {
		return err
	}
	if vendorEntry, err = NewIndexEntry(r.Vendor); err != nil {
		return err
	}
	if licenceEntry, err = NewIndexEntry(r.Licence); err != nil {
		return err
	}
	if packagerEntry, err = NewIndexEntry(r.Packager); err != nil {
		return err
	}
	if groupEntry, err = NewIndexEntry(r.Group); err != nil {
		return err
	}
	if urlEntry, err = NewIndexEntry(r.URL); err != nil {
		return err
	}
	if sourceRPMEntry, err = NewIndexEntry(fmt.Sprintf("%s-%s.src.rpm", r.Name, r.FullVersion())); err != nil {
		return err
	}
	if progEntry, err = NewIndexEntry("/bin/sh"); err != nil {
		return err
	}
	if preinEntry, err = NewIndexEntry(r.prein); err != nil {
		return err
	}
	if postinEntry, err = NewIndexEntry(r.postin); err != nil {
		return err
	}
	if preunEntry, err = NewIndexEntry(r.preun); err != nil {
		return err
	}
	if postunEntry, err = NewIndexEntry(r.postun); err != nil {
		return err
	}

	r.AddTag(tagHeaderI18NTable, headerI18NTableEntry)
	r.AddTag(tagName, nameEntry)
	r.AddTag(tagVersion, versionEntry)
	r.AddTag(tagRelease, releaseEntry)
	r.AddTag(tagArch, archEntry)
	r.AddTag(tagOS, osEntry)
	r.AddTag(tagVendor, vendorEntry)
	r.AddTag(tagLicence, licenceEntry)
	r.AddTag(tagPackager, packagerEntry)
	r.AddTag(tagGroup, groupEntry)
	r.AddTag(tagURL, urlEntry)

	// rpm utilities look for the sourcerpm tag to deduce if this is not a source rpm (if it has a sourcerpm,
	// it is NOT a source rpm).
	r.AddTag(tagSourceRPM, sourceRPMEntry)
	if r.prein != "" {
		r.AddTag(tagPrein, preinEntry)
		r.AddTag(tagPreinProg, progEntry)
	}
	if r.postin != "" {
		r.AddTag(tagPostin, postinEntry)
		r.AddTag(tagPostinProg, progEntry)
	}
	if r.preun != "" {
		r.AddTag(tagPreun, preunEntry)
		r.AddTag(tagPreunProg, progEntry)
	}
	if r.postun != "" {
		r.AddTag(tagPostun, postunEntry)
		r.AddTag(tagPostunProg, progEntry)
	}

	return nil
}

// WritePayloadIndexes writes payload related indexes
func (r *RPM) WritePayloadIndexes() error {
	var (
		err error

		payloadFormatEntry,
		sizeEntry,
		payloadCompressorEntry,
		payloadDigestEntry,
		payloadDigestAlgoEntry,
		payloadFlagsEntry *IndexEntry
	)

	if err := r.cpio.Close(); err != nil {
		return errors.Wrap(err, "failed to close cpio payload")
	}
	if err := r.compressedPayload.Close(); err != nil {
		return errors.Wrap(err, "failed to close gzip payload")
	}

	if sizeEntry, err = NewIndexEntry([]int32{int32(r.payloadSize)}); err != nil {
		return err
	}
	if payloadFormatEntry, err = NewIndexEntry("cpio"); err != nil {
		return err
	}
	if payloadCompressorEntry, err = NewIndexEntry(r.Compressor); err != nil {
		return err
	}
	if payloadFlagsEntry, err = NewIndexEntry("9"); err != nil {
		return err
	}
	if payloadDigestEntry, err = NewIndexEntry([]string{fmt.Sprintf("%x", sha256.Sum256(r.payload.Bytes()))}); err != nil {
		return err
	}
	if payloadDigestAlgoEntry, err = NewIndexEntry([]int32{hashAlgoSHA256}); err != nil {
		return err
	}
	r.AddTag(tagSize, sizeEntry)
	r.AddTag(tagPayloadFormat, payloadFormatEntry)
	r.AddTag(tagPayloadCompressor, payloadCompressorEntry)
	r.AddTag(tagPayloadFlags, payloadFlagsEntry)
	r.AddTag(tagPayloadDigest, payloadDigestEntry)
	r.AddTag(tagPayloadDigestAlgo, payloadDigestAlgoEntry)

	return nil
}

// WriteFileIndexes writes file related index headers to the header
func (r *RPM) WriteFileIndexes() error {
	var (
		err error
		basenamesEntry,
		dirindexesEntry,
		dirnamesEntry,
		fileSizesEntry,
		fileModesEntry,
		fileUserNameEntry,
		fileGroupNameEntry,
		fileMTimesEntry,
		fileDigestsEntry,
		fileLinkTosEntry,
		fileFlagsEntry,
		fileINodeEntry,
		fileDigestAlgoEntry,
		fileVerifyFlagsEntry,
		fileRDevsEntry,
		fileLangsEntry *IndexEntry
	)

	// Add all of the files, sorted alphabetically.
	var fnames []string
	for fn := range r.files {
		fnames = append(fnames, fn)
	}
	sort.Strings(fnames)
	for _, fn := range fnames {
		if err := r.writeFile(r.files[fn]); err != nil {
			return errors.Wrapf(err, "failed to write file %q", fn)
		}
	}

	if basenamesEntry, err = NewIndexEntry(r.basenames); err != nil {
		return err
	}
	if dirindexesEntry, err = NewIndexEntry(r.dirindexes); err != nil {
		return err
	}
	if dirnamesEntry, err = NewIndexEntry(r.di.AllDirs()); err != nil {
		return err
	}
	if fileSizesEntry, err = NewIndexEntry(r.filesizes); err != nil {
		return err
	}
	if fileModesEntry, err = NewIndexEntry(r.filemodes); err != nil {
		return err
	}
	if fileUserNameEntry, err = NewIndexEntry(r.fileowners); err != nil {
		return err
	}
	if fileGroupNameEntry, err = NewIndexEntry(r.filegroups); err != nil {
		return err
	}
	if fileMTimesEntry, err = NewIndexEntry(r.filemtimes); err != nil {
		return err
	}
	if fileDigestsEntry, err = NewIndexEntry(r.filedigests); err != nil {
		return err
	}
	if fileLinkTosEntry, err = NewIndexEntry(r.filelinktos); err != nil {
		return err
	}
	if fileFlagsEntry, err = NewIndexEntry(r.fileflags); err != nil {
		return err
	}

	r.AddTag(tagBasenames, basenamesEntry)
	r.AddTag(tagDirindexes, dirindexesEntry)
	r.AddTag(tagDirnames, dirnamesEntry)
	r.AddTag(tagFileSizes, fileSizesEntry)
	r.AddTag(tagFileModes, fileModesEntry)
	r.AddTag(tagFileUserName, fileUserNameEntry)
	r.AddTag(tagFileGroupName, fileGroupNameEntry)
	r.AddTag(tagFileMTimes, fileMTimesEntry)
	r.AddTag(tagFileDigests, fileDigestsEntry)
	r.AddTag(tagFileLinkTos, fileLinkTosEntry)
	r.AddTag(tagFileFlags, fileFlagsEntry)

	inodes := make([]int32, len(r.dirindexes))
	digestAlgo := make([]int32, len(r.dirindexes))
	verifyFlags := make([]int32, len(r.dirindexes))
	fileRDevs := make([]int16, len(r.dirindexes))
	fileLangs := make([]string, len(r.dirindexes))

	for ii := range inodes {
		// is inodes just a range from 1..len(dirindexes)? maybe different with hard links
		inodes[ii] = int32(ii + 1)
		digestAlgo[ii] = hashAlgoSHA256
		// With regular files, it seems like we can always enable all of the verify flags
		verifyFlags[ii] = int32(-1)
		fileRDevs[ii] = int16(1)
	}

	if fileINodeEntry, err = NewIndexEntry(inodes); err != nil {
		return err
	}
	if fileDigestAlgoEntry, err = NewIndexEntry(digestAlgo); err != nil {
		return err
	}
	if fileVerifyFlagsEntry, err = NewIndexEntry(verifyFlags); err != nil {
		return err
	}
	if fileRDevsEntry, err = NewIndexEntry(fileRDevs); err != nil {
		return err
	}
	if fileLangsEntry, err = NewIndexEntry(fileLangs); err != nil {
		return err
	}

	r.AddTag(tagFileINodes, fileINodeEntry)
	r.AddTag(tagFileDigestAlgo, fileDigestAlgoEntry)
	r.AddTag(tagFileVerifyFlags, fileVerifyFlagsEntry)
	r.AddTag(tagFileRDevs, fileRDevsEntry)
	r.AddTag(tagFileLangs, fileLangsEntry)

	return nil
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
	r.fileflags = append(r.fileflags, uint32(f.Type))

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
