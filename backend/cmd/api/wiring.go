// Package main — production composition root for the CodeAuditor MCP
// monolith. BuildMCPRegistry wires the 33 S2 tools + 5 S4
// code-health tools across 9 groups onto a single mcpkit.Registry
// so cmd/api/main.go and tests can both call it. The function is
// extracted from main() so the registry wiring is testable without
// booting a database / Docker / HTTP server.
//
// Tool inventory (mirrors openspec/changes/mcp-monolith/specs/mcp/spec.md
// Requirement: S2 Tool Inventory — 33 tools, plus 5 S4 code-health
// tools for 38 total):
//
//	mcp-sandbox       (4)  run_code, list_languages, healthcheck, get_install_hint
//	mcp-challenges    (5)  list_challenges, get_challenge, create_temp_challenge,
//	                          generate_challenge, validate_challenge
//	mcp-user          (6)  get_profile, get_language_progress, get_history,
//	                          get_learning_profile, record_audit_attempt,
//	                          update_learning_profile
//	mcp-curriculum    (4)  get_topics, get_topic, suggest_topic, review_topic
//	mcp-dictionary    (4)  search_term, get_term, add_term, list_terms
//	mcp-pedagogical   (6)  get_learning_objective, generate_story_context,
//	                          create_scaffolded_steps, validate_solution,
//	                          get_socratic_prompt, peer_review_topic
//	mcp-external_docs (3)  get_library_docs, search_documentation, get_real_example
//	mcp-tutor         (1)  lookup (shipped via the dictionary change)
//	mcp-code-health   (5)  analyze_repo, get_analysis, get_repo_issues,
//	                          generate_health_report, suppress_issue
//	                          (S4 code-health-dashboard, PR 2)
//
// Deferred (S3+/S5+ — must stay absent): mcp-inspiration.
package main

import (
	"github.com/anomalyco/codeauditor/backend/internal/core/services"
	codehealthsvc "github.com/anomalyco/codeauditor/backend/internal/core/services/code_health"
	sandboxpkg "github.com/anomalyco/codeauditor/backend/internal/infrastructure/driven/sandbox"
	"github.com/anomalyco/codeauditor/backend/internal/infrastructure/driven/sandbox/providers"
	challengesmcp "github.com/anomalyco/codeauditor/backend/internal/infrastructure/mcp/challenges"
	codehealthmcp "github.com/anomalyco/codeauditor/backend/internal/infrastructure/mcp/code_health"
	curriculummcp "github.com/anomalyco/codeauditor/backend/internal/infrastructure/mcp/curriculum"
	dictionarymcp "github.com/anomalyco/codeauditor/backend/internal/infrastructure/mcp/dictionary"
	externaldocsmcp "github.com/anomalyco/codeauditor/backend/internal/infrastructure/mcp/external_docs"
	pedagogicalmcp "github.com/anomalyco/codeauditor/backend/internal/infrastructure/mcp/pedagogical"
	sandboxmcp "github.com/anomalyco/codeauditor/backend/internal/infrastructure/mcp/sandbox"
	tutormcp "github.com/anomalyco/codeauditor/backend/internal/infrastructure/mcp/tutor"
	usermcp "github.com/anomalyco/codeauditor/backend/internal/infrastructure/mcp/user"
	"github.com/anomalyco/codeauditor/backend/internal/mcpkit"
)

