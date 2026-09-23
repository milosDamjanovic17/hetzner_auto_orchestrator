package preflight

import "testing"

// fixture mirrors the shape All returns: sorted by server type, then location.
func fixture() []Availability {
	return []Availability{
		{ServerType: "cax11", Location: "fsn1", City: "Falkenstein", Available: false},
		{ServerType: "cax11", Location: "hel1", City: "Helsinki", Available: true},
		{ServerType: "cx22", Location: "fsn1", City: "Falkenstein", Available: true},
		{ServerType: "cx22", Location: "hel1", City: "Helsinki", Available: true},
		{ServerType: "cx22", Location: "nbg1", City: "Nuremberg", Available: false},
		{ServerType: "cx32", Location: "fsn1", City: "Falkenstein", Available: true},
	}
}

// names flattens groups to "type@location" in output order.
func names(groups []LocationAvailability) []string {
	out := []string{}
	for _, g := range groups {
		for _, r := range g.Available {
			out = append(out, r.ServerType+"@"+r.Location)
		}
	}
	return out
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// The question is "what can I deploy there right now". Listing a type that is
// out of stock answers a different question and invites a create that fails.
func TestAvailableInExcludesUnavailable(t *testing.T) {
	got, err := AvailableIn(fixture(), []string{"fsn1"})
	if err != nil {
		t.Fatalf("AvailableIn: %v", err)
	}
	want := []string{"cx22@fsn1", "cx32@fsn1"}
	if !equal(names(got), want) {
		t.Errorf("got %v, want %v", names(got), want)
	}
}

// Output is read per location in the order the user typed them; asking for a
// location twice must not print its servers twice.
func TestAvailableInKeepsOrderDropsDuplicates(t *testing.T) {
	got, err := AvailableIn(fixture(), []string{"hel1", "fsn1", "hel1"})
	if err != nil {
		t.Fatalf("AvailableIn: %v", err)
	}
	want := []string{"cax11@hel1", "cx22@hel1", "cx22@fsn1", "cx32@fsn1"}
	if !equal(names(got), want) {
		t.Errorf("got %v, want %v", names(got), want)
	}
	if len(got) != 2 {
		t.Errorf("got %d groups, want 2", len(got))
	}
}

// A typo like "fsn" must be an error. An empty result would read as "nothing
// in stock in Falkenstein", which is false and would send the user elsewhere.
func TestAvailableInUnknownLocationErrors(t *testing.T) {
	if _, err := AvailableIn(fixture(), []string{"fsn1", "fsn"}); err == nil {
		t.Fatal("unknown location returned no error")
	}
}

// The counterpart to the typo case: a real location with nothing in stock is
// a valid answer: an empty group, not an error and not a missing group.
func TestAvailableInKnownLocationNothingInStock(t *testing.T) {
	got, err := AvailableIn(fixture(), []string{"nbg1"})
	if err != nil {
		t.Fatalf("AvailableIn: %v", err)
	}
	if len(got) != 1 || got[0].Location != "nbg1" || len(got[0].Available) != 0 {
		t.Errorf("got %+v, want one empty nbg1 group", got)
	}
}

// The group carries the city so output can say "Falkenstein", not only "fsn1".
func TestAvailableInCarriesCity(t *testing.T) {
	got, err := AvailableIn(fixture(), []string{"fsn1"})
	if err != nil {
		t.Fatalf("AvailableIn: %v", err)
	}
	if got[0].City != "Falkenstein" {
		t.Errorf("city = %q, want Falkenstein", got[0].City)
	}
}

// The CLI tells `preflight fsn1 nbg1` (two locations) from `preflight cx22
// fsn1` (type + location) with IsLocation. A server type matching as a
// location would silently turn a pair query into a location listing.
func TestIsLocationDoesNotMatchServerTypes(t *testing.T) {
	all := fixture()
	if !IsLocation(all, "fsn1") {
		t.Error("fsn1 not recognised as a location")
	}
	if IsLocation(all, "cx22") {
		t.Error("server type cx22 recognised as a location")
	}
}

func TestLookup(t *testing.T) {
	all := fixture()
	if ok, err := Lookup(all, "cx22", "fsn1"); err != nil || !ok {
		t.Errorf("cx22@fsn1 = %v, %v; want true, nil", ok, err)
	}
	if ok, err := Lookup(all, "cx22", "nbg1"); err != nil || ok {
		t.Errorf("cx22@nbg1 = %v, %v; want false, nil", ok, err)
	}
	// Both of these must be errors, never a bare false: a typo is not
	// "out of stock".
	if _, err := Lookup(all, "cx2", "fsn1"); err == nil {
		t.Error("unknown server type returned no error")
	}
	if _, err := Lookup(all, "cx32", "hel1"); err == nil {
		t.Error("type not offered in location returned no error")
	}
}
