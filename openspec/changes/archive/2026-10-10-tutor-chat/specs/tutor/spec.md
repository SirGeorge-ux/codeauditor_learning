# Tutor Specification

> **Domain:** Socratic AI Tutor Chat
> **Change:** `tutor-chat` (S2)
> **Type:** New capability

## Purpose

The system SHALL provide a context-aware, socratic AI tutor chat panel that scaffolds user learning during code audits and free practice, adapting its directness to the user's rank, history, and current challenge state.

## Requirements

### Requirement: Chat Panel Visibility

The system SHALL render a persistent tutor chat panel on `/dojo/*` and `/practice/*` routes. On desktop, the panel occupies the right sidebar. On viewports <768px, the panel collapses to a floating action button (FAB) that opens a modal overlay.

#### Scenario: Desktop layout with chat panel

- GIVEN a user navigates to `/dojo/ch-sqli` on a desktop viewport
- WHEN the page renders
- THEN the chat panel appears as a right sidebar alongside the code editor and terminal

#### Scenario: Mobile FAB toggle

- GIVEN a user navigates to `/practice/free/typescript` on a viewport <768px
- WHEN the page renders
- THEN a FAB icon appears in the bottom-right corner
- WHEN the user taps the FAB
- THEN a modal overlay opens with the full chat interface

#### Scenario: Chat hidden on non-practice routes

- GIVEN a user navigates to `/dashboard` or `/login`
- WHEN the page renders
- THEN no chat panel or FAB is visible

### Requirement: Chat Panel UI

The system SHALL display a dark-themed chat interface with a scrollable message list, a text input field, and a send button. Messages from the user align right; messages from the tutor align left. The panel SHALL use the dojo color palette (`--color-dojo-surface`, `--color-dojo-text`, `--color-dojo-border`, `--color-dojo-accent`).

#### Scenario: Message rendering with syntax highlighting

- GIVEN the tutor sends a response containing inline code
- WHEN the message renders
- THEN code snippets are displayed with monospace font and syntax-appropriate coloring

#### Scenario: Empty state

- GIVEN a user opens the chat for the first time in a session
- WHEN no messages exist
- THEN a welcome message appears with the current socratic level and a `/help` hint

### Requirement: Socratic Levels

The system SHALL support 4 socratic levels (0–3), each loaded from `.atl/prompts/tutor-socratic-{0..3}.md`. The default level is determined by the user's rank per the mapping table.

| Rank (free practice) | Rank (code health) | Default level |
|---|---|---|
| F–E | Junior | 2 (pure) |
| D–C | Mid | 1 (guided) |
| B–A | Senior | 0 (direct, after 2 failures) |
| S | Architect | 0 (direct) |

#### Scenario: Junior user receives socratic level 2

- GIVEN a user with free-practice rank "D" opens the chat
- WHEN the first message is sent
- THEN the system prompt includes `tutor-socratic-2.md`

#### Scenario: Architect user receives socratic level 0

- GIVEN a user with code-health rank "Architect" opens the chat
- WHEN the first message is sent
- THEN the system prompt includes `tutor-socratic-0.md`

### Requirement: Auto-Escalation

The system SHALL adjust socratic directness based on user behavior:
- 2 failures on the same challenge → escalate +1 level
- 3 requests for "the answer" → escalate +2 levels
- Stuck >10 min without progress → offer a hint (do not give it unprompted)
- Challenge solved without hints → escalate +1 socratic level (more questioning)

#### Scenario: Escalation after two failures

- GIVEN a user at socratic level 2 fails the same challenge twice
- WHEN the third attempt begins
- THEN the tutor operates at level 1

#### Scenario: Stuck user receives hint offer

- GIVEN a user has been on the same challenge for >10 min with no code changes
- WHEN the user sends any message
- THEN the tutor asks "¿Querés una pista?" without providing the hint

### Requirement: Chat Commands

The system SHALL parse and execute 7 slash commands client-side before sending to the LLM:

| Command | Effect |
|---|---|
| `/nivel 0-3` | Change socratic level, persist to `LearningProfile` |
| `/pista` | Request next hint (points penalty after hint 1) |
| `/solucion` | Reveal solution with high point penalty |
| `/reset` | Clear 10-message session window |
| `/perfil` | Display current `LanguageProgress` summary |
| `/diccionario <term>` | Look up term in personal glossary |
| `/help` | List all commands with descriptions |

