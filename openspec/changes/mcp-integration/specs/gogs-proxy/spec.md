# Delta for gogs-proxy

## ADDED Requirements

### Requirement: REQ-GP-001 — Backend MUST proxy Gogs API calls

The backend SHALL act as a secure proxy between the frontend and the Gogs instance. The Gogs API token MUST be stored exclusively in the `GOGS_TOKEN` environment variable and MUST never be exposed in HTTP responses, error messages, or log output. All Gogs requests SHALL originate from the backend process only.

(Previously: No Gogs integration existed; frontend had no way to access real repositories.)

#### Scenario: Token never exposed in response

- GIVEN a valid authenticated request to any `/api/v1/gogs/*` endpoint
- WHEN the backend forwards the request to the Gogs API using `GOGS_TOKEN`
- THEN the response to the frontend SHALL NOT contain the token or any substring of it
- AND the response SHALL contain only the proxied Gogs data or a structured error

#### Scenario: Token sanitized from logs

- GIVEN the GogsClient makes a request to the Gogs API
- WHEN the request or response is logged
- THEN the `GOGS_TOKEN` value SHALL be redacted or omitted from all log output
- AND no Authorization header value SHALL appear in plaintext in logs

#### Scenario: Frontend cannot supply its own token

- GIVEN a request to `/api/v1/gogs/repos` or `/api/v1/gogs/file`
- WHEN the request includes a custom `X-Gogs-Token` or similar header
- THEN the backend SHALL ignore the supplied token
- AND the backend SHALL use only the server-side `GOGS_TOKEN` environment variable

### Requirement: REQ-GP-002 — MUST support listing user repositories

The backend SHALL expose `GET /api/v1/gogs/repos` which calls the Gogs API to list repositories accessible to the authenticated user. The endpoint SHALL return a JSON array of repository objects containing at minimum: `id`, `full_name`, `description`, `private`, `default_branch`.

#### Scenario: Successful repo list

- GIVEN a valid JWT in the Authorization header
- WHEN the client sends `GET /api/v1/gogs/repos`
- THEN the backend SHALL forward the request to the Gogs `/api/v1/user/repos` endpoint
- AND the response SHALL be a JSON array of repository objects
- AND the HTTP status SHALL be 200

#### Scenario: Empty repository list

- GIVEN a valid JWT and a user with no repositories
- WHEN the client sends `GET /api/v1/gogs/repos`
- THEN the backend SHALL return an empty JSON array `[]`
- AND the HTTP status SHALL be 200

#### Scenario: Gogs API returns error

- GIVEN the Gogs instance is unreachable or returns a 5xx error
- WHEN the client sends `GET /api/v1/gogs/repos`
- THEN the backend SHALL return a structured error response with `{"error": "<message>", "code": "<error_code>"}`
- AND the HTTP status SHALL reflect the Gogs error (e.g., 502 for unreachable)

### Requirement: REQ-GP-003 — MUST support fetching file contents

The backend SHALL expose `POST /api/v1/gogs/file` accepting a JSON body with `owner` (string), `repo` (string), `branch` (string), and `path` (string). The endpoint SHALL call the Gogs API to retrieve the file contents and return them as JSON with at minimum: `name`, `path`, `content` (base64-encoded), `encoding`, `size` (bytes).

#### Scenario: Successful file fetch

- GIVEN a valid JWT and a valid file path in an existing repository
- WHEN the client sends `POST /api/v1/gogs/file` with `{owner, repo, branch, path}`
- THEN the backend SHALL call the Gogs `/api/v1/repos/{owner}/{repo}/contents/{path}?ref={branch}` endpoint
- AND the response SHALL include `name`, `path`, `content`, `encoding`, and `size`
- AND the HTTP status SHALL be 200

#### Scenario: File not found

- GIVEN a valid JWT but a non-existent file path
- WHEN the client sends `POST /api/v1/gogs/file` with `{owner, repo, branch, path}`
- THEN the backend SHALL return `{"error": "file not found", "code": "FILE_NOT_FOUND"}`
- AND the HTTP status SHALL be 404

#### Scenario: Missing required fields

