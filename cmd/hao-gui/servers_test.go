package main

import (
	"errors"
	"fmt"
	"testing"

	"github.com/milosDamjanovic17/hetzner_auto_orchestrator/internal/hetzner"
	"github.com/milosDamjanovic17/hetzner_auto_orchestrator/internal/preflight"
)

// An out-of-stock type in the form would cost the user a cost check and a
// confirm, only for Hetzner to refuse the create.
func TestCreateOptionsOffersOnlyAvailablePairs(t *testing.T) {
	o := createOptions("moose02", []preflight.Availability{
		{ServerType: "cpx02", Location: "fsn1", Available: true},
		{ServerType: "cpx02", Location: "ash", Available: false},
	}, nil, nil)
	if len(o.Types) != 1 || o.Types[0].Location != "fsn1" {
		t.Errorf("Types = %+v, want only cpx02 in fsn1", o.Types)
	}
	// The project travels with the options: it is what the create checks against.
	if o.Context != "moose02" {
		t.Errorf("Context = %q, want moose02", o.Context)
	}
	if o.Images == nil || o.SSHKeys == nil {
		t.Error("nil lists reach the page as null; want empty lists")
	}
}

// "Still running" means Hetzner accepted the write: shown as a notice, not
// an error. Every other error, "outcome unknown" above all, must stay an
// error: the page must never say a write worked when it may not have.
func TestFinishOnlyTurnsStillRunningIntoANotice(t *testing.T) {
	running := fmt.Errorf("snapshot of server 42: %w", hetzner.ErrStillRunning)
	if note, err := finish(running); err != nil || note != running.Error() {
		t.Errorf("still running: got (%q, %v), want the message as a notice", note, err)
	}
	unknown := fmt.Errorf("deleting server 42: %w", hetzner.ErrOutcomeUnknown)
	if _, err := finish(unknown); !errors.Is(err, hetzner.ErrOutcomeUnknown) {
		t.Errorf("outcome unknown: got %v, want it returned as an error", err)
	}
	if note, err := finish(nil); note != "" || err != nil {
		t.Errorf("done: got (%q, %v), want (\"\", nil)", note, err)
	}
}
