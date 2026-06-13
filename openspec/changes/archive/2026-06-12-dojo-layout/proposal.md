# Proposal: Dojo UI Layout

## Intent

Implement the foundational "Dojo" application layout to provide the structured workspace necessary for code auditing challenges. This layout strictly follows the "Dark IDE / Cyber-minimalista" design manifesto, creating a hacking-tool aesthetic that separates challenge context from execution impact.

## Scope

### In Scope
- Minimalist Sidebar component with navigation icons (Dashboard, Dojo, MCP Connections, Vault).
- Main Layout container implementing the primary structural zones.
- Left Zone (Contexto): Panel for code smell descriptions and repository origins.
- Right Zone (Impacto): Split-view panel (Code editor area + integrated test terminal area).
- Application of Dark IDE aesthetic (sharp edges, code editor-like panels) using the existing Tailwind v4 Dojo palette in `styles.css`.
- Typographic setup (Inter/Roboto for reading, monospace for code/titles).
- Angular page stubs: Dashboard, Dojo, MCP Connections, and Vault.

### Out of Scope
- Integration of a real code editor (CodeMirror/Monaco); will use styled placeholders.
- Real terminal integration or SSE streaming; will use a styled output area.
- Functional business logic, state management (Signals), or backend routing for these views.

## Capabilities

### New Capabilities
- `dojo-layout`: Defines the main workspace structure, navigation, and visual zones (Context and Impact) for code challenges.

### Modified Capabilities
- None

## Approach

Create an Angular 21 Standalone Layout component utilizing CSS Grid to establish the Sidebar, Left Zone (Context), and Right Zone (Impact). The Right Zone will use a vertical Flexbox split for the Code and Terminal sections. Tailwind v4 utility classes will enforce the aesthetic: `bg-[#0D1117]` for the base, `bg-[#161B22]` for surfaces, and `rounded-sm` or `rounded-none` for sharp geometry. Navigation will be wired in `app.routes.ts` pointing to lightweight standalone page stubs.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `frontend/codeauditor/src/app/app.routes.ts` | Modified | Add routes for Dashboard, Dojo, MCP Connections, and Vault. |
| `frontend/codeauditor/src/app/app.component.html` | Modified | Update root component to use the new Layout. |
| `frontend/codeauditor/src/app/layout/` | New | Create Sidebar and Main Layout components. |
| `frontend/codeauditor/src/app/pages/` | New | Create placeholder components for the main views. |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Over-engineering the stubs | Medium | Strictly limit components to static HTML/CSS structure; defer all logic to later phases. |
| Responsive layout issues on small screens | Low | Focus strictly on desktop "IDE" experience for now, matching the domain focus. |

## Rollback Plan

Revert routing changes in `app.routes.ts`, restore the original `app.component.html`, and delete the newly created `layout/` and `pages/` directories.

## Dependencies

- Existing Tailwind v4 configuration and Dojo color palette in `src/styles.css`.
- Angular 21 Standalone Component architecture.

## Success Criteria

- [ ] Sidebar allows navigation between Dashboard, Dojo, MCP Connections, and Vault stubs.
- [ ] Dojo view correctly implements a Left Context panel and a Right Impact split-panel (Code/Terminal).
- [ ] Aesthetic strictly adheres to the Dark IDE manifesto (abyssal black background, sharp borders, appropriate typography).
