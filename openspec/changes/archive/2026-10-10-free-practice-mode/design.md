# Design: Free Practice Mode (S3)

## Technical Approach

S3 ships on-demand challenge generation: user picks language → topic → F–S level, the backend orchestrates `mcp-curriculum` + `mcp-pedagogical` + the LLM cascade to emit a v2 `Challenge`, validates it, persists it (`origin="generated"`), and returns it to a rebuilt `/practice/free` page. The two hard blockers resolved here are (1) un-stubbing `ChallengeService.GenerateChallenge` and (2) adding a structured-JSON `Complete` method to `LLMClientPort`, which today is chat-only (`StreamChat`). The cascade (`Groq → DeepSeek → MiniMax`) is reused unchanged in ranking; S3 adds only the non-streaming JSON-mode path. S5 later generalizes the cascade; S3 keeps it minimal.

## Architecture Decisions

| Option | Tradeoff | Decision |
|---|---|---|
| Extend `GenerateChallengeHint` vs new struct | New struct churns all callers for no benefit | Extend `GenerateChallengeHint` with `TopicID`, `Level` |
| `Complete` vs overload `StreamChat` with a JSON flag | `StreamChat` is streaming + event-callback; challenge gen is a single blocking call | New `Complete(ctx, CompleteRequest) (CompleteResponse, error)` |
| `NewChallengeService` signature change vs option funcs | Signature change breaks `main.go`, `wiring_test.go`, HTTP wiring | Option funcs: `WithCurriculumService`, `WithPedagogicalService`, `WithLLMClient`, `WithSandboxExecutor` (additive, nil-safe) |
| `GenerateChallenge` reuse `Create()` dedup vs direct `Insert` | `Create` dedups on `source_repo` (empty for generated) → false dedup hits | Direct `repo.Insert` after `validateChallengeV2`; no dedup (daily cap + client hash guard duplicates) |
| Sandbox validate via `PedagogicalService.ValidateSolution` vs `SandboxExecutor` | `ValidateSolution` is a static header/length check (PR 4), never runs code | Use `ports.SandboxExecutor` directly for the solution-runs-tests gate; `ValidateSolution` stays the static pre-check |
| `CURRICULA_BASE_DIR` env vs hardcode | Hardcode is fragile across Coolify/compose cwd | Env `CURRICULA_BASE_DIR`, default `.atl/curricula` (path-traversal guard unchanged at segment level) |
| `Complete` timeout | Streaming 5s first-token is meaningless for sync call | 5s whole-response timeout, mirroring `DefaultFirstTokenTimeout` |

## Data Flow

```
PracticeUseCase.generateChallenge()
 └─ mcp.callTool('mcp-challenges.generate_challenge', {language, topicId, level})
    └─ generateChallengeTool.Execute (mcp/challenges/challenges_tool.go)
       └─ serviceAdapter.GenerateChallenge (mcp/challenges/adapter.go)
          └─ ChallengeService.GenerateChallenge (core/services/challenge_service.go)
             ├─ curricula.GetTopic(lang, topicID)              → TopicDetail
             ├─ pedagogy.GetLearningObjective(lang, topic, lvl) → LearningObjective
             ├─ pedagogy.GenerateStoryContext(lang, topic)      → StoryContext
             ├─ build System+Prompt (identity, level, topic, story+restrictions, schema)
             ├─ llm.Complete(ctx, CompleteRequest{…})           → raw JSON
             ├─ parse JSON → challengeDraft (v2)
             ├─ loop ≤3: sandbox.Execute(lang, solution+tests) — on fail, append error → regenerate
             ├─ validateChallengeV2 (cardinality + spoilers)
             ├─ map F–S level → difficulty (junior..architect)
             ├─ build Challenge{Origin:"generated", GeneratedBy:"free-practice-v1", CreatedBy:userID}
             ├─ repo.Insert(ctx, &challenge)
             └─ return Challenge
```

## File Changes

