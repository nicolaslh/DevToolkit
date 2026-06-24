package main

import (
	"github.com/nic/devtoolkit/internal/pkg/curlconv"
	"github.com/nic/devtoolkit/internal/pkg/urlparse"
)

// NetworkService exposes URL parsing/rebuilding (R5) and cURL conversion (R6).
// The MVP scope is local-only; the HTTP client (R7) and network probe (R8) are
// added in a later iteration.
type NetworkService struct{}

// ParseURL decomposes a URL into protocol/host/port/path and query params.
func (s *NetworkService) ParseURL(raw string) (urlparse.Parts, error) {
	return urlparse.Parse(raw)
}

// BuildURL reassembles a URL from edited parts.
func (s *NetworkService) BuildURL(parts urlparse.Parts) (string, error) {
	return urlparse.Build(parts)
}

// ConvertCurl converts a cURL command into request code for the target language
// ("python" | "javascript" | "go" | "java").
func (s *NetworkService) ConvertCurl(cmd string, target string) (string, error) {
	return curlconv.Convert(cmd, target)
}
