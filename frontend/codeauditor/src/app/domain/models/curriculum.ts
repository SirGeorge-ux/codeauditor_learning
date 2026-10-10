// curriculum.ts — domain types for the Free Practice Mode feature.
//
// Pure TypeScript with ZERO Angular/RxJS/infrastructure imports. The
// shapes mirror the wire contract used between the Angular frontend and
// the Go MCP monolith:
//   - CurriculumTopic: the per-language topic descriptor returned by
//     `mcp-curriculum.get_topics` (see design.md §Data Shapes).
//   - GenerateChallengeArgs: the `arguments` payload for
//     `mcp-challenges.generate_challenge` (same design.md section).
//
// Tier validation is provided as `isF_SLevel` so the application layer can
// guard user input before it ever reaches the backend. The list
// `F_S_LEVELS` is canonical — anything that maps to the 4-bucket
// difficulty (`junior|mid|senior|architect`) at persistence time is
// computed in the backend (see `difficultyForLevel` in
// backend/internal/core/services/challenge_service.go).

// CurriculumTopic is the row shape returned by mcp-curriculum.get_topics.
// The Go curriculum reader (`backend/internal/core/services/curriculum_service.go`)
// already emits exactly these fields — keep this surface in lockstep.
export interface CurriculumTopic {
  // topic slug (e.g. "inheritance"); also the `topicId` for
  // generate_challenge args.
  id: string;
  // human-readable title (matches the slug by convention; not always
  // identical for languages with long names).
  title: string;
  // F..S letter band; the canonical free-practice range per
  // AGENTS.md §0.
  difficulty: string;
  // short blurb (1-3 lines) shown in the topic-list component.
  description: string;
}

// GenerateChallengeArgs is the payload sent to
// `mcp-challenges.generate_challenge` via MCPClientPort.callTool.
// The wire shape lives in openspec/changes/free-practice-mode/design.md
// §Data Shapes; the Go adapter maps it onto
// `GenerateChallengeHint{TopicID, Level}`.
export interface GenerateChallengeArgs {
  language: string;
  topicId: string;
  // F..S band; the backend maps F,E→junior / D,C→mid / B,A→senior /
  // S→architect at persistence time (see difficultyForLevel helper).
  level: string;
}

// Canonical F..S range, ascending from "foundations" to "sage". Mirrors
// the band documented in docs/curricula-format.md §3 and AGENTS.md §0.
// Order matters: the level-selector component renders them in this
// order so the user sees the ladder left-to-right / top-to-bottom.
export const F_S_LEVELS = ['F', 'E', 'D', 'C', 'B', 'A', 'S'] as const;

// Type-narrowing predicate for the F..S band. Use it anywhere a string
// comes from an untrusted source (URL, query param, generated UI state)
// before it crosses the wire.
export type F_SLevel = (typeof F_S_LEVELS)[number];

export function isF_SLevel(value: unknown): value is F_SLevel {
  return typeof value === 'string' && (F_S_LEVELS as readonly string[]).includes(value);
}
