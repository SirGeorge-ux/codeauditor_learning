# Dojo Layout Specification

## Purpose
Defines the main workspace structure, navigation, and visual zones (Context and Impact) for code challenges, strictly applying the Dark IDE / Cyber-minimalista aesthetic.

## Requirements

### Requirement: Sidebar Navigation
The system MUST provide a sidebar with 4 nav items (Dashboard, Dojo, MCP Connections, Vault) that highlights the active route and supports a collapsed/expanded state.

#### Scenario: Default collapsed state
- GIVEN the application is loaded
- WHEN the sidebar is rendered
- THEN it MUST default to a collapsed state showing only icons

#### Scenario: Expand on hover
- GIVEN the sidebar is collapsed
- WHEN the user hovers over the sidebar
- THEN it MUST expand to show text labels alongside icons

#### Scenario: Active route highlight
- GIVEN the user is on the /dojo route
- WHEN the sidebar is visible
- THEN the Dojo navigation item MUST display a distinct active styling

#### Scenario: Route navigation
- GIVEN the user clicks a navigation item
- WHEN the click event fires
- THEN the system MUST navigate to the corresponding route

#### Scenario: Persistence across views
- GIVEN the user navigates between views
- WHEN the new route loads
- THEN the sidebar state and active highlight MUST update accordingly

### Requirement: Main Layout
The system MUST structure the main layout as a Flexbox container combining the sidebar and a content area.

#### Scenario: Two-pane structure
- GIVEN the application initializes
- WHEN the main layout renders
- THEN it MUST display the sidebar and a main content area side-by-side

#### Scenario: Responsive content area
- GIVEN the main layout is active
- WHEN the viewport resizes
- THEN the main content area MUST fill all remaining horizontal space

#### Scenario: Sidebar width constraints
- GIVEN the sidebar expands or collapses
- WHEN the transition occurs
- THEN the main content area MUST adjust its width without breaking the layout

#### Scenario: Content area scrolling
- GIVEN the main content exceeds the viewport height
- WHEN the user scrolls
- THEN only the content area MUST scroll, leaving the sidebar fixed

#### Scenario: Layout stability
- GIVEN varying amounts of content in the main area
- WHEN the content changes dynamically
- THEN the Flexbox structure MUST prevent content overflow from pushing the sidebar off-screen

### Requirement: Left Zone (Context)
The system MUST provide a Left Zone panel displaying the challenge description, repository origin, and code smell information.

#### Scenario: Render challenge description
- GIVEN a challenge is selected
- WHEN the Left Zone renders
- THEN it MUST display the challenge description text

#### Scenario: Render repository origin
- GIVEN a challenge has a repository source
- WHEN the user views the Left Zone
- THEN it MUST clearly display the repository origin URL or path

#### Scenario: Render code smell info
- GIVEN the challenge focuses on a specific code smell
- WHEN the Left Zone is active
- THEN it MUST display the code smell name and definition

#### Scenario: Independent vertical scrolling
- GIVEN the context text is very long
- WHEN the user scrolls the Left Zone
- THEN only the Left Zone content MUST scroll

#### Scenario: Empty state handling
- GIVEN no challenge is currently active
- WHEN the Left Zone renders
- THEN it MUST display a placeholder message indicating no context

### Requirement: Right Zone (Impact)
The system MUST provide a Right Zone split-view panel featuring a code display area and a terminal output area.

#### Scenario: Vertical split initialization
- GIVEN the Dojo view is loaded
- WHEN the Right Zone renders
- THEN it MUST display as a vertically split container

#### Scenario: Code editor placeholder
- GIVEN the user views the top half of the Right Zone
- WHEN the area renders
- THEN it MUST display a styled placeholder for the code editor

#### Scenario: Terminal output placeholder
- GIVEN the user views the bottom half of the Right Zone
- WHEN the area renders
- THEN it MUST display a styled placeholder for the terminal

#### Scenario: Proportional sizing
- GIVEN the Right Zone renders initially
- WHEN no resizing has occurred
- THEN the code and terminal panels MUST maintain a predetermined height ratio

#### Scenario: Terminal scrolling
- GIVEN the terminal area receives extensive simulated output
- WHEN the output exceeds the panel height
- THEN the terminal panel MUST provide independent vertical scrolling

### Requirement: Theme Application
The system MUST apply the Dark IDE styling per the Dojo palette, using specific dark backgrounds, sharp geometry, and distinct typography.

#### Scenario: Base background application
- GIVEN the application is viewed
- WHEN the layout renders
- THEN the base background MUST use the `bg-[#0D1117]` utility

#### Scenario: Surface background application
- GIVEN a panel like the Left Zone renders
- WHEN the user views the panel
- THEN the surface background MUST use the `bg-[#161B22]` utility

#### Scenario: Sharp geometry enforcement
- GIVEN a UI component renders
- WHEN the user inspects its borders
- THEN it MUST use `rounded-none` or `rounded-sm` for sharp edges

#### Scenario: Reading typography
- GIVEN description or instruction text renders
- WHEN the text is displayed
- THEN it MUST use a sans-serif font (Inter or Roboto)

#### Scenario: Monospace typography
- GIVEN code, terminal output, or technical titles render
- WHEN the text is displayed
- THEN it MUST use a monospace font family

### Requirement: Routing Configuration
The system MUST configure routing for /dashboard, /dojo, /mcp, and /vault, directing to standalone page stubs.

#### Scenario: Dashboard route
- GIVEN the application is running
- WHEN the user navigates to `/dashboard`
- THEN the Dashboard stub component MUST render

#### Scenario: Dojo route
- GIVEN the application is running
- WHEN the user navigates to `/dojo`
- THEN the Dojo stub component MUST render within the layout

#### Scenario: MCP Connections route
- GIVEN the application is running
- WHEN the user navigates to `/mcp`
- THEN the MCP Connections stub component MUST render

#### Scenario: Vault route
- GIVEN the application is running
- WHEN the user navigates to `/vault`
- THEN the Vault stub component MUST render

#### Scenario: Default route fallback
- GIVEN the application is running
- WHEN the user navigates to an undefined or base route
- THEN the system MUST redirect to the `/dashboard` route