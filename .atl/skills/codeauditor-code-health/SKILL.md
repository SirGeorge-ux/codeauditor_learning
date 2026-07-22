---
name: codeauditor-code-health
description: Use this skill when an agent needs to analyze a repository (Gogs or GitHub) for code health, or design the code-health-dashboard feature. Trigger phrases include "code health", "analizar mi repo", "audit my repo", "analyze academy-mic", "detectar code smells en mi código", "github audit", "gogs audit", "repo health", "health score".
---

# Code Health (S4+)

> **Outcome:** a complete health dashboard for a repo (Gogs or GitHub), with ranked issues, health score, and the option to generate challenges from the detected issues.
> **Time estimate:** 30s-2min per repo (depends on size + cache).
> **LLM used:** Ollama local (qwen2.5-coder:3b) for file-by-file, DeepSeek V3 (free tier) for whole-repo analysis.

## When to load

Load this skill when the user asks:
- "Analiza mi repo academy-mic"
- "Dame los 5 issues más importantes de mi código"
- "Quiero un code health de <owner>/<repo>"
- "Audita el repo <X> de Gogs"
- "Audita el repo <X> de GitHub"
- "Generate challenges from my repo"

## Pre-flight

1. Read `AGENTS.md` (root).
2. Read `docs/business-domain.md` §3.3 (Code health mode flow).
3. Read `docs/llm-strategy.md` (when to use Ollama vs DeepSeek).
4. Read `docs/mcp-tools.md` §3.7 (`mcp-code-health`).
5. Check the source: is it Gogs or GitHub?

## The flow (high level)

```
User: "Analiza el repo academy-mic en Gogs"
   ↓
1. mcp-code-health.analyze_repo("academy-mic", "gogs")
   ↓
2. Background job starts:
   ├─ Get repo tree (Gogs API or GitHub API)
   ├─ Filter files: source files only, size < 1MB, total < 300 files
   ├─ For each file:
   │   ├─ Check cache by SHA (skip if cached)
   │   ├─ LLM analysis (Ollama local for 1 file, DeepSeek for big files)
   │   ├─ Detect: security, performance, refactor, style issues
   │   └─ Cache the result
   └─ Rank issues by severity
   ↓
3. mcp-code-health.get_analysis(analysis_id)
   ↓
4. Dashboard displays:
   ├─ Health score (0-100)
   ├─ Top 10 issues (with link to file + line)
   ├─ Languages breakdown
   ├─ "Generate challenges from these" (creates curated challenges)
   └─ "Drill into issue #N"
```

## What gets detected

| Categoría | Ejemplos |
|---|---|
| **security** | SQLi, XSS, command injection, hardcoded secrets, insecure deserialization, SSRF, path traversal |
| **performance** | N+1 queries, O(n²) loops, memory leaks, missing indexes, unnecessary re-renders |
| **refactor** | God classes, long methods (>50 lines), duplicated code, feature envy, inappropriate intimacy |
| **style** | Magic numbers, poor naming, dead code, unused imports, console.log left in |
| **architecture** | Leaky abstractions, tight coupling, circular dependencies, god modules |

## Health score calculation

```
health_score = 100 - Σ(weight_per_issue)

weight_per_issue = {
  critical: 15,
  high: 8,
  medium: 3,
  low: 1,
}

clamp(0, 100)
```

## Step-by-step (when implementing or debugging)

### Step 1 — Validate the source

```go
if source != "gogs" && source != "github" {
    return error("invalid source")
}
```

### Step 2 — Get the repo tree

**Gogs:**
```go
files, err := gogsClient.ListFiles(ctx, owner, repo, branch)
// Returns: [{ path, size, sha, type }]
```

**GitHub:**
```go
files, err := githubClient.ListFiles(ctx, owner, repo, branch)
// Same shape
```

### Step 3 — Filter files

```go
sourceFiles := filter(files, isSourceFile, size<1MB)
// Skip: node_modules, .git, dist, build, vendor, generated
// Skip: images, binaries, large files
```

Limit to 300 files per analysis (configurable). If more, process in batches and update the user.

### Step 4 — Check cache

