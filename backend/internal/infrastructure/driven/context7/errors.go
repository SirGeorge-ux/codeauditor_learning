package context7

import "errors"

// errMissingLibrary is returned by GetLibraryDocs when the caller
// passes an empty library segment. The MCP tool layer validates this
// up front; the adapter-level guard is defense-in-depth (a call site
// that bypasses the tool layer — e.g. a unit test that builds the
// adapter directly — still fails fast rather than producing an
// upstream 404).
var errMissingLibrary = errors.New("context7: library is required")

// errNotImplemented marked the PR 1 stub adapter's methods. The PR 4
// implementation removed the dependency from each method; the variable
// is left for any reference inside other code (search-replace guard)
// but is no longer wired into any control flow.
var errNotImplemented = errors.New("context7: not implemented in this PR")
