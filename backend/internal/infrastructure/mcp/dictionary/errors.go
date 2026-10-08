package dictionary

import (
	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

// dictionaryErrNotFound is a re-exported alias used by the Get tool's
// error-mapping path so the wire shape ("null on missing") stays inside
// this package without leaking to importers. It MUST equal
// ports.ErrTermNotFound byte-for-byte because the adapter compares with
// errors.Is.
var dictionaryErrNotFound = ports.ErrTermNotFound
