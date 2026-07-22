# Harness: Fix a Failing Test

> **Step-by-step workflow** for diagnosing and fixing a failing test in CodeAuditor.
> **Audience:** devs, agents, juniors.
> **Estimated time:** 5-30 minutes.
> **Output:** the test passes, root cause understood, no regression.

---

## Prerequisites

- [ ] You can run `make test` (or `make test-backend` / `make test-frontend`).
- [ ] You have the failing test name or a chunk of the error message.

---

## Step 1 — Reproduce in isolation

Don't try to fix what you can't reproduce.

### Backend (Go)

```bash
cd backend

# Run a specific test
go test -v -run Test<Name> ./internal/<package>/...

# Run a specific test in a file
go test -v -run TestNameOfTest ./internal/package/file_test.go

# With more verbosity
go test -v -count=1 -run TestName ./internal/...

# Skip integration tests
go test -short ./...
```

### Frontend (Vitest)

```bash
cd frontend/codeauditor

# Run a specific test file
npx ng test --include='**/<name>.spec.ts'

# Run a specific test by name
npx ng test -- --testNamePattern='<pattern>'

# Watch mode (re-runs on save)
npx ng test --watch
```

### E2E (Playwright)

```bash
cd frontend/codeauditor

# Run a specific test
pnpm e2e -- --grep "<test name>"

# Headed mode (see the browser)
pnpm e2e -- --grep "<test name>" --headed

# Update snapshots
pnpm e2e -- --grep "<test name>" --update-snapshots
```

---

## Step 2 — Read the error

The error message is the most important thing. Categorize it:

| Error type | What it tells you | Next step |
|---|---|---|
| **Assertion failure** | The test expected X, got Y. | Compare expected vs actual. Find the gap. |
| **Compile error** | Type / signature mismatch. | Read the line. Fix the type. |
| **Import error** | Missing / wrong path. | Check the path exists, the export exists. |
| **Timeout** | Async code didn't complete. | Add timeout, check for missing `await` or `subscribe`. |
| **Network error** | Test depended on something running. | Mock the HTTP, or use `httptest`. |
| **Setup error** | Test bed / fixture failed. | Check `beforeEach`, providers, mocks. |
| **Snapshot mismatch** | UI changed. | Review the diff, accept if intentional, fix code if regression. |
| **Flaky** | Sometimes passes, sometimes not. | Look for race conditions, shared state, timing. |

---

## Step 3 — Common causes (cheat sheet)

### 3.1 Backend Go

| Symptom | Likely cause | Fix |
|---|---|---|
| `undefined: X` | Missing import or variable | Add import or define |
| `cannot use Y as type Z` | Type mismatch | Check signatures match |
| `nil pointer dereference` | You dereffed nil | Add nil check, or fix the test setup |
| `context deadline exceeded` | Test took too long | Increase timeout, or speed up code |
| Test panics on startup | Global state init failed | Check `main.go` / setup functions |
| Test passes alone, fails in suite | Shared state / global var | Use `t.Cleanup` or move to local var |
| Mock not being called | Test mock setup wrong | Check the method name and args match |
| `expected X, got Y` and Y looks right | Test expectation wrong | Update the test, not the code (unless bug) |

### 3.2 Frontend Angular

| Symptom | Likely cause | Fix |
|---|---|---|
| `Cannot read property of undefined` | Async data not loaded | Use `signal()` with null check, or wait for it |
| `expect was called but no request` | Test didn't make the call | Check the component logic |
| `expected one matching request, found 0` | Wrong URL or method | Check `httpMock.expectOne(url)` URL |
| `No provider for X` | Missing service in TestBed | Add to `providers` array |
| `NG0950: Input is required` | Missing required input | Provide all required inputs in test |
| `Timeout in test` | Observable not subscribed | `subscribe()` in test |
| `Cannot match any routes` | Router config issue | Add `provideRouter()` in test bed |
| Test passes alone, fails with others | Test pollution | Use `beforeEach` to reset state |

### 3.3 E2E Playwright

| Symptom | Likely cause | Fix |
|---|---|---|
| `browserType.launch: ...` | Chromium deps missing | Install system libs (see quality-gate skill) |
| `Element not found` | Selector wrong or page not loaded | Use `await page.waitForSelector(...)` |
| `Timeout exceeded` | Network slow or page heavy | Increase timeout: `test.setTimeout(30000)` |
| `Element is not visible` | Display none, z-index, off-screen | Check CSS, scroll to element |
| `Locator.click: Element is intercepted` | Overlay or animation | Use `force: true` or wait for animation |
| `Strict mode violation` | Selector matches multiple elements | Use `.first()`, `.nth(0)`, or more specific selector |

---

## Step 4 — Decide: fix the test, or fix the code?

This is the critical question.

### Fix the test when:
- The test is **too strict** (e.g. expects exact timestamp).
- The test was **wrong from the start** (e.g. expected behavior is different from spec).
- The test is **testing implementation details** instead of behavior.
- The test **depends on the old API** but the API was intentionally changed.

### Fix the code when:
- The code has a real **bug**.
- The code has a **regression** from a recent change.
- The code is **not implementing the spec**.
- The test was **right**, and the code was wrong.

**Rule of thumb:** if the test was passing before the change you're working on, and now it fails, the **code** is the problem.

If the test was failing on master, and you didn't touch it, look at **why it was failing** (probably flaky, outdated, or never worked).

---

## Step 5 — Fix

### Pattern 1: Update the assertion

