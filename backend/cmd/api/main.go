package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	_ "github.com/lib/pq"

	"github.com/anomalyco/codeauditor/backend/internal/core/services"
	codehealth "github.com/anomalyco/codeauditor/backend/internal/core/services/code_health"
	"github.com/anomalyco/codeauditor/backend/internal/core/services/code_health/rules"
	"github.com/anomalyco/codeauditor/backend/internal/infrastructure/driven/brave"
	"github.com/anomalyco/codeauditor/backend/internal/infrastructure/driven/context7"
	"github.com/anomalyco/codeauditor/backend/internal/infrastructure/driven/github"
	"github.com/anomalyco/codeauditor/backend/internal/infrastructure/driven/gogs"
	ollama "github.com/anomalyco/codeauditor/backend/internal/infrastructure/driven/ollama"
	sandboxpkg "github.com/anomalyco/codeauditor/backend/internal/infrastructure/driven/sandbox"
	"github.com/anomalyco/codeauditor/backend/internal/infrastructure/driven/sandbox/providers"
	"github.com/anomalyco/codeauditor/backend/internal/infrastructure/driven/supabase"
	authmiddleware "github.com/anomalyco/codeauditor/backend/internal/infrastructure/driving/authmiddleware"
	"github.com/anomalyco/codeauditor/backend/internal/infrastructure/driving/handlers"
	"github.com/anomalyco/codeauditor/backend/internal/mcpkit"
	"github.com/anomalyco/codeauditor/backend/internal/ports"
)

