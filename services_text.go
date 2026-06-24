package main

import (
	"github.com/nic/devtoolkit/internal/pkg/differ"
	"github.com/nic/devtoolkit/internal/pkg/mockgen"
	"github.com/nic/devtoolkit/internal/pkg/regexx"
)

// TextService exposes text/JSON diff (R1), regex testing (R3) and mock data (R4). All local.
type TextService struct{}

// Diff compares two texts side-by-side, optionally normalizing JSON first.
func (s *TextService) Diff(left string, right string, opts differ.Options) (differ.Result, error) {
	return differ.Diff(left, right, opts)
}

// TestRegex runs a pattern against text and returns highlighted matches.
func (s *TextService) TestRegex(pattern string, text string, flags regexx.Flags) (regexx.Result, error) {
	return regexx.Test(pattern, text, flags)
}

// RegexLibrary returns the built-in common regex presets.
func (s *TextService) RegexLibrary() []regexx.Preset {
	return regexx.Library()
}

// GenerateMock produces count random items of the given kind
// ("name" | "address" | "phone" | "email" | "bankcard" | "lorem").
func (s *TextService) GenerateMock(kind string, count int) ([]string, error) {
	return mockgen.Generate(mockgen.Kind(kind), count)
}
