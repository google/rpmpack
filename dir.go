package rpmpack

// DirIndex holds the index from files to directory names.
type DirIndex struct {
	m map[string]int32
	l []string
}

func NewDirIndex() *DirIndex {
	return &DirIndex{m: make(map[string]int32)}
}

func (d *DirIndex) Get(value string) int32 {

	if idx, ok := d.m[value]; ok {
		return idx
	}
	newIdx := int32(len(d.l))
	d.l = append(d.l, value)

	d.m[value] = newIdx
	return newIdx
}

func (d *DirIndex) AllDirs() []string {
	return d.l
}
