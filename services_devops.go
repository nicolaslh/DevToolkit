package main

import (
	"github.com/nic/devtoolkit/internal/pkg/chmod"
	"github.com/nic/devtoolkit/internal/pkg/cronx"
	"github.com/nic/devtoolkit/internal/pkg/pathx"
)

// DevOpsService exposes Cron (R9), Chmod (R10) and Env/Path (R11). All local.
type DevOpsService struct{}

// BuildCron assembles a 5-field cron expression from visual field selections.
func (s *DevOpsService) BuildCron(f cronx.Fields) string {
	return cronx.Build(f)
}

// ParseCron validates an expression and returns its description and next 5 runs.
func (s *DevOpsService) ParseCron(expr string) (cronx.Info, error) {
	return cronx.Parse(expr)
}

// ChmodFromNumeric converts a numeric permission value to symbolic + checkbox state.
func (s *DevOpsService) ChmodFromNumeric(numeric string) (chmod.Result, error) {
	return chmod.FromNumeric(numeric)
}

// ChmodFromPerms converts checkbox state to numeric + symbolic representations.
func (s *DevOpsService) ChmodFromPerms(p chmod.Perms) chmod.Result {
	return chmod.FromPerms(p)
}

// SplitPath splits a PATH string into ordered entries.
func (s *DevOpsService) SplitPath(raw string) ([]string, error) {
	return pathx.Split(raw)
}

// DedupPath removes duplicate entries, keeping first occurrence.
func (s *DevOpsService) DedupPath(entries []string) []string {
	return pathx.Dedup(entries)
}

// ComparePath classifies entries as left-only / right-only / shared.
func (s *DevOpsService) ComparePath(a string, b string) ([]pathx.DiffEntry, error) {
	return pathx.Compare(a, b)
}

// JoinPath recombines entries into a single PATH string.
func (s *DevOpsService) JoinPath(entries []string) string {
	return pathx.Join(entries)
}
