package preflight

import (
	"strings"
	"unicode"
)

// The three kinds of question a preflight query can ask.
const (
	ModeAll       = "all"       // no words: every type in every location
	ModeLookup    = "lookup"    // `<type> <location>`: one yes/no
	ModeLocations = "locations" // one or more locations: what is available there
)

// Words splits a query the way the CLI reads its arguments: lower-cased,
// separated by commas and/or whitespace. "fsn1, nbg1" and "fsn1 nbg1" ask the
// same question.
func Words(query string) []string {
	return strings.FieldsFunc(strings.ToLower(query), func(r rune) bool {
		return r == ',' || unicode.IsSpace(r)
	})
}

// Answer is the reply to one query. Mode says which of the other fields is
// set; the rest stay empty.
type Answer struct {
	Mode       string                 `json:"mode"`
	All        []Availability         `json:"all"`        // ModeAll
	ServerType string                 `json:"serverType"` // ModeLookup
	Location   string                 `json:"location"`   // ModeLookup
	Available  bool                   `json:"available"`  // ModeLookup
	Groups     []LocationAvailability `json:"groups"`     // ModeLocations
}

// Ask answers a query against data already fetched with All. It is the one
// place that decides what a query means, so the CLI and the GUI cannot give
// different answers to the same words.
func Ask(all []Availability, words []string) (Answer, error) {
	switch {
	case len(words) == 0:
		return Answer{Mode: ModeAll, All: all}, nil

	// Two words where the first is not a location: `<type> <location>`.
	// Otherwise `fsn1 nbg1` would be read as server type "fsn1".
	case len(words) == 2 && !IsLocation(all, words[0]):
		ok, err := Lookup(all, words[0], words[1])
		if err != nil {
			return Answer{}, err
		}
		return Answer{Mode: ModeLookup, ServerType: words[0], Location: words[1], Available: ok}, nil

	default:
		groups, err := AvailableIn(all, words)
		if err != nil {
			return Answer{}, err
		}
		return Answer{Mode: ModeLocations, Groups: groups}, nil
	}
}
