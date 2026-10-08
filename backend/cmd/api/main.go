package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	_ "github.com/lib/pq"

	"github.com/anomalyco/codeauditor/backend/internal/core/services"
	"github.com/anomalyco/codeauditor/backend/internal/infrastructure/driven/gogs"
	ollama "github.com/anomalyco/codeauditor/backend/internal/infrastructure/driven/ollama"
	sandboxpkg "github.com/anomalyco/codeauditor/backend/internal/infrastructure/driven/sandbox"
	"github.com/anomalyco/codeauditor/backend/internal/infrastructure/driven/sandbox/providers"
	"github.com/anomalyco/codeauditor/backend/internal/infrastructure/driven/supabase"
	authmiddleware "github.com/anomalyco/codeauditor/backend/internal/infrastructure/driving/authmiddleware"
	"github.com/anomalyco/codeauditor/backend/internal/infrastructure/driving/handlers"
	dictionarymcp "github.com/anomalyco/codeauditor/backend/internal/infrastructure/mcp/dictionary"
	sandboxmcp "github.com/anomalyco/codeauditor/backend/internal/infrastructure/mcp/sandbox"
	tutormcp "github.com/anomalyco/codeauditor/backend/internal/infrastructure/mcp/tutor"
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
	historyService := services.NewAuditHistoryService(db)
	auditService.WithHistory(historyService)
	vaultHandler := handlers.NewVaultHandler(historyService)
	log.Println("Audit history tracking enabled")

	// Initialize challenge service
	challengeService := services.NewChallengeService(db)
	challengeHandler := handlers.NewChallengeHandler(challengeService)
	auditService.WithChallengeService(challengeService)
	log.Println("Challenge service initialized")

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

	// MCP tools registry. The dictionary group (search_term, get_term,
	// add_term, list_terms) AND the tutor group (tutor_lookup) are
	// registered here AND mounted on the HTTP router under /mcp/tools/*
	// so external callers (tutor LLM, scripted clients) can invoke them
	// via the JSON-RPC 2.0 transport the dictionary spec mandates. The
	// HTTP bridge translates the JWT-derived user_id from auth middleware
	// into the MCP tool context so user-scoped tools see the same
	// user_id the REST endpoints use — this is the bridge that preserves
	// the dictionary spec's Row-Level Security invariant end-to-end.
	mcpRegistry := mcpkit.NewRegistry()
	dictMCPSvc := dictionarymcp.NewServiceAdapter(dictionaryService)
	mcpRegistry.Register(dictionarymcp.NewSearchTermTool(dictMCPSvc))
	mcpRegistry.Register(dictionarymcp.NewGetTermTool(dictMCPSvc))
	mcpRegistry.Register(dictionarymcp.NewAddTermTool(dictMCPSvc))
	mcpRegistry.Register(dictionarymcp.NewListTermsTool(dictMCPSvc))
	tutorMCPSvc := tutormcp.NewServiceAdapter(tutorLookupService)
	mcpRegistry.Register(tutormcp.NewTutorLookupTool(tutorMCPSvc))
	// Sandbox group: 4 tools (run_code, list_languages, healthcheck,
	// get_install_hint). Wired to the DockerSandbox + ProviderRegistry
	// so run_code can enforce --cap-drop=ALL/--network=none/--read-only
	// even when the audit flow has fallen back to LocalSandbox.
	sandboxMCPSvc := sandboxmcp.NewServiceAdapter(dockerSb, registry)
	mcpRegistry.Register(sandboxmcp.NewRunCodeTool(sandboxMCPSvc))
	mcpRegistry.Register(sandboxmcp.NewListLanguagesTool(sandboxMCPSvc))
	mcpRegistry.Register(sandboxmcp.NewHealthcheckTool(sandboxMCPSvc))
	mcpRegistry.Register(sandboxmcp.NewGetInstallHintTool(sandboxMCPSvc))
	mcpHTTPServer := mcpkit.NewServer(mcpRegistry)
	log.Printf("MCP dictionary + tutor + sandbox groups registered and mounted on /mcp (%d tools)", mcpRegistry.Count())

	// Initialize progress handler (user language progress + learning profile)
	progressHandler := handlers.NewProgressHandler(progressService)
	log.Println("Progress handler initialized")

	auditHandler := handlers.NewAuditHandler(auditService)
	log.Println("Audit service initialized")

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
