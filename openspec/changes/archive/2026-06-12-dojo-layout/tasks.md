# Tasks: Dojo Layout

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines | 700–900 |
| 400-line budget risk | High |
| Chained PRs recommended | Yes |
| Suggested split | PR 1 → PR 2 → PR 3 |
| Delivery strategy | ask-on-risk |
| Chain strategy | stacked-to-main |

Decision needed before apply: Yes
Chained PRs recommended: Yes
Chain strategy: stacked-to-main
400-line budget risk: High

### Suggested Work Units

| Unit | Goal | Likely PR | Notes |
|------|------|-----------|-------|
| 1 | Phase 1–2: Dependencies + Shared Components | PR 1 | ResizeDirective, ContextPanel, CodePanel, TerminalPanel |
| 2 | Phase 3–4: Sidebar + Layout Shell + Routes | PR 2 | Sidebar, MainLayout, all page stubs, routes |
| 3 | Phase 5: Verification + Build | PR 3 | Angular build verification |

## Phase 1: Dependencies

- [x] 1.1 Install `@lucide/angular` in frontend: `pnpm remove lucide-angular && pnpm add @lucide/angular`

## Phase 2: Shared Components

- [x] 2.1 Create `ResizeDirective` at `frontend/codeauditor/src/app/infrastructure/components/shared/resize.directive.ts`:
  - @Directive selector `[appResize]`
  - @Input() minWidth: number = 200, maxWidth: number = 800
  - @Input() initialWidth: number = 400
  - @Output() widthChange: Signal<number> via output function
  - HostListener('mousedown') on separator div, stores startX
  - HostListener('document:mousemove') calculates delta, applies min/max, emits via signal
  - HostListener('document:mouseup') removes listeners

- [x] 2.2 Create `ContextPanelComponent` at `frontend/codeauditor/src/app/infrastructure/components/shared/context-panel.component.ts`:
  - Standalone, imports CommonModule
  - Dark-themed panel: bg-dojo-surface, rounded-sm
  - Shows challenge description stub, repo origin, code smell info
  - Uses Inter font for body text

- [x] 2.3 Create `CodePanelComponent` at `frontend/codeauditor/src/app/infrastructure/components/shared/code-panel.component.ts`:
  - Standalone, imports CommonModule
  - Textarea styled as code editor: bg-dojo-surface, monospace font, rounded-sm
  - Placeholder text: "// Your code here"

- [x] 2.4 Create `TerminalPanelComponent` at `frontend/codeauditor/src/app/infrastructure/components/shared/terminal-panel.component.ts`:
  - Standalone, imports CommonModule
  - Black panel: bg-black, text-terminal-green (monospace green text)
  - Command prompt stub: `> _`
  - Scrolling for output

## Phase 3: Sidebar

- [x] 3.1 Create `SidebarComponent` at `frontend/codeauditor/src/app/infrastructure/components/layout/sidebar.component.ts`:
  - Standalone, imports RouterModule, CommonModule
  - Nav items typed array: `{ icon: LucideIcon, label: string, route: string }`
  - Icons: LayoutDashboard (Dashboard), Binary (Dojo), Server (MCP), Shield (Vault)
  - `isCollapsed` signal (default true), toggle button
  - `activeRoute` computed from Router.url
  - Items highlighted with text-dojo-accent when route matches
  - Lucide icons via @lucide/angular

## Phase 4: Layout Shell

- [x] 4.1 Create `MainLayoutComponent` at `frontend/codeauditor/src/app/infrastructure/components/layout/main-layout.component.ts`:
  - Standalone, imports RouterOutlet
  - Flex container: Sidebar + router-outlet, full viewport height (h-screen)
  - Sidebar fixed width, content area fills remaining space
  - Dark IDE theme: bg-dojo-base

- [x] 4.2 Update `frontend/codeauditor/src/app/app.routes.ts`:
  - Add routes: `/dashboard` → DashboardPageComponent, `/dojo` → DojoPageComponent, `/mcp` → McpPageComponent, `/vault` → VaultPageComponent
  - Wrap protected routes in MainLayoutComponent
  - Redirect `/` to `/dashboard`

- [x] 4.3 Create `DashboardPageComponent` at `frontend/codeauditor/src/app/infrastructure/components/dashboard/dashboard-page.component.ts`:
  - Standalone, protected (redirect to login if not authenticated)
  - Welcome content with user email from AuthService
  - Dark theme: bg-dojo-base

- [x] 4.4 Create `DojoPageComponent` at `frontend/codeauditor/src/app/infrastructure/components/dojo/dojo-page.component.ts`:
  - Standalone, protected
  - Flex row: ContextPanel (left, ~300px) + SplitView (right) using ResizeDirective
  - SplitView: CodePanel (top) + TerminalPanel (bottom), vertical split
  - Dark theme: bg-dojo-base

- [x] 4.5 Create `McpPageComponent` at `frontend/codeauditor/src/app/infrastructure/components/mcp/mcp-page.component.ts`:
  - Standalone, protected stub
  - Placeholder: "MCP Connections" heading

- [x] 4.6 Create `VaultPageComponent` at `frontend/codeauditor/src/app/infrastructure/components/vault/vault-page.component.ts`:
  - Standalone, protected stub
  - Placeholder: "Vault" heading

## Phase 5: Verification

- [x] 5.1 Build Angular: `cd academy-mic/frontend/codeauditor && npx ng build`
- [x] 5.2 Fix any TypeScript or template errors
- [x] 5.3 Verify routes work: navigate to /dashboard, /dojo, /mcp, /vault