- GIVEN a valid JWT
- WHEN the client sends `POST /api/v1/gogs/file` with a body missing any of `owner`, `repo`, `branch`, or `path`
- THEN the backend SHALL return `{"error": "missing required field: <field>", "code": "BAD_REQUEST"}`
- AND the HTTP status SHALL be 400

### Requirement: REQ-GP-004 — MUST return structured errors for Gogs API failures

All error responses from the gogs-proxy endpoints SHALL follow a consistent JSON structure: `{"error": "<human-readable message>", "code": "<MACHINE_READABLE_CODE>"}`. The `code` field SHALL use uppercase snake_case identifiers (e.g., `GOGS_UNREACHABLE`, `FILE_NOT_FOUND`, `AUTH_FAILED`).

#### Scenario: Gogs instance unreachable

- GIVEN the Gogs instance is down or unreachable
- WHEN any `/api/v1/gogs/*` endpoint is called
- THEN the backend SHALL return `{"error": "Gogs service is unavailable", "code": "GOGS_UNREACHABLE"}`
- AND the HTTP status SHALL be 502

#### Scenario: Gogs authentication failure

- GIVEN the `GOGS_TOKEN` is invalid or expired
- WHEN any `/api/v1/gogs/*` endpoint is called
- THEN the backend SHALL return `{"error": "Gogs authentication failed", "code": "GOGS_AUTH_FAILED"}`
- AND the HTTP status SHALL be 502
- AND the response SHALL NOT include the token value

#### Scenario: Gogs rate limit exceeded

- GIVEN the Gogs API returns a 403 rate limit response
- WHEN any `/api/v1/gogs/*` endpoint is called
- THEN the backend SHALL return `{"error": "Gogs rate limit exceeded", "code": "GOGS_RATE_LIMITED"}`
- AND the HTTP status SHALL be 429

### Requirement: REQ-GP-005 — SHOULD enforce a maximum file size

The GogsClient SHALL reject files exceeding 1 MB (1,048,576 bytes). When a file exceeds this limit, the endpoint SHALL return a structured error with code `FILE_TOO_LARGE`.

#### Scenario: File within size limit

- GIVEN a file of 500 KB exists in a repository
- WHEN the client requests it via `POST /api/v1/gogs/file`
- THEN the backend SHALL return the file contents normally
- AND the HTTP status SHALL be 200

#### Scenario: File exceeds size limit

- GIVEN a file of 2 MB exists in a repository
- WHEN the client requests it via `POST /api/v1/gogs/file`
- THEN the backend SHALL return `{"error": "file exceeds maximum size of 1 MB", "code": "FILE_TOO_LARGE"}`
- AND the HTTP status SHALL be 413

#### Scenario: File exactly at size limit

- GIVEN a file of exactly 1,048,576 bytes exists in a repository
- WHEN the client requests it via `POST /api/v1/gogs/file`
- THEN the backend SHALL return the file contents normally
- AND the HTTP status SHALL be 200

### Requirement: REQ-GP-006 — All gogs endpoints MUST be protected by JWT auth middleware

Both `GET /api/v1/gogs/repos` and `POST /api/v1/gogs/file` SHALL be registered under the existing `/api/v1` route group that applies `authmiddleware.AuthMiddleware`. Requests without a valid JWT SHALL be rejected before reaching the GogsHandler.

#### Scenario: Valid JWT access

- GIVEN a request with a valid Supabase JWT in the Authorization header
- WHEN the request targets `/api/v1/gogs/repos`
- THEN the auth middleware SHALL allow the request to proceed
- AND the GogsHandler SHALL process the request

#### Scenario: Missing JWT rejected

- GIVEN a request without an Authorization header
- WHEN the request targets `/api/v1/gogs/repos`
- THEN the auth middleware SHALL reject the request
- AND the HTTP status SHALL be 401
- AND the GogsHandler SHALL NOT be invoked

#### Scenario: Expired JWT rejected

- GIVEN a request with an expired Supabase JWT
- WHEN the request targets `/api/v1/gogs/file`
- THEN the auth middleware SHALL reject the request
- AND the HTTP status SHALL be 401
- AND the GogsHandler SHALL NOT be invoked
