package rpmpack

import (
	"fmt"
	"regexp"
)

type rpmSense uint32

// SenseAny specifies no specific version compare
// SenseLess specifies less then the specified version
// SenseGreater specifies greater then the specified version
// SenseEqual specifies equal to the specified version
const (
	SenseAny  rpmSense = 0
	SenseLess rpmSense = 1 << iota
	SenseGreater
	SenseEqual
)

type relationCategory string

const (
	RequiresCategory   relationCategory = "requires"
	ObsoletesCategory  relationCategory = "obsoletes"
	SuggestsCategory   relationCategory = "suggests"
	RecommendsCategory relationCategory = "recommends"
	ConflictsCategory  relationCategory = "conflicts"
	ProvidesCategory   relationCategory = "provides"
)

var relationMatch = regexp.MustCompile(`([^=<>\s]*)\s*((?:=|>|<|>=|<=)*)\s*(.*)?`)

// Relation is the structure of rpm sense relationships
type Relation struct {
	Name    string
	Version string
	Sense   rpmSense
}

// String return the string representation of the Relation
func (r *Relation) String() string {
	return fmt.Sprintf("%s%v%s", r.Name, r.Sense, r.Version)
}

// GoString return the string representation of the Relation
func (r *Relation) GoString() string {
	return r.String()
}

// Equal compare the equality of two relations
func (r *Relation) Equal(o *Relation) bool {
	return r.String() == o.String()
}

// Relations is a slice of Relation pointers
type Relations []*Relation

// String return the string representation of the Relations
func (r *Relations) String() string {
	var (
		val   string
		total = len(*r)
	)

	for idx, relation := range *r {
		val += fmt.Sprintf("%s%v%s", relation.Name, relation.Sense, relation.Version)
		if idx < total-1 {
			val += ","
		}
	}

	return val
}

// GoString return the string representation of the Relations
func (r *Relations) GoString() string {
	return r.String()
}

// Set parse a string into a Relation and append it to the Relations slice if it is missing
// this is used by the flag package
func (r *Relations) Set(value string) error {
	relation, err := NewRelation(value)
	if err != nil {
		return err
	}
	r.addIfMissing(relation)

	return nil
}

func (r *Relations) addIfMissing(value *Relation) {
	for _, relation := range *r {
		if relation.Equal(value) {
			return
		}
	}

	*r = append(*r, value)
}

// AddToIndex add the relations to the specified category on the index
func (r *Relations) AddToIndex(category relationCategory, h *index) error {
	var (
		nameTag,
		versionTag,
		flagsTag int
		num      = len(*r)
		names    = make([]string, num)
		versions = make([]string, num)
		flags    = make([]uint32, num)
	)

	if num == 0 {
		return nil
	}

	switch category {
	case ProvidesCategory:
		nameTag = tagProvides
		versionTag = tagProvideVersion
		flagsTag = tagProvideFlags
	case RequiresCategory:
		nameTag = tagRequires
		versionTag = tagRequireVersion
		flagsTag = tagRequireFlags
	case ObsoletesCategory:
		nameTag = tagObsoletes
		versionTag = tagObsoleteVersion
		flagsTag = tagObsoleteFlags
	case SuggestsCategory:
		nameTag = tagSuggests
		versionTag = tagSuggestVersion
		flagsTag = tagSuggestFlags
	case RecommendsCategory:
		nameTag = tagRecommends
		versionTag = tagRecommendVersion
		flagsTag = tagRecommendFlags
	case ConflictsCategory:
		nameTag = tagConflicts
		versionTag = tagConflictVersion
		flagsTag = tagConflictFlags
	default:
		return fmt.Errorf("unknown category %s", category)
	}

	for idx := range *r {
		relation := (*r)[idx]
		names[idx] = relation.Name
		versions[idx] = relation.Version
		flags[idx] = uint32(relation.Sense)
	}

	h.Add(nameTag, entry(names))
	h.Add(versionTag, entry(versions))
	h.Add(flagsTag, entry(flags))

	return nil
}

// NewRelation parse a string into a Relation
func NewRelation(related string) (*Relation, error) {
	var (
		err   error
		sense rpmSense
	)
	parts := relationMatch.FindStringSubmatch(related)
	if sense, err = parseSense(parts[2]); err != nil {
		return nil, err
	}

	return &Relation{
		Name:    parts[1],
		Version: parts[3],
		Sense:   sense,
	}, nil
}

var senseStrings = map[rpmSense]string{
	SenseAny:                  "",
	SenseLess:                 "<",
	SenseGreater:              ">",
	SenseEqual:                "=",
	SenseLess | SenseEqual:    "<=",
	SenseGreater | SenseEqual: ">=",
}

// String return the string representation of the rpmSense
func (r rpmSense) String() string {
	var (
		ok  bool
		ret string
	)

	if ret, ok = senseStrings[r]; !ok {
		return "UNKNOWN"
	}

	return ret
}

// GoString return the string representation of the rpmSense
func (r rpmSense) GoString() string {
	return r.String()
}

func parseSense(sense string) (rpmSense, error) {
	var (
		ret     rpmSense
		toMatch string
	)
	for ret, toMatch = range senseStrings {
		if sense == toMatch {
			return ret, nil
		}
	}

	return ret, fmt.Errorf("unknown sense value")
}