```go
for _, file := range sourceFiles {
    cached := cache.Get("analysis:" + file.SHA)
    if cached != nil {
        results = append(results, cached)
        continue
    }
    // Analyze
}
```

Cache key: `analysis:<sha>` (the file's git SHA, not path — same content = same analysis).

### Step 5 — Analyze with LLM

**For each file:**

```go
// Choose the model based on file size
model := "ollama-local"
if len(file.Content) > 50_000 {  // >50KB
    model = "openrouter-deepseek-v3"
}

prompt := fmt.Sprintf(`
Analyze this %s file for code health issues.

File: %s
Language: %s

```%s
%s
```

Detect issues in these categories:
- security: SQLi, XSS, command injection, hardcoded secrets, etc.
- performance: N+1, O(n²), memory leaks, etc.
- refactor: god classes, long methods, duplicated code, etc.
- style: magic numbers, poor naming, dead code, etc.
- architecture: leaky abstractions, tight coupling, etc.

Output as JSON:
{
  "issues": [
    {
      "severity": "low|medium|high|critical",
      "category": "...",
      "line": <int>,
      "message": "...",
      "evidence": "...",
      "suggested_fix": "..."
    }
  ]
}
`, file.Language, file.Path, file.Language, file.Language, file.Content)

result := llm.Generate(prompt, json_mode=true)
```

### Step 6 — Cache and accumulate

```go
cache.Set("analysis:"+file.SHA, result, 7*24*time.Hour)  // 7 days
results = append(results, result)
```

### Step 7 — Rank and aggregate

```go
allIssues := aggregate(results)
ranked := sortBySeverity(allIssues)
top10 := ranked[:min(10, len(ranked))]
healthScore := calculateHealthScore(allIssues)
```

### Step 8 — Persist and notify

```go
analysis := Analysis{
    ID: analysisID,
    Owner: owner,
    Name: name,
    Source: source,
    HealthScore: healthScore,
    TopIssues: top10,
    AllIssues: allIssues,
    LanguagesBreakdown: countByLanguage(files),
    Status: "completed",
    CreatedAt: now,
}
db.Save(analysis)
notifyUser(userID, analysisID)
```

## "Generate challenges from these" (the killer feature)

When the user clicks this button on the dashboard:

```go
for _, issue := range topIssues {
    challenge, err := mcpChallenges.GenerateChallengeFromIssue(ctx, issue, repo)
    if err != nil {
        log.Warn("could not generate challenge", err)
        continue
    }
    // Persist with origin="code_health"
    // Open in /dojo/<tempId>
}
```

The `GenerateChallengeFromIssue` is a variant of `GenerateChallenge` that uses:
- The actual code from the file.
- The issue as the "smell" to fix.
- The file's context (e.g. "this is in `academy-mic/backend/...`").
- `solution_code` is generated by the LLM based on the `suggested_fix`.

## Edge cases

| Caso | Manejo |
|---|---|
| Repo > 300 files | Process in batches, show progress bar |
| File > 1MB | Skip (too big for LLM context) + warn user |
| Repo is empty | Show "no source files" message |
| Gogs unreachable | Show error + retry button |
| Rate limit (GitHub) | Cache + use DeepSeek for the rest |
| Issue in a file the user didn't write | Don't generate a challenge (mark as "external contribution") |
| Same issue in many files | Group them, show count + file paths |

## Don't do

- ❌ Don't analyze files >1MB (LLM context overflow).
- ❌ Don't analyze >300 files at once (rate limits + UX).
- ❌ Don't use paid models by default (Ollama local is enough for 1 file).
- ❌ Don't generate a challenge without a `solution_code` (validatable).
- ❌ Don't show the analysis immediately if it takes >5s — show a progress bar.
- ❌ Don't cache by file path (use SHA, so a rename doesn't break cache).
- ❌ Don't expose the Gogs/GitHub token to the LLM (only the file content).

## Resources

- **Spec:** `openspec/changes/code-health-dashboard/`
- **Doc:** `docs/business-domain.md` §3.3
- **Doc:** `docs/llm-strategy.md`
- **Doc:** `docs/mcp-tools.md` §3.7
- **Skill:** `codeauditor-free-practice-generator` (similar LLM generation flow)
- **Harness:** `add-code-health-rule.harness.md`
