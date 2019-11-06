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
	"encoding/binary"
	"fmt"
	"sort"
	"time"

	"github.com/pkg/errors"
)

const (
	signatures = 0x3e
	immutable  = 0x3f

	typeInt16       = 0x03
	typeInt32       = 0x04
	typeString      = 0x06
	typeBinary      = 0x07
	typeStringArray = 0x08
)

// Only integer types are aligned. This is not just an optimization - some versions
// of rpm fail when integers are not aligned. Other versions fail when non-integers are aligned.
var boundaries = map[int]int{
	typeInt16: 2,
	typeInt32: 4,
}

type IndexEntry struct {
	rpmtype, count int
	data           []byte
}

func (e IndexEntry) indexBytes(tag, contentOffset int) ([]byte, error) {
	b := &bytes.Buffer{}
	if err := binary.Write(b, binary.BigEndian, []int32{int32(tag), int32(e.rpmtype), int32(contentOffset), int32(e.count)}); err != nil {
		// binary.Write can fail if the underlying Write fails, or the types are invalid.
		// bytes.Buffer's write never error out, it can only panic with OOM.
		return nil, err
	}
	return b.Bytes(), nil
}

func intEntry(rpmtype, size int, value interface{}) (*IndexEntry, error) {
	b := &bytes.Buffer{}
	if err := binary.Write(b, binary.BigEndian, value); err != nil {
		// binary.Write can fail if the underlying Write fails, or the types are invalid.
		// bytes.Buffer's write never error out, it can only panic with OOM.
		return nil, err
	}
	return &IndexEntry{rpmtype, size, b.Bytes()}, nil
}

func NewIndexEntry(value interface{}) (*IndexEntry, error) {
	switch value := value.(type) {
	case []int16:
		return intEntry(typeInt16, len(value), value)
	case []uint16:
		return intEntry(typeInt16, len(value), value)
	case []int32:
		return intEntry(typeInt32, len(value), value)
	case []uint32:
		return intEntry(typeInt32, len(value), value)
	case string:
		return &IndexEntry{typeString, 1, append([]byte(value), byte(00))}, nil
	case time.Time:
		v := []int32{int32(value.Unix())}
		return intEntry(typeInt32, len(v), v)
	case []byte:
		return &IndexEntry{typeBinary, len(value), value}, nil
	case []string:
		b := [][]byte{}
		for _, v := range value {
			b = append(b, []byte(v))
		}
		bb := append(bytes.Join(b, []byte{00}), byte(00))
		return &IndexEntry{typeStringArray, len(value), bb}, nil
	}

	return nil, fmt.Errorf("unsupported index entry type %T", value)
}

type index struct {
	entries map[int]*IndexEntry
	h       int
}

func newIndex(h int) *index {
	return &index{entries: make(map[int]*IndexEntry), h: h}
}
func (i *index) Add(tag int, e *IndexEntry) {
	i.entries[tag] = e
}
func (i *index) sortedTags() []int {
	t := []int{}
	for k := range i.entries {
		t = append(t, k)
	}
	sort.Ints(t)
	return t
}

func pad(w *bytes.Buffer, rpmtype, offset int) error {
	// We need to align integer entries...
	if b, ok := boundaries[rpmtype]; ok && offset%b != 0 {
		if _, err := w.Write(make([]byte, b-offset%b)); err != nil {
			// binary.Write can fail if the underlying Write fails, or the types are invalid.
			// bytes.Buffer's write never error out, it can only panic with OOM.
			return err
		}
	}

	return nil
}

// Bytes returns the bytes of the index.
func (i *index) Bytes() ([]byte, error) {
	var err error
	var entry *IndexEntry
	var entryBytes []byte
	w := &bytes.Buffer{}
	// Even the header has three parts: The lead, the index entries, and the entries.
	// Because of alignment, we can only tell the actual size and offset after writing
	// the entries.
	entryData := &bytes.Buffer{}
	tags := i.sortedTags()
	offsets := make([]int, len(tags))
	for ii, tag := range tags {
		e := i.entries[tag]
		if err := pad(entryData, e.rpmtype, entryData.Len()); err != nil {
			return nil, err
		}
		offsets[ii] = entryData.Len()
		entryData.Write(e.data)
	}
	if entry, err = i.eigenHeader(); err != nil {
		return nil, err
	}
	entryData.Write(entry.data)

	// 4 magic and 4 reserved
	w.Write([]byte{0x8e, 0xad, 0xe8, 0x01, 0, 0, 0, 0})
	// 4 count and 4 size
	// We add the pseudo-NewIndexEntry "eigenHeader" to count.
	if err = binary.Write(w, binary.BigEndian, []int32{int32(len(i.entries)) + 1, int32(entryData.Len())}); err != nil {
		return nil, errors.Wrap(err, "failed to write eigenHeader")
	}
	// Write the eigenHeader index NewIndexEntry
	if entry, err = i.eigenHeader(); err != nil {
		return nil, err
	}
	if entryBytes, err = entry.indexBytes(i.h, entryData.Len()-0x10); err != nil {
		return nil, err
	}
	w.Write(entryBytes)
	// Write all of the other index entries
	var idxBytes []byte
	for ii, tag := range tags {
		e := i.entries[tag]
		if idxBytes, err = e.indexBytes(tag, offsets[ii]); err != nil {
			return nil, err
		}
		w.Write(idxBytes)
	}
	w.Write(entryData.Bytes())
	return w.Bytes(), nil
}

// the eigenHeader is a weird NewIndexEntry. Its index NewIndexEntry is sorted first, but its content
// is last. The content is a 16 byte index NewIndexEntry, which is almost the same as the index
// NewIndexEntry except for the offset. The offset here is ... minus the length of the index NewIndexEntry region.
// Which is always 0x10 * number of entries.
// I kid you not.
func (i *index) eigenHeader() (*IndexEntry, error) {
	b := &bytes.Buffer{}
	if err := binary.Write(b, binary.BigEndian, []int32{int32(i.h), int32(typeBinary), -int32(0x10 * (len(i.entries) + 1)), int32(0x10)}); err != nil {
		// binary.Write can fail if the underlying Write fails, or the types are invalid.
		// bytes.Buffer's write never error out, it can only panic with OOM.
		return nil, err
	}

	return NewIndexEntry(b.Bytes())
}

func lead(name, fullVersion string) []byte {
	// RPM format = 0xedabeedb
	// version 3.0 = 0x0300
	// type binary = 0x0000
	// machine archnum (i386?) = 0x0001
	// name ( 66 bytes, with null termination)
	// osnum (linux?) = 0x0001
	// sig type (header-style) = 0x0005
	// reserved 16 bytes of 0x00
	n := []byte(fmt.Sprintf("%s-%s", name, fullVersion))
	if len(n) > 65 {
		n = n[:65]
	}
	n = append(n, make([]byte, 66-len(n))...)
	b := []byte{0xed, 0xab, 0xee, 0xdb, 0x03, 0x00, 0x00, 0x00, 0x00, 0x01}
	b = append(b, n...)
	b = append(b, []byte{0x00, 0x01, 0x00, 0x05}...)
	b = append(b, make([]byte, 16)...)
	return b
}
