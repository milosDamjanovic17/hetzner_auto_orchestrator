package main

import (
	"reflect"
	"strings"
	"testing"
)

// The GUI's one hard security rule: no bound method may hand a token to
// JavaScript. Everything returned crosses into the web layer, where a token
// could be logged, rendered or leaked. This is the Phase 2 counterpart of the
// secrets package's plaintext-leak test: the mistake most likely to slip in
// silently, say by returning a config.Context instead of a service.ContextInfo.

var errorType = reflect.TypeOf((*error)(nil)).Elem()

// tokenFields walks t and returns the path of every field that could carry a
// token to JS: an exported field whose Go or JSON name contains "token" (any
// case), unless it is a bool, which cannot hold a secret (Status.EnvTokenIgnored).
// An interface field is reported too: what it carries at run time is unknown.
func tokenFields(t reflect.Type, path string, seen map[reflect.Type]bool) []string {
	switch t.Kind() {
	case reflect.Pointer, reflect.Slice, reflect.Array:
		return tokenFields(t.Elem(), path+"[]", seen)
	case reflect.Map:
		return append(tokenFields(t.Key(), path+"{key}", seen), tokenFields(t.Elem(), path+"{}", seen)...)
	case reflect.Interface:
		return []string{path + " (interface: contents unknown)"}
	case reflect.Struct:
	default:
		return nil
	}

	// A type already walked was reported the first time; this also stops
	// self-referencing types from recursing forever.
	if seen[t] {
		return nil
	}
	seen[t] = true

	var found []string
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		// JSON skips unexported fields, so they never reach JS. This also
		// keeps the walk out of time.Time's internals.
		if !f.IsExported() {
			continue
		}
		jsonName, _, _ := strings.Cut(f.Tag.Get("json"), ",")
		if jsonName == "-" {
			continue
		}
		fpath := path + "." + f.Name
		named := strings.Contains(strings.ToLower(f.Name), "token") ||
			strings.Contains(strings.ToLower(jsonName), "token")
		if named && f.Type.Kind() != reflect.Bool {
			found = append(found, fpath)
			continue
		}
		found = append(found, tokenFields(f.Type, fpath, seen)...)
	}
	return found
}

// boundMethods returns every method Wails binds: the exported methods of
// *App. The pointer matters: methods on a pointer receiver are not in the
// method set of App itself, and the test would then check nothing.
func boundMethods() []reflect.Method {
	t := reflect.TypeOf(&App{})
	methods := make([]reflect.Method, t.NumMethod())
	for i := range methods {
		methods[i] = t.Method(i)
	}
	return methods
}

func TestNoBoundMethodReturnsAToken(t *testing.T) {
	for _, m := range boundMethods() {
		for i := 0; i < m.Type.NumOut(); i++ {
			out := m.Type.Out(i)
			// A returned error reaches JS only as its message, and messages
			// never contain a token (see the service tests).
			if out == errorType {
				continue
			}
			for _, f := range tokenFields(out, m.Name+"()", map[reflect.Type]bool{}) {
				t.Errorf("bound method result may carry a token to JS: %s", f)
			}
		}
	}
}

// Guards the test above against passing because it looked at nothing: if the
// method set were read from App instead of *App, or a refactor moved the
// bound methods elsewhere, every check would silently pass.
func TestBoundMethodsAreTheOnesTheFrontendCalls(t *testing.T) {
	got := map[string]bool{}
	for _, m := range boundMethods() {
		got[m.Name] = true
	}
	for _, name := range []string{"Status", "Contexts", "AddContext", "Servers", "Zones", "Preflight", "ConsoleURL"} {
		if !got[name] {
			t.Errorf("method %s not found on *App; the boundary test is not seeing the bound methods", name)
		}
	}
}

// The walker itself must catch the leaks it exists for, including ones nested
// inside generics, slices and pointers, and ones named differently than "Token".
func TestTokenFieldsCatchesLeaks(t *testing.T) {
	type secret struct {
		Name     string
		APIToken string // renamed, still a token
	}
	type tagged struct {
		Value string `json:"hcloud_token"` // innocent Go name, leaky JSON name
	}
	type flagOnly struct {
		EnvTokenIgnored bool // a bool cannot carry a secret: allowed
	}
	type loose struct {
		Extra any // could hold anything at run time
	}
	cases := []struct {
		name string
		typ  reflect.Type
		want int
	}{
		{"nested in generic listing", reflect.TypeOf(Listing[*secret]{}), 1},
		{"json tag", reflect.TypeOf(map[string][]tagged{}), 1},
		{"bool flag", reflect.TypeOf(flagOnly{}), 0},
		{"interface field", reflect.TypeOf(loose{}), 1},
	}
	for _, c := range cases {
		got := tokenFields(c.typ, "x", map[reflect.Type]bool{})
		if len(got) != c.want {
			t.Errorf("%s: found %v, want %d finding(s)", c.name, got, c.want)
		}
	}
}
