# Delta for repo-browser

## ADDED Requirements

### Requirement: REQ-RB-001 — McpPage MUST display a list of repositories

The `McpPageComponent` SHALL fetch repositories from the Gogs backend on initialization and render them in a scrollable list. Each repository entry SHALL display `full_name` and `description`. If the repository list is empty, a placeholder message SHALL be shown.

#### Scenario: Repositories load on mount

- GIVEN the user navigates to the `/mcp` route
- WHEN the `McpPageComponent` initializes
- THEN the component SHALL call `GogsService.listRepos()`
- AND the component SHALL render a list of repository entries
- AND each entry SHALL show the repository `full_name` and `description`

#### Scenario: Empty repository list

- GIVEN the user has no repositories in Gogs
- WHEN the `McpPageComponent` finishes loading
- THEN the component SHALL display a message: "No repositories found. Connect a Gogs account to get started."
- AND no repository entries SHALL be rendered

#### Scenario: Repository list refreshes

- GIVEN the repository list is already displayed
- WHEN the user triggers a refresh action
- THEN the component SHALL re-fetch the repository list from the Gogs backend
- AND the displayed list SHALL update with the latest data

### Requirement: REQ-RB-002 — User MUST be able to select a repository and browse files

Upon selecting a repository from the list, the component SHALL display a file browser view showing the repository's file tree. The user SHALL be able to navigate directories and select individual files. The file browser SHALL support at least one level of directory depth.

#### Scenario: Select repository shows files

- GIVEN the repository list is displayed
- WHEN the user clicks on a repository entry
- THEN the component SHALL switch to a file browser view for that repository
- AND the file browser SHALL display the files in the repository's default branch

#### Scenario: Navigate into a directory

- GIVEN the file browser is displaying a directory listing
- WHEN the user clicks on a directory entry
- THEN the component SHALL update the file browser to show the contents of that subdirectory
- AND a breadcrumb or back navigation SHALL be available

#### Scenario: Select a file

- GIVEN the file browser is displaying a directory with files
- WHEN the user clicks on a file entry
- THEN the component SHALL highlight the selected file
- AND the file's content SHALL be fetched via `GogsService.getFile()`
- AND the component SHALL display the file content in a preview panel

### Requirement: REQ-RB-003 — On file selection, MUST create a temporary Challenge and navigate to /dojo/:tempId

When the user confirms a file selection (via a "Start Audit" or equivalent action), the component SHALL construct a temporary `Challenge` object with a synthetic `tempId`, store it via the `ChallengeService`, and navigate to `/dojo/:tempId`. The temporary challenge SHALL include the file content, file path as `repoUrl`, and appropriate default values for `difficulty`, `category`, `language`, and `codeSmell`.

#### Scenario: Temp challenge created from file

- GIVEN a file is selected and its content is loaded
- WHEN the user clicks "Start Audit"
- THEN the component SHALL create a `Challenge` object with a unique `tempId`
- AND the challenge `code` SHALL be set to the file content
- AND the challenge `repoUrl` SHALL be set to the file path
- AND the challenge SHALL be stored in the `ChallengeService`
- AND the router SHALL navigate to `/dojo/:tempId`

#### Scenario: Temp challenge has default values

- GIVEN a file is selected
- WHEN the temporary `Challenge` is constructed
- THEN `difficulty` SHALL default to `"mid"`
- AND `category` SHALL default to `"imported"`
- AND `language` SHALL be inferred from the file extension
- AND `codeSmell` SHALL default to `"pending-analysis"`

#### Scenario: Navigation to Dojo with temp challenge

- GIVEN the temp challenge is stored
- WHEN the router navigates to `/dojo/:tempId`
- THEN the `DojoPageComponent` SHALL retrieve the challenge by `tempId` from `ChallengeService`
- AND the Monaco editor SHALL display the imported file content
- AND the ContextPanel SHALL show the file path as the repository origin

### Requirement: REQ-RB-004 — MUST show loading states during API calls

The `McpPageComponent` SHALL display a loading indicator whenever an API call to the Gogs backend is in progress. This includes: initial repository list fetch, file tree fetch, and individual file content fetch.

#### Scenario: Loading during repo list fetch

- GIVEN the user navigates to `/mcp`
- WHEN the repository list request is in flight
- THEN the component SHALL display a loading spinner or "Loading repositories..." message
- AND the repository list SHALL NOT be visible until the request completes

#### Scenario: Loading during file tree fetch

- GIVEN the user selects a repository
- WHEN the file tree request is in flight
- THEN the component SHALL display a loading indicator in the file browser area
- AND the previous view (if any) SHALL remain visible or be replaced with the loading state

#### Scenario: Loading during file content fetch

- GIVEN the user selects a file
- WHEN the file content request is in flight
- THEN the component SHALL display a loading indicator in the preview panel
- AND the "Start Audit" action SHALL be disabled until loading completes

### Requirement: REQ-RB-005 — MUST show error states for failed API calls

The `McpPageComponent` SHALL display user-friendly error messages when any Gogs API call fails. Error messages SHALL be derived from the structured error response (`error` field) and SHALL include a retry option where applicable.

#### Scenario: Repo list fetch fails

- GIVEN the Gogs backend returns an error for the repo list
- WHEN the error response is received
- THEN the component SHALL display the error message (e.g., "Gogs service is unavailable")
- AND the component SHALL offer a "Retry" button to re-attempt the fetch

#### Scenario: File fetch fails

- GIVEN the user selects a file and the file content request fails
- WHEN the error response is received
- THEN the component SHALL display the error message in the preview area
- AND the component SHALL offer a "Retry" button
- AND the previously selected repository view SHALL remain intact

#### Scenario: File too large error

- GIVEN the user selects a file that exceeds the 1 MB limit
- WHEN the backend returns a `FILE_TOO_LARGE` error
- THEN the component SHALL display: "This file exceeds the maximum size of 1 MB. Please select a smaller file."
- AND no retry option SHALL be offered (the file cannot be fetched)

### Requirement: REQ-RB-006 — GogsService MUST be injectable and use Angular HttpClient

The `GogsService` SHALL be an Angular injectable service (`@Injectable({ providedIn: 'root' })`) that wraps all Gogs backend API calls. It SHALL use Angular's `HttpClient` for HTTP requests and return `Observable` types. The service SHALL expose at minimum `listRepos(): Observable<Repo[]>` and `getFile(params: FileRequest): Observable<FileContent>`.

#### Scenario: Service is injectable

- GIVEN any component or service in the Angular application
- WHEN it injects `GogsService` via the constructor or `inject()` function
- THEN the service instance SHALL be provided and functional
- AND no additional module configuration SHALL be required

#### Scenario: listRepos returns observable

- GIVEN the `GogsService` is injected
- WHEN `listRepos()` is called
- THEN it SHALL return an `Observable<Repo[]>`
- AND the underlying implementation SHALL use `HttpClient.get()` to call `/api/v1/gogs/repos`

#### Scenario: getFile returns observable

- GIVEN the `GogsService` is injected
- WHEN `getFile({owner, repo, branch, path})` is called
- THEN it SHALL return an `Observable<FileContent>`
- AND the underlying implementation SHALL use `HttpClient.post()` to call `/api/v1/gogs/file` with the params as JSON body