// main is the entry point for the CodeAuditor API server.
func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Load environment variables
	databaseURL := os.Getenv("DATABASE_URL")
	supabaseURL := os.Getenv("SUPABASE_URL")
	supabaseAnonKey := os.Getenv("SUPABASE_ANON_KEY")
	supabaseJWTsecret := os.Getenv("SUPABASE_JWT_SECRET")
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Validate required env vars
	if databaseURL == "" {
		log.Fatal("DATABASE_URL environment variable is required")
	}
	if supabaseURL == "" {
		log.Fatal("SUPABASE_URL environment variable is required")
	}
	if supabaseAnonKey == "" {
		log.Fatal("SUPABASE_ANON_KEY environment variable is required")
	}
	if supabaseJWTsecret == "" {
		log.Fatal("SUPABASE_JWT_SECRET environment variable is required")
	}

	// Determine sandbox mode
	sandboxMode := os.Getenv("SANDBOX_MODE")
	if sandboxMode == "" {
		sandboxMode = "auto"
	}

	// Initialize database connection
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Verify database connection
	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}
	log.Println("Connected to database")

	// Initialize Supabase client
	supabaseClient := supabase.NewSupabaseClient(supabaseURL, supabaseAnonKey)
	log.Println("Supabase client initialized")

	// Initialize JWT auth adapter
	authAdapter := supabase.NewSupabaseAuthAdapter(supabaseJWTsecret)
	log.Println("JWT auth adapter initialized")

	// Initialize the provider registry shared by both sandbox adapters.
	registry := providers.NewDefaultRegistry()
	log.Printf("Provider registry initialized: %d languages", len(registry.Languages()))

	// Initialize sandbox executor. dockerSb is always created so the
	// mcp-sandbox tool group can be wired (its run_code tool requires the
	// hardened DockerSandbox). When SANDBOX_MODE=local is forced, the
	// audit flow still uses LocalSandbox but mcp-sandbox.* tools report
	// the docker runtime as degraded via /mcp healthcheck.
	var dockerSb *sandboxpkg.DockerSandbox
	var sandboxExecutor ports.SandboxExecutor
	switch sandboxMode {
	case "docker":
		dockerSb = sandboxpkg.NewDockerSandbox(30*time.Second, registry)
		sandboxExecutor = dockerSb
		log.Printf("Sandbox mode: docker")
	case "local":
		dockerSb = sandboxpkg.NewDockerSandbox(30*time.Second, registry)
		sandboxExecutor = sandboxpkg.NewLocalSandbox(30*time.Second, registry)
		log.Printf("Sandbox mode: local (mcp-sandbox tools will report docker unavailable)")
	default: // auto
		dockerSb = sandboxpkg.NewDockerSandbox(30*time.Second, registry)
		if err := dockerSb.Healthcheck(context.Background()); err != nil {
			log.Printf("Docker unavailable, falling back to LocalSandbox: %v", err)
			sandboxExecutor = sandboxpkg.NewLocalSandbox(30*time.Second, registry)
		} else {
			sandboxExecutor = dockerSb
			log.Printf("Sandbox mode: docker (auto)")
		}
	}

	// Initialize audit service
	auditService := services.NewAuditService(sandboxExecutor)

	// Initialize Ollama client (optional — skip if env var not set)
	ollamaBaseURL := os.Getenv("OLLAMA_BASE_URL")
	ollamaModel := os.Getenv("OLLAMA_MODEL")
	if ollamaBaseURL != "" {
		ollamaClient := ollama.NewClient(ollamaBaseURL, ollamaModel)
		auditService.WithOllama(ollamaClient)
		log.Printf("Ollama client initialized (model: %s)", ollamaClient.Model())
	} else {
		log.Println("Ollama not configured — AI analysis disabled")
	}

	// Initialize user progress service (hexagonal: adapter implements the port)
	progressRepo := supabase.NewSupabaseProgressRepository(db)
	progressService := services.NewUserProgressService(progressRepo)
	auditService.WithProgress(progressService)
	log.Println("User progress tracking enabled")

	// Initialize audit history service
	historyRepo := supabase.NewSupabaseAuditHistoryRepository(db)
	historyService := services.NewAuditHistoryService(historyRepo)
	auditService.WithHistory(historyService)
	vaultHandler := handlers.NewVaultHandler(historyService)
	log.Println("Audit history tracking enabled")

	// Initialize challenge repo. The ChallengeService itself is
	// constructed BELOW (after curriculum + pedagogical + cascade are
	// wired) so the un-stubbed GenerateChallenge orchestration has all
	// its dependencies in place. The audit service's
	// WithChallengeService call moves with it.
	challengeRepo := supabase.NewSupabaseChallengeRepository(db)

	// Initialize dictionary service (personal tech glossary — S2)
	dictionaryRepo := supabase.NewSupabaseDictionaryRepository(db)
	dictionaryService := services.NewDictionaryService(dictionaryRepo)
	dictionaryHandler := handlers.NewDictionaryHandler(dictionaryService)
	log.Println("Dictionary service initialized")

	// Initialize the tutor lookup service (the bridge the tutor LLM uses
	// to look up user-defined terms before answering "Que es X?" style
	// questions). It depends ONLY on DictionaryService; the Socratic
	// level-aware framing is the dark logic it owns so it can be reused
	// by both the MCP tutor tool and the (future) HTTP tutor-chat
	// endpoint.
	tutorLookupService := services.NewTutorLookupService(dictionaryService)
	log.Println("Tutor lookup service initialized")

	// MCP tools registry. The full S2 inventory (33 tools across 8
	// groups — see BuildMCPRegistry for the group table) is wired here
	// and mounted on the HTTP router under /mcp/tools/* so external
	// callers (tutor LLM, scripted clients) can invoke any tool via
	// the JSON-RPC 2.0 transport. The HTTP bridge translates the
	// JWT-derived user_id from auth middleware into the MCP tool
	// context so user-scoped tools see the same user_id the REST
	// endpoints use — this is the bridge that preserves the dictionary
	// spec's Row-Level Security invariant end-to-end.
	//
	// 19 tools land in this PR (6 user + 4 curriculum + 6 pedagogical
	// + 3 external_docs); the other 14 were registered in prior PRs
	// (4 dictionary + 1 tutor + 4 sandbox + 5 challenges).
	//
	// User service — needed by mcp-user.get_profile, the only tool
	// in that group that reads the `usuarios` row directly. Other
	// mcp-user tools read through UserProgressService + AuditHistoryService.
	userProfileRepo := supabase.NewSupabaseUserProfileRepository(db)
	userService := services.NewUserService(userProfileRepo)
	log.Println("User profile service initialized")

	// Curriculum service — reads .atl/curricula/<lang>.md per
	// docs/curricula-format.md. The baseDir is resolved relative to the
	// current working directory so production (Coolify) and dev
	// (compose stack) both find the curricula without an env override.
	//
	// S3 PR 2 (free-practice-mode) added the CURRICULA_BASE_DIR env
	// override so production deployments can mount the curricula at a
	// read-only path (Coolify compose volume) without relying on the
	// cwd-relative default. The default `.atl/curricula` is unchanged
	// for backwards compatibility. Missing base dir in a fresh
	// checkout causes get_topics/get_topic to surface
	// ErrCurriculumLanguageNotFound at the tool layer (mapped to
	// JSON-RPC -32602 by the dispatcher); the service stays safe to
	// instantiate.
	curriculumBaseDir := os.Getenv("CURRICULA_BASE_DIR")
	if curriculumBaseDir == "" {
		curriculumBaseDir = filepath.Join(".", ".atl", "curricula")
	}
	curriculumService := services.NewCurriculumService(services.WithBaseDir(curriculumBaseDir))
	log.Printf("Curriculum service initialized (baseDir=%s)", curriculumService.Curriculum())

	// Pedagogical service — depends on the curriculum service so it
	// can read topic bodies when constructing learning objectives +
	// scaffolding steps. No external API yet (S5 wires the LLM
	// cascade); the Socratic prompt constructor is the deterministic
	// backbone.
	pedagogicalService := services.NewPedagogicalService(curriculumService)
	log.Println("Pedagogical service initialized")

	// External docs service — fuses library-docs (Context7) +
	// general web search (Brave) via the ports defined in
	// internal/ports/external_docs.go. API keys live ONLY in
	// .env / Coolify secrets; the MCP tools see rendered text.
	context7Client := context7.NewClient(
		os.Getenv("CONTEXT7_BASE_URL"),
		os.Getenv("CONTEXT7_API_KEY"),
	)
	braveClient := brave.NewClient(
		os.Getenv("BRAVE_BASE_URL"),
		os.Getenv("BRAVE_API_KEY"),
	)
	externalDocsService := services.NewExternalDocsService(context7Client, braveClient)
	log.Println("External docs service initialized (Context7 + Brave)")

	// Build the LLM cascade ONCE so both the tutor service AND the
	// un-stubbed ChallengeService.GenerateChallenge orchestration
	// share the same provider ranking (Groq → DeepSeek → MiniMax).
	// This is the PR 2 (free-practice-mode) rewire: previously the
	// cascade was constructed inside TutorSystem and was unreachable
	// from ChallengeService; PR 2 surfaces BuildTutorCascade so the
	// same `*llmcascade.Cascade` instance drives both flows. Empty
	// API keys mark providers disabled; the cascade silently skips
	// them.
	tutorProviderCfg := TutorProvidersConfig{
		GroqBaseURL:     "https://api.groq.com/openai/v1",
		GroqAPIKey:      os.Getenv("GROQ_API_KEY"),
		GroqModel:       os.Getenv("GROQ_MODEL"),
		DeepSeekBaseURL: "https://openrouter.ai/api/v1",
		DeepSeekAPIKey:  os.Getenv("OPENROUTER_API_KEY"),
		DeepSeekModel:   os.Getenv("DEEPSEEK_MODEL"),
		MiniMaxBaseURL:  os.Getenv("MINIMAX_BASE_URL"),
		MiniMaxAPIKey:   os.Getenv("MINIMAX_API_KEY"),
		MiniMaxModel:    os.Getenv("MINIMAX_MODEL"),
	}
	sharedCascade := BuildTutorCascade(tutorProviderCfg)

	// Challenge service — NOW wired with all four optional deps:
	// curriculum + pedagogical + LLM cascade + sandbox executor. The
	// un-stubbed GenerateChallenge orchestration lives behind these
	// option funcs; an unconfigured deployment (e.g. test with no API
	// keys) gets ErrChallengeGenerationNotWired on the wire.
	challengeService := services.NewChallengeService(
		challengeRepo,
		services.WithCurriculumService(curriculumService),
		services.WithPedagogicalService(pedagogicalService),
		services.WithLLMClient(sharedCascade),
		services.WithSandboxExecutor(sandboxExecutor),
	)
	challengeHandler := handlers.NewChallengeHandler(challengeService)
	auditService.WithChallengeService(challengeService)
	log.Println("Challenge service initialized (curriculum + pedagogical + LLM cascade + sandbox wired)")

	mcpRegistry := BuildMCPRegistry(WiringDeps{
		DockerSandbox:       dockerSb,
		ProviderRegistry:    registry,
		DictionaryService:   dictionaryService,
		ChallengeService:    challengeService,
		TutorLookupService:  tutorLookupService,
		UserService:         userService,
		UserProgressService: progressService,
		AuditHistoryService: historyService,
		CurriculumService:   curriculumService,
		PedagogicalService:  pedagogicalService,
		ExternalDocsService: externalDocsService,
	})
	mcpHTTPServer := mcpkit.NewServer(mcpRegistry)
	log.Printf("MCP monolith registered and mounted on /mcp (%d tools)", mcpRegistry.Count())

	// Initialize progress handler (user language progress + learning profile)
	progressHandler := handlers.NewProgressHandler(progressService)
	log.Println("Progress handler initialized")

	auditHandler := handlers.NewAuditHandler(auditService)
	log.Println("Audit service initialized")

	// Tutor chat system (PR 2 of the tutor-chat change): constructs
	// the TutorService + HTTP handler. The LLM cascade was built
	// earlier (sharedCascade) and is passed in via TutorSystemDeps so
	// the same `*llmcascade.Cascade` drives both the tutor chat path
	// AND the un-stubbed ChallengeService.GenerateChallenge. Reuses
	// the mcp-monolith user profile repository so the same DB row is
	// read by both the JSON-RPC and HTTP paths.
	progressRepoForTutor := supabase.NewSupabaseUserProfileRepository(db)
	tutorWiring, tutorWiringErr := TutorSystem(TutorSystemDeps{
		ProfileRepository: supabase.NewSupabaseProgressRepository(db),
		PromptsDir:        filepath.Join(".", ".atl", "prompts"),
		Providers:         tutorProviderCfg,
		Cascade:           sharedCascade,
	})
	if tutorWiringErr != nil {
		// The .atl/prompts tree is part of the production image, so
		// a missing prompt at process start is a fatal deploy bug —
		// not a runtime degradation. Failing fast keeps the
		// diagnostic surface tight.
		log.Fatalf("Tutor wiring failed: %v", tutorWiringErr)
	}
	tutorHandler := tutorWiring.Handler
	_ = progressRepoForTutor // keep the userProfile dep thread alive for the S5 cascade path
	log.Println("Tutor service initialized (shared LLM cascade + SSE handler)")

	// Initialize handlers
	authHandler := handlers.NewAuthHandler(supabaseClient, authAdapter, db)

	// Initialize Gogs client (optional — skip if env vars not set)
	var gogsHandler *handlers.GogsHandler
	gogsBaseURL := os.Getenv("GOGS_BASE_URL")
	gogsToken := os.Getenv("GOGS_TOKEN")
	if gogsBaseURL != "" && gogsToken != "" {
		gogsClient := gogs.NewGogsClient(gogsBaseURL, gogsToken)
		gogsHandler = handlers.NewGogsHandler(gogsClient)
		log.Println("Gogs client initialized")
	} else {
		log.Println("Gogs client not configured (GOGS_BASE_URL and GOGS_TOKEN not set)")
	}

	// Code health service (S4, PR 1 — backend foundation; PR 2
	// wires the MCP tools; PR 3 wires the dashboard). The
	// construction is env-var driven so the binary boots
	// without GITHUB_TOKEN (GitHub source is gated at the
	// SourceClient registry; Gogs is the always-on default).
	// See openspec/changes/code-health-dashboard/{proposal,design}.md.
	var codeHealthService *codehealth.CodeHealthService
	{
		codeHealthRepo := supabase.NewSupabaseCodeHealthRepository(db)
		analysisCache := supabase.NewSupabaseAnalysisCache(db)
		// Source clients keyed by source string. Gogs is
		// always-on if its env vars are set; GitHub is
		// gated on GITHUB_TOKEN.
		sourceClients := map[string]ports.SourceClient{}
		if gogsBaseURL != "" && gogsToken != "" {
			sourceClients["gogs"] = gogs.NewGogsClient(gogsBaseURL, gogsToken)
		}
		githubBaseURL := os.Getenv("GITHUB_BASE_URL")
		if githubBaseURL == "" {
			githubBaseURL = "https://api.github.com"
		}
		if token := os.Getenv("GITHUB_TOKEN"); token != "" {
			sourceClients["github"] = github.NewGitHubClient(githubBaseURL, token)
			log.Println("GitHub client initialized")
		} else {
			log.Println("GitHub client not configured (GITHUB_TOKEN not set)")
		}
		// multiSource dispatches by source string. The
		// analyzer's per-file pipeline uses a per-call
		// SourceClient (set via SetSource), so the service
		// updates the analyzer before each AnalyzeRepo.
		multiSource := &multiSourceClient{clients: sourceClients}
		registry := rules.NewDefaultRegistry()
		analyzer := codehealth.NewAnalyzer(codehealth.AnalyzerConfig{
			Source:      multiSource,
			Cache:       analysisCache,
			Registry:    registry,
			MaxFiles:    codehealth.DefaultMaxFiles,
			MaxFileSize: codehealth.DefaultMaxFileSize,
			// SuppressionChecker wired below once the
			// service is constructed (chicken-and-egg:
			// the service IS the suppression checker).
		})
		codeHealthService = codehealth.NewCodeHealthService(codeHealthRepo, analyzer)
		analyzer.SetSuppressionChecker(codeHealthService)
		log.Println("Code health service initialized (analyzer + cache + 6 rules)")
	}

	r := chi.NewRouter()

	// Global middleware (no global timeout — SSE endpoint handles its own timeout)
	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// Health check endpoint (no auth required)
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok","time":"` + time.Now().Format(time.RFC3339) + `"}`))
	})

	// Auth routes (public)
	r.Post("/auth/register", authHandler.Register)
	r.Post("/auth/login", authHandler.Login)

	// Protected auth routes
	r.Group(func(r chi.Router) {
		r.Use(authmiddleware.AuthMiddleware(authAdapter))
		r.Post("/auth/logout", authHandler.Logout)
		r.Get("/auth/me", authHandler.Me)
	})

	// MCP monolito HTTP transport: JSON-RPC 2.0 over HTTP, JWT-protected.
	// Mounted at /mcp so the dictionary spec's `POST /mcp/tools/call`
	// contract reaches the registered tools end-to-end. The MCP HTTP
	// server owns its own auth (passed in below) so we do NOT wrap the
	// parent group with AuthMiddleware here — that would double-validate
	// the JWT and reject requests whose Authorization header has any
	// whitespace quirk a single middleware would otherwise accept.
	r.Mount("/mcp", mcpHTTPServer.Router(authmiddleware.AuthMiddleware(authAdapter)))

	// API v1 routes (protected)
	r.Route("/api/v1", func(r chi.Router) {
		r.Use(authmiddleware.AuthMiddleware(authAdapter))
		// Audit SSE endpoint — no global timeout; runs its own context timeout
		r.Post("/audit", auditHandler.HandleSSE)
		// Gogs proxy endpoints
		if gogsHandler != nil {
			r.Get("/gogs/repos", gogsHandler.ListRepos)
			r.Post("/gogs/file", gogsHandler.GetFile)
		}
		// Vault / history endpoints
		r.Get("/audit/history", vaultHandler.GetHistory)
		r.Get("/audit/stats", vaultHandler.GetStats)
		// Challenge endpoints
		r.Get("/challenges", challengeHandler.ListChallenges)
		r.Get("/challenges/{id}", challengeHandler.GetChallenge)
		r.Post("/challenges", challengeHandler.CreateChallenge)
		// Dictionary endpoints (S2)
		r.Get("/dictionary", dictionaryHandler.ListOrSearchTerms)
		r.Post("/dictionary", dictionaryHandler.CreateTerm)
		r.Get("/dictionary/{id}", dictionaryHandler.GetTerm)
		r.Put("/dictionary/{id}", dictionaryHandler.UpdateTerm)
		r.Delete("/dictionary/{id}", dictionaryHandler.DeleteTerm)
		// User progress endpoints (JWT user_id must match path :id)
		r.Get("/users/{id}/progress", progressHandler.GetAllProgress)
		r.Get("/users/{id}/progress/{lang}", progressHandler.GetLanguageProgress)
		r.Put("/users/{id}/progress/{lang}", progressHandler.UpdateLanguageProgress)
		r.Get("/users/{id}/learning-profile", progressHandler.GetLearningProfile)
		r.Put("/users/{id}/learning-profile", progressHandler.UpdateLearningProfile)
		// Tutor chat endpoint (PR 2 of the tutor-chat change). Streams
		// SSE token/status/done/error frames per design.md §5. The
		// AuthMiddleware above enforces JWT presence so no inline
		// check is required in the handler — but the handler still
		// surfaces a 400 if the user_id stamp is missing (defense
		// in depth against wiring races).
		r.Post("/tutor/chat", tutorHandler.StreamChat)
	})

	addr := ":" + port
	srv := &http.Server{Addr: addr, Handler: r}

	go func() {
		log.Printf("CodeAuditor API server starting on %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("Shutting down server...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited gracefully")
}

// multiSourceClient dispatches ListFiles / GetFileContent to
// the right underlying client based on a "current source"
// string the analyzer (or the service) sets before each call.
// The MVP uses a flat string ("gogs" or "github") that the
// service stamps on the analysis row.
//
// A future PR can extend the contract to auto-detect from the
// repo descriptor (e.g. owner prefix "github.com/" -> GitHub).
type multiSourceClient struct {
	clients       map[string]ports.SourceClient
	currentSource string
}

// SetSource selects which underlying client subsequent
// ListFiles / GetFileContent calls will use. The analyzer's
// service calls this before each analyzer.AnalyzeRepo
// invocation via the WithSource option (not yet wired; PR 2).
func (m *multiSourceClient) SetSource(source string) {
	m.currentSource = source
}

// ListFiles dispatches to the client for the currently-set
// source. Returns an error if the source is unknown.
func (m *multiSourceClient) ListFiles(ctx context.Context, owner, repo, branch string) ([]ports.FileEntry, error) {
	client, ok := m.clients[m.currentSource]
	if !ok {
		return nil, fmt.Errorf("code-health: no source client for %q (set GITHUB_TOKEN or GOGS_BASE_URL+GOGS_TOKEN)", m.currentSource)
	}
	return client.ListFiles(ctx, owner, repo, branch)
}

// GetFileContent dispatches to the client for the
// currently-set source.
func (m *multiSourceClient) GetFileContent(ctx context.Context, owner, repo, branch, path string) (*ports.FileContent, error) {
	client, ok := m.clients[m.currentSource]
	if !ok {
		return nil, fmt.Errorf("code-health: no source client for %q", m.currentSource)
	}
	return client.GetFileContent(ctx, owner, repo, branch, path)
}
