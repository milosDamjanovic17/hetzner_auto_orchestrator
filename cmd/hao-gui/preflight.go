package main

import (
	"context"

	"github.com/milosDamjanovic17/hetzner_auto_orchestrator/internal/hetzner"
	"github.com/milosDamjanovic17/hetzner_auto_orchestrator/internal/preflight"
)

// PreflightResult is one preflight answer plus the context it was fetched
// with, for the same reason as Listing.
type PreflightResult struct {
	Context string           `json:"context"`
	Answer  preflight.Answer `json:"answer"`
}

// Preflight answers a query typed the way the CLI takes it: "fsn1, nbg1",
// "ccx13 ash", or empty for everything. Parsing and meaning live in
// internal/preflight, shared with the CLI, so both give the same answer.
func (a *App) Preflight(query string) (PreflightResult, error) {
	svc, err := a.service()
	if err != nil {
		return PreflightResult{}, err
	}
	name, checker, err := svc.ActiveChecker()
	if err != nil {
		return PreflightResult{}, err
	}
	ctx, cancel := context.WithTimeout(a.ctx, apiTimeout)
	defer cancel()
	all, err := checker.All(ctx)
	if err != nil {
		return PreflightResult{}, err
	}
	ans, err := preflight.Ask(all, preflight.Words(query))
	if err != nil {
		return PreflightResult{}, err
	}
	return PreflightResult{Context: name, Answer: ans}, nil
}

// ConsoleURL is where API tokens and project members are managed. The page
// opens it in the system browser, not inside the app window.
func (a *App) ConsoleURL() string {
	return hetzner.ConsoleURL
}