| File | Action | Description |
|---|---|---|
| `backend/internal/ports/tutor.go` | Modify | Add `Complete`, `CompleteRequest`, `CompleteResponse`, `JSONSchema` to `LLMClientPort` |
| `backend/internal/core/domain/models/challenge_models.go` | Modify | Extend `GenerateChallengeHint` with `TopicID`, `Level` |
| `backend/internal/core/services/challenge_service.go` | Modify | Un-stub `GenerateChallenge`; add deps + option funcs + `ErrChallengeGenerationNotWired` |
| `backend/internal/infrastructure/driven/llm_cascade/{cascade,groq,deepseek,minimax}.go` | Modify | Add `Complete` to `providerClient` + each provider (JSON-mode request) |
| `backend/internal/infrastructure/mcp/challenges/{challenges_tool.go,adapter.go}` | Modify | Extend `GenerateHint` wire shape with `topicId`, `level`; map into domain hint |
| `backend/cmd/api/main.go` | Modify | `CURRICULA_BASE_DIR` env override; wire new deps into `ChallengeService` |
| `.atl/curricula/typescript.md` | Create | 10–15 TypeScript topics (canonical format) |
| `frontend/.../domain/models/curriculum.ts` | Create | `CurriculumTopic`, `GenerateChallengeArgs` types |
| `frontend/.../application/practice.use-case.ts` | Create | Signals + command flow + daily cap |
| `frontend/.../components/practice/{language-selector,level-selector,topic-list,generated-challenge}.component.ts` | Create | 4 dumb standalone components |
| `frontend/.../components/practice/practice-page.component.ts` | Modify | Replace STUB with real selector + generated-challenge layout |

## Interfaces / Contracts

`LLMClientPort` (ports/tutor.go) gains:

```go
type JSONSchema struct {
    Type                 string         `json:"type"`
    Properties           map[string]any `json:"properties"`
    Required             []string       `json:"required"`
    AdditionalProperties bool           `json:"additionalProperties"`
}

type CompleteRequest struct {
    System            string      // identity + rules (system role)
    Prompt            string      // topic + story + restrictions + schema instruction
    JSONSchema        *JSONSchema // nil = free text
    PreferredProvider string      // "groq"|"deepseek"|"minimax"|""
}

type CompleteResponse struct {
    Content  string        // raw JSON when JSONSchema set
    Provider string
    Latency  time.Duration
}

// added method:
Complete(ctx context.Context, req CompleteRequest) (CompleteResponse, error)
```

`GenerateChallengeHint` (domain) extended:

```go
type GenerateChallengeHint struct {
    Language   string `json:"language,omitempty"`
    Difficulty string `json:"difficulty,omitempty"` // junior|mid|senior|architect
    Focus      string `json:"focus,omitempty"`
    TopicID    string `json:"topicId,omitempty"` // "inheritance"
    Level      string `json:"level,omitempty"`   // F..S
}
```

`ChallengeService` wiring (option funcs, nil-safe):

```go
type ChallengeService struct {
    repo      ports.ChallengeRepository
    curricula *CurriculumService
    pedagogy  *PedagogicalService
    llm       ports.LLMClientPort
    sandbox   ports.SandboxExecutor
}
func NewChallengeService(repo ports.ChallengeRepository, opts ...ChallengeServiceOption) *ChallengeService
func WithCurriculumService(*CurriculumService) ChallengeServiceOption
func WithPedagogicalService(*PedagogicalService) ChallengeServiceOption
func WithLLMClient(ports.LLMClientPort) ChallengeServiceOption
func WithSandboxExecutor(ports.SandboxExecutor) ChallengeServiceOption
```

`GenerateChallenge` returns `ErrChallengeGenerationNotWired` when `llm` is nil. F–S→difficulty map: `F,E→junior`, `D→mid`, `C→mid`, `B→senior`, `A→senior`, `S→architect` (helper `difficultyForLevel`).

## Data Shapes

**LLM prompt (assembled into `System` + `Prompt`):** `System` = "You are a senior code auditor generating a runnable challenge." + no-spoiler rule. `Prompt` = `Language: {lang}\nTopic: {slug}\nLevel: {level}\nStory: {story}\nLearning objective: {objective}\nRestrictions: {…}\nCommon mistakes: {…}\n\nReturn JSON matching the schema.` 5 logical parts folded into 2 strings (identity + 4 context parts), not 5 messages.

**v2 `Challenge` JSON schema (model output):**

```json
{"type":"object","properties":{
  "title":{"type":"string"},"description":{"type":"string"},
  "language":{"type":"string"},"code":{"type":"string"},
  "learningObjectives":{"type":"array","items":{"type":"string"}},
  "hints":{"type":"array","items":{"type":"object","properties":{
     "level":{"type":"integer"},"content":{"type":"string"},"costPoints":{"type":"integer"}}}},
  "commonMistakes":{"type":"array","items":{"type":"string"}},
  "expectedFindings":{"type":"array","items":{"type":"object","properties":{
     "severity":{"type":"string"},"category":{"type":"string"},"message":{"type":"string"},
     "evidence":{"type":"string"},"suggestedFix":{"type":"string"}}}},
  "testCases":{"type":"array","items":{"type":"object","properties":{
     "name":{"type":"string"},"input":{},"expectedOutput":{},"weight":{"type":"integer"}}}},
  "linterRules":{"type":"array","items":{"type":"object"}},
  "solutionCode":{"type":"string"},"solutionExplanation":{"type":"string"}
},"required":["title","description","language","code","hints","expectedFindings","testCases","solutionCode"],"additionalProperties":false}
```

