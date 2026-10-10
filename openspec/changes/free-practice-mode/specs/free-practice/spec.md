# free-practice Specification (delta for free-practice-mode)

> **Capability:** `free-practice` (NEW), `challenges` (MODIFIED)
> **Change:** `free-practice-mode` (S3)
> **Depends on:** `mcp-monolith` (S2, archived 2026-10-10), `tutor-chat` (S2, archived 2026-10-10), `challenge-rebuild` (S1, archived 2026-07-22), `language-progress` (S1, archived 2026-07-22)
> **This spec is a delta that, on archive, lands at:** `openspec/specs/free-practice/spec.md` (new) and `openspec/specs/challenges/spec.md` (modified)

## Purpose

The `free-practice` capability generates challenges on demand from a versioned curriculum and a user-selected F–S level, persists them with `origin="generated"`, and surfaces them in the existing dojo flow. The `challenges` capability is modified to expose a non-streaming JSON-mode `Complete` method on the LLM client port and to ship a real (un-stubbed) `ChallengeService.GenerateChallenge` orchestration that drives the S2 MCP tools (`mcp-curriculum`, `mcp-pedagogical`, `mcp-challenges`) and the S2 LLM cascade.

The change is additive: curated mode is unaffected. Rollback restores the `/practice/free` page STUB, removes the option-func dependencies from `ChallengeService`, and leaves the MCP tool registered (it returns a clean wiring error). No DB migration is required — generated rows reuse the existing `challenges` columns.

## ADDED Requirements

### Requirement: Practice Generation Flow

The system SHALL let a user generate a challenge on demand by selecting a language, a topic from that language's curriculum, and an F–S level, and SHALL return a v2 `Challenge` that opens in the existing dojo flow.

#### Scenario: Happy path generation

- GIVEN the user is on `/practice/free` and the language `typescript` has a curriculum at `.atl/curricula/typescript.md`
- WHEN the user selects topic `inheritance` at level `D`
- THEN the system returns a `Challenge` with `language=typescript`, `origin=generated`, `GeneratedBy=free-practice-v1`, non-empty `testCases`, `solutionCode`, and `expectedFindings`
- AND the dojo route opens with the generated challenge loaded

#### Scenario: Generated challenge re-opens in dojo

- GIVEN a challenge was generated and persisted in a previous session
- WHEN the user navigates to that challenge via the dojo listing
- THEN the dojo flow loads it identically to a curated challenge

#### Scenario: Topic not in curriculum

- GIVEN the user requests a `topicId` that does not exist in the curriculum for the selected language
- WHEN the system reads the curriculum
- THEN the system returns `ErrCurriculumTopicNotFound` and the UI surfaces a localized "Topic not available" error

#### Scenario: Unsupported language

- GIVEN the user requests a language with no curriculum file
- WHEN the system reads the curriculum base dir
- THEN the system returns `ErrCurriculumLanguageNotFound` and the UI disables generation for that language

### Requirement: Curriculum-Driven Topic Catalog

The system SHALL read topics exclusively from `.atl/curricula/<language>.md` files, where each file declares an ordered `## Topics` section with one `### <topic-slug>` entry per topic. Topics SHALL be addressable by slug, title, and F–S difficulty, and SHALL carry a short description.

#### Scenario: `mcp-curriculum.get_topics` returns the file's topics

- GIVEN `.atl/curricula/typescript.md` declares 12 topics under `## Topics`
- WHEN the frontend calls `mcp-curriculum.get_topics(typescript)`
- THEN the response is an array of 12 topic descriptors ordered as in the file

#### Scenario: `mcp-curriculum.get_topic` returns topic detail

- GIVEN the curriculum declares `### inheritance` with `F: junior` and a description
- WHEN the frontend calls `mcp-curriculum.get_topic(typescript, inheritance)`
- THEN the response includes `id=inheritance`, `title=inheritance`, `difficulty=F..S`, and the description

#### Scenario: Curricula base dir is overridable

- GIVEN the env var `CURRICULA_BASE_DIR=/opt/curricula` is set
- WHEN the system reads a curriculum
- THEN the read resolves under `/opt/curricula/<language>.md`, not the default `.atl/curricula`