// WiringDeps is the bag of services BuildMCPRegistry needs to wire the
// 32 S2 tools. Every field is required; main.go constructs each
// service before calling BuildMCPRegistry so the wiring stays
// testable (tests can construct minimal services for registry-shape
// assertions without booting Docker or Postgres).
type WiringDeps struct {
	// Driven sandbox — used by mcp-sandbox.run_code (which enforces
	// --cap-drop=ALL, --network=none, --read-only at the type level).
	DockerSandbox *sandboxpkg.DockerSandbox
	// Provider registry — used by mcp-sandbox.list_languages +
	// mcp-sandbox.get_install_hint to enumerate and install languages.
	ProviderRegistry *providers.ProviderRegistry
	// Core services — every MCP tool delegates to one of these.
	DictionaryService   *services.DictionaryService
	ChallengeService    *services.ChallengeService
	TutorLookupService  *services.TutorLookupService
	UserService         *services.UserService
	UserProgressService *services.UserProgressService
	AuditHistoryService *services.AuditHistoryService
	CurriculumService   *services.CurriculumService
	PedagogicalService  *services.PedagogicalService
	ExternalDocsService *services.ExternalDocsService
	// CodeHealthService (S4) — the orchestrator for the
	// mcp-code-health group. The wiring in main.go
	// constructs it after the multi-source clients are
	// built; nil-safe (BuildMCPRegistry tolerates a nil
	// service: the adapter returns an error per call).
	CodeHealthService *codehealthsvc.CodeHealthService
}