**`MCPClientPort.callTool` request:**

```json
{"name":"mcp-challenges.generate_challenge","args":{"language":"typescript","topicId":"inheritance","level":"D"}}
```

**`mcp-curriculum.get_topics` response:** `[{ "id":"variables","title":"variables","difficulty":"F","description":"…" }, …]` (maps to `CurriculumTopic`).

## Testing Strategy

| Layer | What | Approach |
|---|---|---|
| Go service | `challenge_service_generate_test.go` | Table-driven: fake curriculum/pedagogy/LLM/sandbox; assert prompt assembly, level mapping, retry loop, `origin/generatedBy/createdBy`, persist-via-Insert |
| Go ports | `ports/tutor_test.go` (add) | Compile-time: `Complete` present on `LLMClientPort` |
| Go cascade | `llm_cascade/complete_test.go` | Provider fallthrough + JSON-mode request body shape (Groq/OpenRouter `response_format`), MiniMax text fallback |
| Angular use case | `practice.use-case.spec.ts` | Signal transitions, command flow, daily-cap enforcement (≥20 rejected), localStorage persistence |
| Angular components | 4 new specs | Toggle/render + `input.required` + `output` emit behaviors |

## Threat Matrix

N/A — S3 introduces no routing, shell, subprocess, VCS/PR-automation, or executable-file-classification boundary. Two pre-existing boundaries are consumed unchanged: (1) sandbox code execution via `ports.SandboxExecutor` with existing `--cap-drop=ALL --network=none --read-only` flags (AGENTS.md §7); (2) curricula filesystem read via `CurriculumService` path-traversal guard (`ErrCurriculumInvalidLanguage`). `CURRICULA_BASE_DIR` changes only the baseDir value, not the guarded segment logic.

## Migration / Rollout

Additive. Rollback: restore `practice-page.component.ts` STUB from git, delete `practice.use-case.ts` + 4 components, delete `.atl/curricula/typescript.md` (nothing else reads it), revert `challenge_service.go` un-stub (tool stays registered, returns clean error). No DB migration — generated rows use existing `challenges` columns.

## Work-unit breakdown (preview for sdd-tasks)

1. Backend: `Complete` on `LLMClientPort` + cascade adapter (Groq/DeepSeek/MiniMax)
2. Backend: `ChallengeService.GenerateChallenge` un-stub + orchestration loop + option funcs
3. Content: `.atl/curricula/typescript.md` seed (10–15 topics, F–S ordered)
4. Backend: `CURRICULA_BASE_DIR` env override + wire new deps in `main.go`
5. Frontend: `domain/models/curriculum.ts` + `practice.ts` types
6. Frontend: `PracticeUseCase` signals + command flow
7. Frontend: 4 dumb components (language/level/topic/result)
8. Frontend: replace `practice-page.component.ts` STUB
9. Frontend: daily count via localStorage + 20/day cap
10. Verify: `make validate` equivalent (go test + pnpm test + lint)

## Risks (design-level mitigation)

- **JSON-mode reliability** → `Complete` uses `response_format` only on Groq/OpenRouter; MiniMax falls back to text + JSON parser; retry ≤2.
- **Broken generated code** → validate loop (≤3) runs `solution_code` in sandbox; discard on 3rd failure.
- **Daily cap is client-side** → soft cost guard, not security; server-side cap is S5.
- **LLM cost** → 20/day cap bounds Groq free-tier usage.
- **Seed quality** → ties to `add-curriculum-topic.harness.md`; content is a `sdd-tasks` deliverable, follow-up if weak.

## Open Questions

- [ ] `mcp-pedagogical.validate_solution` is static (no sandbox) — confirm S3 runs the sandbox gate inside `GenerateChallenge` (recommended) rather than extending `PedagogicalService.ValidateSolution`.
- [ ] Canonical route: `/practice/free` (proposal) vs `/practice` (current stub) — resolve in tasks.
