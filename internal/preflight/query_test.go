package preflight

import "testing"

// Users type locations the way they read them in docs and the CLI help:
// "fsn1, nbg1", "FSN1 NBG1", or with stray spaces. All must ask one question.
func TestWordsAcceptsCommasSpacesAndCase(t *testing.T) {
	for _, q := range []string{"fsn1,nbg1", "fsn1, nbg1", " FSN1  nbg1 ", "fsn1 ,, nbg1"} {
		got := Words(q)
		if !equal(got, []string{"fsn1", "nbg1"}) {
			t.Errorf("Words(%q) = %v, want [fsn1 nbg1]", q, got)
		}
	}
	if got := Words("  "); len(got) != 0 {
		t.Errorf("Words(blank) = %v, want none: a blank query means 'everything'", got)
	}
}

// An empty query is the full picture, not an error.
func TestAskNoWordsReturnsEverything(t *testing.T) {
	got, err := Ask(fixture(), nil)
	if err != nil {
		t.Fatalf("Ask: %v", err)
	}
	if got.Mode != ModeAll || len(got.All) != len(fixture()) {
		t.Errorf("got mode %q with %d rows, want %q with %d", got.Mode, len(got.All), ModeAll, len(fixture()))
	}
}

// `<type> <location>` is a yes/no question and must say which pair it answered.
func TestAskTypeAndLocationIsLookup(t *testing.T) {
	got, err := Ask(fixture(), []string{"cx22", "nbg1"})
	if err != nil {
		t.Fatalf("Ask: %v", err)
	}
	if got.Mode != ModeLookup || got.ServerType != "cx22" || got.Location != "nbg1" || got.Available {
		t.Errorf("got %+v, want lookup cx22 in nbg1 = not available", got)
	}
}

// Two locations must list both locations, never be read as "server type fsn1
// in hel1" (which would fail with a confusing unknown-type error).
func TestAskTwoLocationsIsNotALookup(t *testing.T) {
	got, err := Ask(fixture(), []string{"fsn1", "hel1"})
	if err != nil {
		t.Fatalf("Ask: %v", err)
	}
	if got.Mode != ModeLocations || len(got.Groups) != 2 {
		t.Fatalf("got mode %q with %d groups, want %q with 2", got.Mode, len(got.Groups), ModeLocations)
	}
}

// A typo in either word must surface as an error, never as "not available":
// the user would otherwise pick another location for no reason.
func TestAskTyposAreErrors(t *testing.T) {
	for _, words := range [][]string{
		{"cx2", "fsn1"},  // unknown type
		{"cx22", "fsn9"}, // unknown location for a lookup
		{"fsn"},          // unknown location in a list
	} {
		if got, err := Ask(fixture(), words); err == nil {
			t.Errorf("Ask(%v) = %+v, want an error", words, got)
		}
	}
}
