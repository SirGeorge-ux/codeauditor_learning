# Specification: Initial Scaffolding

Establishes the foundational monorepo structure, core architecture, and security boundaries for CodeAuditor.

## Quick Path (Verification)

1. Check `backend/` and `frontend/` directories exist.
2. Verify Go and Angular applications build cleanly.
3. Validate Docker-compose orchestrates local dependencies (Supabase, Ollama).

## 1. Monorepo Foundation

### Purpose
Provide a unified repository with strict separation between the Go backend (Hexagonal Architecture) and Angular 21 frontend.

### Requirements

#### Requirement: Repository Structure
The system MUST provide isolated `backend` and `frontend` directories that build independently.

##### Scenario: Independent builds
- GIVEN the monorepo root
- WHEN the build command is executed for either backend or frontend
- THEN the application MUST build without cross-boundary errors

#### Requirement: Hexagonal Boundaries
The backend MUST isolate core domain logic from external adapters and frameworks.

##### Scenario: Domain purity
- GIVEN the backend Go codebase
- WHEN domain logic is analyzed
- THEN it MUST NOT import external frameworks (e.g., HTTP handlers, DB drivers)

## 2. Supabase Auth Foundation

### Purpose
Secure backend endpoints using Supabase JWT validation middleware.

### Requirements

#### Requirement: Token Validation
The system MUST validate requests using Supabase JWT tokens for all protected API routes.

##### Scenario: Valid authentication
- GIVEN a request with a valid Supabase JWT in the `Authorization` header
- WHEN the request reaches a protected endpoint
- THEN the middleware MUST allow the request and inject the user identity

##### Scenario: Invalid authentication
- GIVEN a request with a missing or invalid JWT
- WHEN the request reaches a protected endpoint
- THEN the middleware MUST reject the request with a 401 Unauthorized status

## 3. SSE Streaming Foundation

### Purpose
Connect the Go backend and Angular frontend for real-time LLM streaming using Server-Sent Events.

### Requirements

#### Requirement: Event Stream Endpoint
The backend MUST expose an endpoint returning `text/event-stream` for LLM responses.

##### Scenario: Streaming text tokens
- GIVEN an authenticated client connected to the SSE endpoint
- WHEN the backend receives tokens from the local Ollama instance
- THEN the backend MUST stream each token immediately to the client
- AND the connection MUST be closed safely upon completion or client disconnect

## 4. Docker Sandbox Execution

### Purpose
Safely execute untrusted user code using isolated, ephemeral Docker containers.

### Requirements

#### Requirement: Zero-Trust Containerization
The system MUST execute code in one-shot containers configured with maximum restriction.

##### Scenario: Untrusted code execution
- GIVEN a payload of untrusted user code
- WHEN the sandbox execution service runs the code
- THEN the Docker container MUST run with `no-new-privileges`, `cap-drop ALL`, `read-only rootfs`, and `--network none`
- AND the system MUST destroy the container after execution finishes or times out