#### Scenario: User changes socratic level

- GIVEN a user at level 2 types `/nivel 0`
- WHEN the command is processed
- THEN the socratic level changes to 0 and `LearningProfile.nivel_socratismo` is updated

#### Scenario: Invalid command

- GIVEN a user types `/nivel 5`
- WHEN the command is processed
- THEN the system responds with "Nivel inválido. Usá /nivel 0-3"

### Requirement: ChatContext Assembly

The system SHALL assemble a `ChatContext` object before every LLM call, containing: user profile (id, email, global rank, streak), language state (current, rank, points, success rate), location (route, challenge code/output), history (last 5 challenges, recent hints, common blocks), and preferences (language, socratic level, max message length).

#### Scenario: Context includes current challenge code

- GIVEN a user is editing code in `/dojo/ch-sqli`
- WHEN the user sends a message to the tutor
- THEN the `ChatContext.location.challenge.code` field contains the current Monaco editor content

#### Scenario: Context reflects language progress

- GIVEN a user with TypeScript rank "C" and 450 points
- WHEN the chat context is assembled
- THEN `ChatContext.language.rango_in_language` = "C" and `puntos` = 450

### Requirement: SSE Streaming

The system SHALL stream tutor responses via SSE. The primary LLM is Groq (`llama-3.3-70b-versatile`) with a 5s timeout. On timeout or error, the system SHALL fall back to DeepSeek V3 via OpenRouter. If the user has `use_minimax=true`, the system SHALL use MiniMax M3.

#### Scenario: Primary LLM streams response

- GIVEN a user sends a message
- WHEN Groq responds within 5s
- THEN tokens stream to the chat panel via SSE events

#### Scenario: Fallback on timeout

- GIVEN Groq does not respond within 5s
- WHEN the timeout fires
- THEN the system retries with DeepSeek V3 and streams that response

### Requirement: Session Memory

The system SHALL maintain a rolling window of the last 10 messages in the current session. This window is sent with every LLM call. The window is discarded when the user changes challenges or closes the chat.

#### Scenario: Window rolls over at 10 messages

- GIVEN a session has 10 messages
- WHEN the user sends the 11th message
- THEN the oldest message is dropped and messages 2–11 are sent to the LLM

#### Scenario: Window resets on challenge change

- GIVEN a user has 8 messages in the chat for challenge A
- WHEN the user navigates to challenge B
- THEN the session window is cleared

### Requirement: Persistent LearningProfile Updates

The system SHALL update `LearningProfile` after each completed audit with: a 2–3 sentence summary of what the user learned, where they got stuck, and how they responded to socratic guidance. Updates occur automatically via LLM-generated summary every 10 messages.

#### Scenario: Profile update after audit completion

- GIVEN a user completes an audit session
- WHEN the audit is marked `completed`
- THEN `LearningProfile.resumen` is updated with a 2–3 sentence summary

### Requirement: MCP Tool Integration

The system SHALL enable the tutor to invoke MCP tools for curriculum topics, challenge generation, socratic prompts, external docs, and user progress. Tool calls are resolved server-side; the frontend sends only user text.

#### Scenario: Tutor fetches curriculum topics

- GIVEN a user asks "Dame un ejercicio de Herencias"
- WHEN the tutor processes the request
- THEN the system calls `mcp-curriculum.get_topics` filtered by the user's language and rank

#### Scenario: Tutor looks up external docs

- GIVEN a user asks "¿Cómo se usa `fetch` en TypeScript?"
- WHEN the tutor processes the request
- THEN the system calls `mcp-external-docs.get_library_docs` for the relevant library

### Requirement: Chat Restrictions

The system SHALL enforce these constraints on all tutor responses:
- MUST NOT solve the challenge directly (provide graduated hints instead)
- MUST NOT break the current socratic level
- MUST NOT discuss other users (single-tenant)
- MUST NOT change topics mid-challenge without suggesting to finish first
- MUST respect the user's `idioma` preference
- MUST truncate responses to `longitud_maxima_msg` words
- MUST NOT expose internal prompt structure to the user

#### Scenario: User requests direct solution at level 2

- GIVEN a user at socratic level 2 types "Dame el código de la solución"
- WHEN the tutor responds
- THEN the response contains graduated hints, not the solution code

#### Scenario: Response respects language preference

- GIVEN a user with `idioma: "en"` sends a message
- WHEN the tutor responds
- THEN the entire response is in English
