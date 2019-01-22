package rpmpack

import (
	"bytes"
	"errors"
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
	Name string
	Body []byte
	Mode int
}

type rpm struct {
	md         RPMMetaData
	di         *DirIndex
	payload    *bytes.Buffer
	cpio       *cpio.Writer
	basenames  []string
	dirindexes []int32
	closed     bool
}

func NewRPM(m RPMMetaData) *rpm {
	p := &bytes.Buffer{}
	return &rpm{
		md:      m,
		di:      NewDirIndex(),
		payload: p,
		cpio:    cpio.NewWriter(p),
	}
}

// Write closes the rpm and writes the whole rpm to an io.Writer
func (r *rpm) Write(w io.Writer) error {
	if r.closed {
		return ErrWriteAfterClose
	}
	if err := r.cpio.Close(); err != nil {
		return err
	}

	if _, err := w.Write(Lead(r.md.Name, r.md.Version, r.md.Release)); err != nil {
		return err
	}
	s := NewIndex(signatures)
	if err := s.Write(w); err != nil {
		return err
	}
	h := NewIndex(immutable)
	r.writeFileIndexes(h)
	if err := h.Write(w); err != nil {
		return err
	}

	if _, err := w.Write(r.payload.Bytes()); err != nil {
		return err
	}
	return nil

}

// WriteFileIndexes writes file related index headers to the header
func (r *rpm) writeFileIndexes(h *index) error {
	h.Add(tagBasenames, StringArrayEntry(r.basenames))
	h.Add(tagDirindexes, Int32Entry(r.dirindexes))
	h.Add(tagDirnames, StringArrayEntry(r.di.AllDirs()))
	return nil
}

func (r *rpm) AddFile(f RPMFile) error {
	dir, file := path.Split(f.Name)
	r.dirindexes = append(r.dirindexes, r.di.Get(dir))
	r.basenames = append(r.basenames, file)
	r.writePayload(f)
	return nil
}

func (r *rpm) writePayload(f RPMFile) error {
	hdr := &cpio.Header{
		Name: f.Name,
		Mode: cpio.FileMode(f.Mode),
		Size: int64(len(f.Body)),
	}
	if err := r.cpio.WriteHeader(hdr); err != nil {
		return err
	}
	if _, err := r.cpio.Write(f.Body); err != nil {
		return err
	}
	return nil
}