#### Scenario: Path-traversal guard remains in effect

- GIVEN the curricula reader is asked for `language=../etc/passwd`
- WHEN the path is resolved against `CURRICULA_BASE_DIR`
- THEN the reader returns `ErrCurriculumInvalidLanguage` and does not read outside the base dir

### Requirement: F–S Level to Difficulty Mapping

The system SHALL map F–S level selections to the existing 4-bucket difficulty field as follows: `F,E → junior`, `D,C → mid`, `B,A → senior`, `S → architect`.

#### Scenario: Level F maps to junior

- GIVEN the user selects level `F`
- WHEN the system builds the `Challenge`
- THEN the persisted `Challenge.Difficulty = "junior"`

#### Scenario: Level S maps to architect

- GIVEN the user selects level `S`
- WHEN the system builds the `Challenge`
- THEN the persisted `Challenge.Difficulty = "architect"`

### Requirement: Per-User Daily Generation Cap

The system SHALL enforce a per-user soft cap of 20 generated challenges per UTC day, tracked client-side in `localStorage` and surfaced in the UI as remaining generations.

#### Scenario: Under-cap user can generate

- GIVEN a user has generated 5 challenges today
- WHEN the user requests another generation
- THEN the system proceeds and the remaining counter shows 15

#### Scenario: At-cap user is blocked

- GIVEN a user has generated 20 challenges today
- WHEN the user requests another generation
- THEN the use case rejects the request without calling the backend, and the UI shows a "Daily limit reached" notice

#### Scenario: Counter resets at UTC midnight

- GIVEN a user generated 20 challenges yesterday
- WHEN the user opens the page after UTC midnight
- THEN the counter shows 20 remaining

### Requirement: Generated Challenge Persistence

The system SHALL persist every generated challenge with `origin="generated"`, `GeneratedBy="free-practice-v1"`, and `CreatedBy=<authenticated user id>`. The persisted row SHALL be openable from the dojo like any curated challenge.

#### Scenario: Persistence fields are set

- GIVEN a user `u-1` requests a generated challenge
- WHEN the system writes the row
- THEN `origin=generated`, `GeneratedBy=free-practice-v1`, `CreatedBy=u-1` are non-empty and persisted

#### Scenario: Generated challenges list in the dojo

- GIVEN 3 challenges with `origin=generated` exist for the user
- WHEN the user navigates to `/dojo`
- THEN the dojo listing includes the 3 generated challenges alongside curated ones, tagged with their `language` and F–S level

### Requirement: Solution-Validates Sandbox Gate

The system SHALL run the generated `solutionCode` together with `testCases` in the sandbox up to 3 times. If a run fails, the system SHALL append the error to the LLM prompt and regenerate. If 3 consecutive runs fail, the system SHALL return `ErrChallengeGenerationFailedSandbox` and not persist the row.

#### Scenario: First-run solution passes

- GIVEN the LLM emits a challenge whose `solutionCode` passes all `testCases` on the first sandbox run
- WHEN the system validates
- THEN the system persists the challenge and returns it to the caller

#### Scenario: Solution fails twice, passes third

- GIVEN the first two sandbox runs fail and the third passes after the prompt has been augmented with prior errors
- WHEN the system validates
- THEN the system persists the challenge (the failure history is not persisted)

#### Scenario: Solution fails three times

- GIVEN three consecutive sandbox runs fail
- WHEN the system reaches the retry ceiling
- THEN the system returns `ErrChallengeGenerationFailedSandbox` and writes no row

### Requirement: LLM-Driven JSON-Mode Generation

The system SHALL generate the v2 `Challenge` JSON by calling a JSON-mode `Complete` method on the LLM client port. The request SHALL carry a system prompt, a topic+story+restrictions prompt, and a JSON schema matching the v2 `Challenge` shape. The system SHALL parse the response and validate the draft with `validateChallengeV2` (cardinality + spoilers) before persisting.

#### Scenario: LLM returns valid JSON

- GIVEN the LLM returns a JSON object matching the v2 `Challenge` schema
- WHEN the system parses and validates
- THEN the system accepts the draft and proceeds to the sandbox gate

#### Scenario: LLM returns invalid JSON

