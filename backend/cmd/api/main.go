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
	"github.com/anomalyco/codeauditor/backend/internal/ports"
	"github.com/anomalyco/codeauditor/backend/internal/infrastructure/driven/gogs"
	ollama "github.com/anomalyco/codeauditor/backend/internal/infrastructure/driven/ollama"
	sandboxpkg "github.com/anomalyco/codeauditor/backend/internal/infrastructure/driven/sandbox"
	"github.com/anomalyco/codeauditor/backend/internal/infrastructure/driven/supabase"
	"github.com/anomalyco/codeauditor/backend/internal/infrastructure/driving/handlers"
	authmiddleware "github.com/anomalyco/codeauditor/backend/internal/infrastructure/driving/authmiddleware"
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

	// Initialize sandbox executor
	var sandboxExecutor ports.SandboxExecutor
	switch sandboxMode {
	case "docker":
		sandboxExecutor = sandboxpkg.NewDockerSandbox(30 * time.Second)
		log.Printf("Sandbox mode: docker")
	case "local":
		sandboxExecutor = sandboxpkg.NewLocalSandbox(30 * time.Second)
		log.Printf("Sandbox mode: local")
	default: // auto
		dockerSb := sandboxpkg.NewDockerSandbox(30 * time.Second)
		if err := dockerSb.Healthcheck(context.Background()); err != nil {
			log.Printf("Docker unavailable, falling back to LocalSandbox: %v", err)
			sandboxExecutor = sandboxpkg.NewLocalSandbox(30 * time.Second)
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
		w.Write([]byte(`{"status":"ok","time":"` + time.Now().Format(time.RFC3339) + `"}`))
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