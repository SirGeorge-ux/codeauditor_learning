# Tasks: real-challenges

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 700–800 |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | PR 1 → PR 2 → PR 3 |
| Delivery strategy | ask-on-risk |
| Chain strategy | pending |

Decision needed before apply: Yes
Chained PRs recommended: Yes
Chain strategy: pending
400-line budget risk: High

### Suggested Work Units

| Unit | Goal | Likely PR | Notes |
|------|------|-----------|-------|
| 1 | Domain & Application layer — Challenge model, port, use case, barrel exports | PR 1 | Base = main; self-contained |
| 2 | Infrastructure mock data — 8 real challenges, challenge service | PR 2 | Base = PR 1; depends on domain types |
| 3 | Dashboard, Dojo, Context panel, routes wiring | PR 3 | Base = PR 2; UI integration |

## Phase 1: Domain & Application Layer

- [x] 1.1 Create `frontend/codeauditor/src/app/domain/models/challenge.ts` — Define `Challenge` interface, `ChallengeDifficulty` enum (`junior | mid | senior`), `ChallengeStatus` enum (`available | locked | completed`), `ChallengeCategory` type (`security | async | angular | logic | error-handling | readability | design`)
- [x] 1.2 Create `frontend/codeauditor/src/app/domain/ports/challenge-repository.port.ts` — Define `ChallengeRepository` interface with `getAllChallenges(): Promise<Challenge[]>`, `getChallengeById(id: string): Promise<Challenge | null>`
- [x] 1.3 Create `frontend/codeauditor/src/app/application/challenge.use-case.ts` — Define `ChallengeUseCase` class with `getAllChallenges()` and `getChallenge(id)` methods delegating to `ChallengeRepository`
- [x] 1.4 Update `frontend/codeauditor/src/app/domain/models/index.ts` — Add `export * from "./challenge"`
- [x] 1.5 Update `frontend/codeauditor/src/app/domain/ports/index.ts` — Add `export * from "./challenge-repository.port"`
- [x] 1.6 Update `frontend/codeauditor/src/app/application/index.ts` — Add `export * from "./challenge.use-case"`

## Phase 2: Infrastructure (Mock Data)

- [x] 2.1 Create `frontend/codeauditor/src/app/infrastructure/repositories/mock-challenge.repository.ts` — Implement `ChallengeRepository` with ALL 8 challenges:
  1. SQL Injection en Login — Login form with string concatenation in query
  2. XSS en Comentarios — Unescaped user input in comment rendering
  3. Diosidad/Cyclomatic Complexity — Deeply nested if/else business logic
  4. Callback Hell / Promesas Anidadas — 4+ levels of nested `.then()` calls
  5. Mutación de Props en Componentes — Child component modifying parent state via `@Input() setter`
  6. Código Muerto / Condiciones Redundantes — `if (x === true) { if (x) {...} }` patterns
  7. Falta de Manejo de Errores — Async function with no try/catch, unhandled promise rejection
  8. Variables con Naming Opaco — `const x = ...; const z = ...;` with no context

- [x] 2.2 Create `frontend/codeauditor/src/app/infrastructure/services/challenge.service.ts` — Angular `@Injectable` service using signals: `challengesSignal`, `selectedChallengeSignal`, `loadChallenges()`, `selectChallenge(id)`, `getChallenge(id)`

## Phase 3: Dashboard Update

- [x] 3.1 Update `frontend/codeauditor/src/app/infrastructure/components/dashboard/dashboard-page.component.ts` — Replace placeholder cards with real challenge cards from `ChallengeService`. Show title, difficulty badge (color: junior=green, mid=yellow, senior=red), category tag, language badge. Add `isLoading` state and `empty state` message. Click navigates to `/dojo/:id`.

## Phase 4: Dojo & Context Update

- [x] 4.1 Update `frontend/codeauditor/src/app/infrastructure/components/shared/context-panel.component.ts` — Add `@Input() challenge: Challenge | null`. Render title, description, repository origin, code smell name + difficulty badge. Show placeholder when `challenge` is null.
- [x] 4.2 Update `frontend/codeauditor/src/app/infrastructure/components/dojo/dojo-page.component.ts` — Inject `ChallengeService`, read `:id` from `ActivatedRoute`, load challenge on init, pass to `<app-context-panel [challenge]="challenge">`
- [x] 4.3 Update `frontend/codeauditor/src/app/app.routes.ts` — Add `{ path: 'dojo/:id', component: DojoPageComponent }` route. Keep existing `/dojo` route for empty state.

## Phase 5: Verification

- [ ] 5.1 Run `cd frontend/codeauditor && npx ng build` — Fix any TypeScript errors
- [ ] 5.2 Verify `/dashboard` renders challenge cards
- [ ] 5.3 Verify clicking a card navigates to `/dojo/:id` and loads challenge context
- [ ] 5.4 Verify empty state on `/dojo` (no id) shows placeholder