- GIVEN the LLM returns text that fails to parse as the v2 `Challenge` shape
- WHEN the system parses
- THEN the system returns `ErrChallengeGenerationInvalidJSON` and writes no row

#### Scenario: Draft fails `validateChallengeV2`

- GIVEN the parsed draft has cardinality violations (e.g. <2 `expectedFindings`) or a spoiler in the description
- WHEN the system validates
- THEN the system returns `ErrChallengeGenerationInvalid` and writes no row

## MODIFIED Requirements

### Requirement: LLMClientPort.Complete Method

> **Diff vs. canonical `openspec/specs/tutor/spec.md` (if any):** the LLM client port now also exposes a non-streaming JSON-mode `Complete` method. Streaming `StreamChat` is unchanged.

The `LLMClientPort` SHALL expose a `Complete(ctx, CompleteRequest) (CompleteResponse, error)` method that performs a single blocking structured completion, returning the raw content (JSON when a schema is supplied) plus the responding provider and latency. The method SHALL use a 5-second whole-response timeout, mirroring `DefaultFirstTokenTimeout`.

#### Scenario: JSON-mode complete succeeds

- GIVEN the cascade is configured with Groq as primary
- WHEN the caller invokes `Complete` with a `JSONSchema`
- THEN the response is non-empty, parseable as JSON, and `Provider=groq`

#### Scenario: Timeout fires

- GIVEN the cascade is unreachable for >5 seconds
- WHEN the caller invokes `Complete`
- THEN the call returns `ErrLLMCompleteTimeout` within 5 seconds

#### Scenario: Provider fallthrough

- GIVEN Groq returns 5xx and DeepSeek is configured next
- WHEN the caller invokes `Complete`
- THEN the cascade transparently retries against DeepSeek and the response's `Provider=deepseek`

### Requirement: ChallengeService.GenerateChallenge Is Wired

> **Diff vs. canonical `openspec/specs/challenges/spec.md`:** `GenerateChallenge` is no longer a stub returning `ErrChallengeGenerateNotImplemented`. It now orchestrates curriculum + pedagogical + LLM cascade + sandbox into a persisted v2 `Challenge`. The service is constructed with option funcs so the new dependencies are additive and nil-safe.

`ChallengeService.GenerateChallenge` SHALL produce a persisted v2 `Challenge` when its dependencies (`curricula`, `pedagogy`, `llm`, `sandbox`) are wired. When `llm` is nil, it SHALL return `ErrChallengeGenerationNotWired` and write no row.

#### Scenario: Wired service generates and persists

- GIVEN `ChallengeService` is constructed with all four dependencies
- WHEN `GenerateChallenge(ctx, userID, hint)` is called with a valid `hint` (carrying `TopicID` and `Level`)
- THEN the system reads the topic, fetches the learning objective, builds the story context, calls the LLM in JSON mode, validates, runs the sandbox gate, persists, and returns the `Challenge`

#### Scenario: Unwired service returns a clean error

- GIVEN `ChallengeService` is constructed without `llm`
- WHEN `GenerateChallenge` is called
- THEN it returns `ErrChallengeGenerationNotWired` and writes no row

#### Scenario: GenerateChallengeHint is extended

- GIVEN the canonical `GenerateChallengeHint` has only `Language`, `Difficulty`, `Focus`
- WHEN S3 lands
- THEN the type is extended with `TopicID string` and `Level string` (F..S); existing callers that omit them continue to compile and behave as before

### Requirement: CURRICULA_BASE_DIR Environment Override

> **Diff vs. canonical `openspec/specs/challenges/spec.md`:** the curricula base dir is now configurable.

`cmd/api/main.go` SHALL honor the `CURRICULA_BASE_DIR` environment variable when constructing the curricula service, defaulting to `.atl/curricula` when unset. The path-traversal guard at the segment level SHALL remain unchanged.

#### Scenario: Default base dir

- GIVEN `CURRICULA_BASE_DIR` is unset
- WHEN the API starts
- THEN `CurriculumService` is constructed with baseDir `.atl/curricula`

#### Scenario: Override applied

- GIVEN `CURRICULA_BASE_DIR=/opt/curricula`
- WHEN the API starts
- THEN `CurriculumService` is constructed with baseDir `/opt/curricula`
