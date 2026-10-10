# Proposal: Socratic AI Tutor Chat

## Intent

**What:** Context-aware socratic tutor chat panel always visible in `/dojo/*` and `/practice/*`, with 4 pedagogical levels, 7 commands, session memory, and persistent `LearningProfile`.

**Why:** Users practice alone with no guided help. A socratic tutor scaffolds discovery instead of giving answers, improving retention by adapting to the user’s level and current challenge.

## Scope

### In Scope
- Chat panel UI (right panel desktop, FAB modal mobile)
- 4 socratic levels (0-3) with auto-escalation
- `ChatContext` assembly and SSE streaming
- 7 commands: `/nivel`, `/pista`, `/solucion`, `/reset`, `/perfil`, `/diccionario`, `/help`
- Short-term (10-msg) and long-term (`LearningProfile`) memory
- MCP tool integration for all commands

### Out of Scope
- Voice input, in-chat code execution, multiplayer tutoring
- Full LLM cascade rebuild (S5)
- `LearningProfile` schema itself (delivered by `language-progress` S1)

## Capabilities

### New Capabilities
- `tutor-chat`: Socratic chat with context awareness, memory, and MCP-driven commands

### Modified Capabilities
- `user`: add `update_learning_profile` MCP tool
- `mcp-pedagogical`: add `get_socratic_prompt` tool
- `mcp-user`: add `get_learning_profile` and `update_learning_profile` tools

## Approach

Frontend: standalone `tutor-chat` component + `TutorChatUseCase` (signals). Commands parsed client-side; text sent via SSE. Backend: `TutorService` in `core/services/` assembles `ChatContext`, loads socratic prompt via `mcp-pedagogical.get_socratic_prompt`, calls `LLMClient` (Groq primary, DeepSeek V3 fallback). Post-LLM validation enforces the socratic level. Auto-escalation lives in `TutorService`.

## Affected Areas

| Area | Impact | Description |
|---|---|---|
| `frontend/.../tutor-chat/` | New | Panel, use case, adapter |
| `frontend/.../dojo/` | Modified | Add panel to layout |
| `frontend/.../practice/` | Modified | Add panel to layout |
| `backend/.../core/services/` | New | `TutorService`, context builder |
| `backend/.../mcp/pedagogical/` | New | `get_socratic_prompt` tool |
| `backend/.../mcp/user/` | Modified | `get_learning_profile`, `update_learning_profile` |
| `.atl/prompts/` | New | 4 socratic system prompts |

## Risks

| Risk | Likelihood | Mitigation |
|---|---|---|
| LLM latency >3s | Med | Cascade fallback; 5s timeout |
| Response breaks socratic level | Med | Post-LLM validation filter |
| Prompt injection via code context | Low | Sanitize `ChatContext.code` |

## Rollback Plan

1. Disable via `TUTOR_CHAT_ENABLED=false`.
2. Revert `/dojo/` and `/practice/` layout commits.
3. No DB schema to revert (`LearningProfile` from S1).

## Dependencies

- `language-progress` S1: `LearningProfile` schema and data
- `mcp-monolith` S2: `mcp-pedagogical` and `mcp-user` tools

## Success Criteria

- [ ] Chat visible in `/dojo/*` and `/practice/*`
- [ ] 4 socratic levels respected in >95% of sampled responses
- [ ] 7 commands functional end-to-end with MCP tools
- [ ] `LearningProfile` updates after each completed audit
- [ ] Latency p95 <3s for first token