// BuildMCPRegistry wires every S2 tool onto a fresh *mcpkit.Registry.
// Callers must NOT reuse the registry across servers (Register panics
// on duplicate tool names per mcpkit's contract).
//
// The order of registration matches the spec's group table so a
// reviewer scanning the function reads it as a 1:1 mirror of the
// inventory. New groups (S3 inspiration, S4 code_health) land in
// their own PRs.
func BuildMCPRegistry(deps WiringDeps) *mcpkit.Registry {
	reg := mcpkit.NewRegistry()

	// mcp-dictionary (4) — shipped via the dictionary change.
	dictMCPSvc := dictionarymcp.NewServiceAdapter(deps.DictionaryService)
	reg.Register(dictionarymcp.NewSearchTermTool(dictMCPSvc))
	reg.Register(dictionarymcp.NewGetTermTool(dictMCPSvc))
	reg.Register(dictionarymcp.NewAddTermTool(dictMCPSvc))
	reg.Register(dictionarymcp.NewListTermsTool(dictMCPSvc))

	// tutor.lookup (1) — shipped via the dictionary change; the
	// tutor LLM uses it to ground term-lookup questions in the
	// user's personal glossary.
	tutorMCPSvc := tutormcp.NewServiceAdapter(deps.TutorLookupService)
	reg.Register(tutormcp.NewTutorLookupTool(tutorMCPSvc))

	// mcp-sandbox (4) — wired to the DockerSandbox + ProviderRegistry
	// so run_code can enforce the production security flags even when
	// the audit flow has fallen back to LocalSandbox.
	sandboxMCPSvc := sandboxmcp.NewServiceAdapter(deps.DockerSandbox, deps.ProviderRegistry)
	reg.Register(sandboxmcp.NewRunCodeTool(sandboxMCPSvc))
	reg.Register(sandboxmcp.NewListLanguagesTool(sandboxMCPSvc))
	reg.Register(sandboxmcp.NewHealthcheckTool(sandboxMCPSvc))
	reg.Register(sandboxmcp.NewGetInstallHintTool(sandboxMCPSvc))

	// mcp-challenges (5) — wired to the production ChallengeService.
	// generate_challenge is a stub returning ErrNotImplemented until
	// the S5 LLM cascade lands (see PR 2 amendment for the contract).
	challengesMCPSvc := challengesmcp.NewServiceAdapter(deps.ChallengeService)
	reg.Register(challengesmcp.NewListChallengesTool(challengesMCPSvc))
	reg.Register(challengesmcp.NewGetChallengeTool(challengesMCPSvc))
	reg.Register(challengesmcp.NewCreateTempChallengeTool(challengesMCPSvc))
	reg.Register(challengesmcp.NewGenerateChallengeTool(challengesMCPSvc))
	reg.Register(challengesmcp.NewValidateChallengeTool(challengesMCPSvc))

	// mcp-user (6) — wired to UserService + UserProgressService +
	// AuditHistoryService. The ServiceAdapter owns the cross-service
	// choreography for record_audit_attempt (SaveSession +
	// RecordAuditCompletion happen in sequence).
	userMCPSvc := usermcp.NewServiceAdapter(deps.UserService, deps.UserProgressService, deps.AuditHistoryService)
	reg.Register(usermcp.NewGetProfileTool(userMCPSvc))
	reg.Register(usermcp.NewGetLanguageProgressTool(userMCPSvc))
	reg.Register(usermcp.NewGetHistoryTool(userMCPSvc))
	reg.Register(usermcp.NewGetLearningProfileTool(userMCPSvc))
	reg.Register(usermcp.NewRecordAuditAttemptTool(userMCPSvc))
	reg.Register(usermcp.NewUpdateLearningProfileTool(userMCPSvc))

	// mcp-curriculum (4) — wired to the production CurriculumService
	// (reads .atl/curricula/<lang>.md per docs/curricula-format.md).
	// The path-traversal guard is the same one unit-tested in
	// curriculum_tool_test.go (3.1); the integration test in
	// mcp/curriculum_http_transport_test.go proves the dispatcher
	// mapping to JSON-RPC -32602.
	curriculumMCPSvc := curriculummcp.NewServiceAdapter(deps.CurriculumService)
	reg.Register(curriculummcp.NewGetTopicsTool(curriculumMCPSvc))
	reg.Register(curriculummcp.NewGetTopicTool(curriculumMCPSvc))
	reg.Register(curriculummcp.NewSuggestTopicTool(curriculumMCPSvc))
	reg.Register(curriculummcp.NewReviewTopicTool(curriculumMCPSvc))

	// mcp-pedagogical (6) — the centerpiece of the tutor chat flow.
	// PedagogicalService owns the socratic prompt construction
	// (level 0..3, prompt-injection guard) and the scaffolding
	// algorithm. The MCP tools in this group are thin wrappers
	// that delegate to the service.
	pedagogicalMCPSvc := pedagogicalmcp.NewServiceAdapter(deps.PedagogicalService)
	reg.Register(pedagogicalmcp.NewGetLearningObjectiveTool(pedagogicalMCPSvc))
	reg.Register(pedagogicalmcp.NewGenerateStoryContextTool(pedagogicalMCPSvc))
	reg.Register(pedagogicalmcp.NewCreateScaffoldedStepsTool(pedagogicalMCPSvc))
	reg.Register(pedagogicalmcp.NewValidateSolutionTool(pedagogicalMCPSvc))
	reg.Register(pedagogicalmcp.NewGetSocraticPromptTool(pedagogicalMCPSvc))
	reg.Register(pedagogicalmcp.NewPeerReviewTopicTool(pedagogicalMCPSvc))

	// mcp-external_docs (3) — wired to the production
	// ExternalDocsService, which fuses Context7 (library docs) +
	// Brave Search (general web) via two ports (DocsClient +
	// WebSearchClient). API keys NEVER leave the backend.
	externalDocsMCPSvc := externaldocsmcp.NewServiceAdapter(deps.ExternalDocsService)
	reg.Register(externaldocsmcp.NewGetLibraryDocsTool(externalDocsMCPSvc))
	reg.Register(externaldocsmcp.NewSearchDocumentationTool(externalDocsMCPSvc))
	reg.Register(externaldocsmcp.NewGetRealExampleTool(externalDocsMCPSvc))

	// mcp-code-health (5) — S4 code-health dashboard. Wired
	// to the production CodeHealthService (constructed in
	// main.go with the multi-source client + 6 seeded
	// rules + Supabase repository + 7d AnalysisCache).
	// The adapter bridges the *services.CodeHealthService
	// to the local Service interface so the tools stay
	// swappable for tests.
	codeHealthMCPSvc := codehealthmcp.NewServiceAdapter(deps.CodeHealthService)
	reg.Register(codehealthmcp.NewAnalyzeRepoTool(codeHealthMCPSvc))
	reg.Register(codehealthmcp.NewGetAnalysisTool(codeHealthMCPSvc))
	reg.Register(codehealthmcp.NewGetRepoIssuesTool(codeHealthMCPSvc))
	reg.Register(codehealthmcp.NewGenerateHealthReportTool(codeHealthMCPSvc))
	reg.Register(codehealthmcp.NewSuppressIssueTool(codeHealthMCPSvc))

	return reg
}
