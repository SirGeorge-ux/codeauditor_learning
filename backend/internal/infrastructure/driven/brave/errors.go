package brave

// Package-local error variables live alongside the adapter so callers
// can branch on adapter-specific failures without importing net/http
// or any third-party SDK. errMissingAPIKey (in adapter.go) is the
// canonical sentinel for misconfigured adapters; no sentinel remains
// for the legacy "not implemented" stub (PR 4 fully wired the
// HTTP path; the stub placeholder was removed).
