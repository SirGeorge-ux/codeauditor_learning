package brave

import "errors"

// errNotImplemented marked the PR 1 stub adapter's methods. The PR 4
// implementation removed the dependency from each method; the variable
// is left for any reference inside other code (search-replace guard)
// but is no longer wired into any control flow.
var errNotImplemented = errors.New("brave: not implemented in this PR")