```go
// Before
if got := result.ID; got != "old-id" {
    t.Errorf("ID = %q, want %q", got, "old-id")
}

// After (code changed, expected value too)
if got := result.ID; got != "new-id" {
    t.Errorf("ID = %q, want %q", got, "new-id")
}
```

### Pattern 2: Add a missing mock

```typescript
// Before (test failed with "No provider for StatsService")
beforeEach(() => {
  TestBed.configureTestingModule({}); // empty
});

// After
beforeEach(() => {
  TestBed.configureTestingModule({
    providers: [
      provideHttpClient(),
      provideHttpClientTesting(),
      { provide: StatsService, useClass: MockStatsService }
    ]
  });
});
```

### Pattern 3: Wait for async

```typescript
// Before (race condition)
it('loads data', () => {
  component.ngOnInit();
  expect(component.data()).toBeTruthy(); // fails: data not loaded yet
});

// After
it('loads data', async () => {
  component.ngOnInit();
  await fixture.whenStable();
  expect(component.data()).toBeTruthy();
});
```

### Pattern 4: Fix the code

```go
// Before (bug: nil pointer if challenge is not found)
func (s *ChallengeService) GetChallenge(ctx context.Context, id string) (*models.Challenge, error) {
    c, err := s.repo.GetByID(ctx, id)
    if err != nil {
        return nil, err
    }
    return c, nil // <- if c is nil, caller panics
}

// After
func (s *ChallengeService) GetChallenge(ctx context.Context, id string) (*models.Challenge, error) {
    c, err := s.repo.GetByID(ctx, id)
    if err != nil {
        return nil, fmt.Errorf("get challenge %q: %w", id, err)
    }
    if c == nil {
        return nil, ErrChallengeNotFound
    }
    return c, nil
}
```

---

## Step 6 — Verify the fix

```bash
# Run the specific test
go test -v -run TestXxx ./internal/...

# Run all tests in the package
go test -v ./internal/<package>/...

# Run all tests
go test ./...

# Frontend
pnpm test

# E2E
pnpm e2e -- --grep "<test>"
```

The test should pass. **And no other tests should fail.**

If you broke another test, **you didn't fix the problem, you moved it.**

---

## Step 7 — Re-run the full quality gate

```bash
make fix
make validate
make test
make build
```

If anything else fails, your fix had unintended side effects. Investigate.

---

## Step 8 — Commit

```bash
git add <changed files>
git commit -m "fix(<scope>): <what was fixed>

The test <TestName> was failing because <root cause>.
Fixed by <change>."
```

**Do NOT** include "and also fixed 5 other unrelated things." One commit per fix.

---

## Checklist

- [ ] Test reproduced in isolation.
- [ ] Error message read and categorized.
- [ ] Root cause identified.
- [ ] Decision: fix test OR fix code (with reason).
- [ ] Fix applied.
- [ ] Specific test passes.
- [ ] Full test suite passes.
- [ ] `make validate` passes.
- [ ] `make build` passes.
- [ ] One commit, descriptive message.

---

## Anti-patterns

| Anti-pattern | Why it's bad |
|---|---|
| `t.Skip()` to make the test pass | You're hiding a problem. Open an issue. |
| Delete the test | You lose coverage. Fix the test or fix the code. |
| `// @ts-ignore` to bypass types | You're hiding a type error. Fix the types. |
| Disable a lint rule for one line | Find a real fix. |
| `time.Sleep(1000)` to wait for async | Use proper async/await. |
| `force: true` in Playwright | Use proper waits. |
| "It works on my machine" | It must work in CI. Reproduce in CI conditions. |
| Change the test to match the wrong code | If the spec is right, the code is wrong. |

---

## If all else fails

1. **Revert** your change: `git checkout -- <file>`.
2. **Verify** the test passes on master.
3. **Cherry-pick** your changes one by one to find which one breaks.
4. **Bisect** with `git bisect` if it's a regression.

```bash
git bisect start
git bisect bad
git bisect good <commit-before-my-changes>
# Test each commit
# When found:
git bisect reset
```

---

## Reference: how to read Go test output

```text
=== RUN   TestPythonProvider
--- FAIL: TestPythonProvider (0.00s)
    providers_test.go:15: Language() = "Python", want "python"
FAIL
FAIL    github.com/anomalyco/codeauditor/backend/internal/infrastructure/driven/sandbox/providers    0.002s
```

Translation:
- `TestPythonProvider` is the failing test.
- `providers_test.go:15` is the line that failed.
- The assertion was: `Language() == "python"`.
- The actual value was `"Python"` (capital P).
- The test is at line 15.

Fix: either change the test to `"Python"` (if that's correct) or change the implementation to return lowercase.

---

## Reference: how to read Vitest output

```text
FAIL  src/app/infrastructure/services/stats.service.spec.ts > StatsService > fetches user stats
AssertionError: expected 'GET' to be 'POST'
 ❯ src/app/infrastructure/services/stats.service.spec.ts:25:27
    23|     service.getUserStats().subscribe(stats => {
    24|       expect(stats.totalAudits).toBe(42);
    25|       expect(req.request.method).toBe('GET');
```

Translation:
- The test "fetches user stats" in `stats.service.spec.ts` failed.
- Line 25, assertion `method === 'GET'`.
- Actual: `'GET'` (left side).
- Expected: `'POST'` (right side).
- Wait — the test EXPECTS `'POST'` but the test name says `getUserStats`. The service is GET-ing but the test was set up to expect POST. Either:
  - The service is correct (GET) and the test is wrong (should expect GET).
  - The service is wrong (should be POST) and the test is right.

Look at the spec / API contract to decide.
