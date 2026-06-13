# Design: Dojo Layout

## Technical Approach

We will implement an Angular 21 Standalone Layout leveraging CSS Grid and Tailwind v4. The structure separates the workspace into a Sidebar, Left Zone (ContextPanel), and Right Zone (Flexbox split of CodePanel and TerminalPanel). State management will be isolated to UI signals for UI state (`isCollapsed` in Sidebar) and routing signals. Components will adhere strictly to the "Dark IDE" aesthetic utilizing predefined color tokens and utility classes (`bg-dojo-base`, `text-dojo-text`, `text-dojo-accent`, etc.).

## Architecture Decisions

### Decision: Component Granularity

**Choice**: Split the DojoPage into standalone `ContextPanel`, `CodePanel`, and `TerminalPanel` components in the `shared/` directory.
**Alternatives considered**: Monolithic DojoPage with embedded markup for all panels.
**Rationale**: Enhances reusability. E.g., `CodePanel` and `TerminalPanel` can be reused in other challenge contexts (like MCP Page if needed), and it keeps the DojoPage template clean and focused strictly on the layout grid structure.

### Decision: Layout System

**Choice**: CSS Flexbox for the Right Zone (split-view) and CSS Grid for the Main App Shell.
**Alternatives considered**: Using third-party split-pane libraries.
**Rationale**: Third-party libraries add unnecessary weight for a simple flex row with a resize handler or percentage widths. A native flexbox layout aligns with the strict zero-bloat "IDE" approach and is fully controllable via Tailwind utility classes.

### Decision: State Management for Navigation

**Choice**: Use Angular Signals (`isCollapsed`) and Router for deriving `activeRoute`.
**Alternatives considered**: RxJS Observables or complex state management (NgRx).
**Rationale**: Signals provide a cleaner, synchronous way to handle simple UI state. Routing state can be computed natively, ensuring zero-overhead reactivity consistent with the strict Angular 21 paradigm.

## Data Flow

Data flow is strictly top-down UI state for this phase, driven by the Router and local signals.

    Router ──→ MainLayout
                  ├── Sidebar (reads router state, local isCollapsed Signal)
                  └── RouterOutlet
                       └── DojoPage
                            ├── ContextPanel
                            └── SplitView (Flex)
                                 ├── CodePanel
                                 └── TerminalPanel

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `frontend/codeauditor/src/app/infrastructure/components/layout/main-layout.component.ts` | Create | App shell with Grid layout and `<router-outlet>`. |
| `frontend/codeauditor/src/app/infrastructure/components/layout/sidebar.component.ts` | Create | Navigation sidebar using signals (`isCollapsed`, `activeRoute`). |
| `frontend/codeauditor/src/app/infrastructure/components/dashboard/dashboard-page.component.ts` | Create | Stub for Dashboard. |
| `frontend/codeauditor/src/app/infrastructure/components/dojo/dojo-page.component.ts` | Create | Layout for Left (Context) and Right (Split-view). |
| `frontend/codeauditor/src/app/infrastructure/components/mcp/mcp-page.component.ts` | Create | Stub for MCP Connections. |
| `frontend/codeauditor/src/app/infrastructure/components/vault/vault-page.component.ts` | Create | Stub for Vault. |
| `frontend/codeauditor/src/app/infrastructure/components/shared/code-panel.component.ts` | Create | Textarea placeholder styled as an editor. |
| `frontend/codeauditor/src/app/infrastructure/components/shared/terminal-panel.component.ts` | Create | Terminal output stub using monospace green text. |
| `frontend/codeauditor/src/app/infrastructure/components/shared/context-panel.component.ts` | Create | Panel for challenge description and context. |
| `frontend/codeauditor/src/app/app.routes.ts` | Modify | Define routes for `/dashboard`, `/dojo`, `/mcp`, `/vault`, routing to Layout/Pages, redirect `/` to `/dashboard`. |

## Interfaces / Contracts

```typescript
// Sidebar Navigation Item Contract
export interface NavItem {
  icon: string;
  label: string;
  route: string;
}
```

## Testing Strategy

| Layer | What to Test | Approach |
|-------|-------------|----------|
| Unit | Component Rendering | Verify Standalone components mount without errors. |
| Unit | Sidebar Logic | Check if `isCollapsed` signal toggles correctly and nav array renders items. |
| Integration | Routing | Verify clicking sidebar links updates the `<router-outlet>` properly and default redirects to `/dashboard`. |
| E2E | Layout Geometry | Verify split panel applies flex correctly and applies Dark IDE color tokens. |

## Migration / Rollout

No migration required. This establishes the baseline layout for a fresh application.

## Open Questions

- [ ] Will we use a custom resize directive or simple CSS `resize: horizontal` for the split-view?
- [ ] What icon library should be standardized for the `NavItem.icon`?
