package main

import (
	"github.com/nic/devtoolkit/internal/pkg/curlconv"
	"github.com/nic/devtoolkit/internal/pkg/dnsx"
	"github.com/nic/devtoolkit/internal/pkg/urlparse"
)

// NetworkService exposes URL parsing/rebuilding (R5), cURL conversion (R6) and
// DNS resolution (R8). The MVP scope is local-only; the HTTP client (R7) is
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

// ResolveDNS resolves A/AAAA/CNAME/MX/NS/TXT records for the given host.
func (s *NetworkService) ResolveDNS(host string) (dnsx.Result, error) {
	return dnsx.Resolve(host)
}
