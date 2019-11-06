package rpmpack

import (
	"bytes"

	"github.com/pkg/errors"
)

var (
	ErrMissingRequiredTag = errors.New("required rpm tag is missing")
	ErrInvalidRPMTagType  = errors.New("rpm tag is wrong type")
)

type requiredInfo struct {
	rpmType     int
	description string
}

var requiredTags = map[string]map[int]*requiredInfo{
	"signatures": map[int]*requiredInfo{
		sigSHA256:      &requiredInfo{typeString, "signature sha256"},
		sigSize:        &requiredInfo{typeInt32, "signature size"},
		sigPayloadSize: &requiredInfo{typeInt32, "signature payload size"},
	},
	"payload": map[int]*requiredInfo{
		tagName:              &requiredInfo{typeString, "rpm name"},
		tagSummary:           &requiredInfo{typeString, "rpm summary"},
		tagDescription:       &requiredInfo{typeString, "rpm description"},
		tagVersion:           &requiredInfo{typeString, "rpm version"},
		tagRelease:           &requiredInfo{typeString, "rpm release"},
		tagSize:              &requiredInfo{typeInt32, "rpm size"},
		tagLicence:           &requiredInfo{typeString, "rpm licence"},
		tagGroup:             &requiredInfo{typeString, "rpm group"},
		tagOS:                &requiredInfo{typeString, "rpm os"},
		tagArch:              &requiredInfo{typeString, "rpm architecture"},
		tagPayloadFormat:     &requiredInfo{typeString, "rpm payload format"},
		tagPayloadCompressor: &requiredInfo{typeString, "rpm payload compressor"},
		tagPayloadFlags:      &requiredInfo{typeString, "rpm payload flags"},
	},
}

func (r *RPM) VerifyRequiredTags() error {
	var err error
	if err = r.verifySignature(); err != nil {
		return err
	}
	if err = r.verifyPayload(); err != nil {
		return err
	}

	return nil
}

func (r *RPM) verifySignature() error {
	var (
		ok    bool
		entry *IndexEntry
	)

	for tag, info := range requiredTags["signatures"] {
		if entry, ok = r.sigIndex.entries[tag]; !ok {
			return errors.Wrap(ErrMissingRequiredTag, info.description)
		}
		if err := verifyEntry(entry, info); err != nil {
			return err
		}
		if tag == sigPayloadSize && !bytes.Equal(entry.data, r.payloadIndex.entries[tagSize].data) {
			return errors.New("signature payload size does not match payload size")
		}
	}

	return nil
}

func (r *RPM) verifyPayload() error {
	var (
		ok    bool
		entry *IndexEntry
	)

	for tag, info := range requiredTags["payload"] {
		if entry, ok = r.payloadIndex.entries[tag]; !ok {
			return errors.Wrap(ErrMissingRequiredTag, info.description)
		}
		if err := verifyEntry(entry, info); err != nil {
			return err
		}
	}
	return nil
}

func verifyEntry(entry *IndexEntry, info *requiredInfo) error {
	if entry.rpmtype != info.rpmType {
		return errors.Wrapf(ErrInvalidRPMTagType, "%s got: %d expected: %d", info.description, entry.rpmtype, info.rpmType)
	}
	if len(entry.data) == 1 && entry.data[0] == 0 {
		return errors.Wrapf(ErrMissingRequiredTag, "%s cannot be empty", info.description)
	}

	return nil
}
