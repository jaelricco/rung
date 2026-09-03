package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"calisthenics/api/internal/ai"
	"calisthenics/api/internal/auth"
	"calisthenics/api/internal/config"
	"calisthenics/api/internal/db"
	"calisthenics/api/internal/events"
	"calisthenics/api/internal/httpx"
	"calisthenics/api/internal/parks"
	"calisthenics/api/internal/plan"
	"calisthenics/api/internal/secret"
	"calisthenics/api/internal/training"
)

func main() {
	log.SetFlags(log.LstdFlags | log.LUTC)
	cfg := config.Load()

	if cfg.DatabaseURL == "" {
		log.Fatal("DATABASE_URL is not set")
	}

	rootCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	startupCtx, cancel := context.WithTimeout(rootCtx, 90*time.Second)
	defer cancel()

	pool, err := db.Connect(startupCtx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer pool.Close()

	if err := db.Migrate(startupCtx, pool); err != nil {
		log.Fatalf("migrations: %v", err)
	}

	authSvc := auth.New(pool, cfg.SecureCookies, auth.OAuthConfig{
		GoogleClientID:      cfg.GoogleClientID,
		GoogleClientSecret:  cfg.GoogleClientSecret,
		GoogleIssuer:        cfg.GoogleIssuer,
		ChatGPTClientID:     cfg.ChatGPTClientID,
		ChatGPTClientSecret: cfg.ChatGPTClientSecret,
		ChatGPTIssuer:       cfg.ChatGPTIssuer,
		AppOrigin:           cfg.AppOrigin,
	})
	trainingSvc := training.New(pool)
	parksSvc := parks.New(pool)
	// Athletes bring their own model accounts. The server holds no model key of
	// its own; all it needs is the secret that seals theirs, and it makes one
	// for itself when nobody has handed it one.
	keystore, generated, err := secret.Keystore(startupCtx, pool, cfg.CredentialsKey)
	if err != nil {
		log.Fatalf("sealing key: %v", err)
	}
	if generated {
		log.Print("sealing athletes' provider keys with a key this server generated for itself; " +
			"set AI_CREDENTIALS_KEY to keep it out of the database instead")
	}

	aiStore := ai.NewStore(pool, keystore, ai.Settings{
		SearchToolVersion: cfg.SearchToolVersion,
		Thinking:          cfg.AIThinking,
	})
	aiHandler := ai.NewHandler(aiStore, pool, trainingSvc)
	// The deterministic planner. It shares nothing with the AI handler except
	// the plan schema, and it needs no model account of any kind — which is
	// the point: it is what answers when everything else cannot.
	planHandler := plan.NewHandler(pool, trainingSvc)
	eventsSvc := events.New(pool, aiStore)

	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		if err := pool.Ping(r.Context()); err != nil {
			httpx.Fail(w, http.StatusServiceUnavailable, "database unreachable")
			return
		}
		httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	// Public
	mux.HandleFunc("POST /api/v1/auth/register", authSvc.Register)
	mux.HandleFunc("POST /api/v1/auth/login", authSvc.Login)
	mux.HandleFunc("POST /api/v1/auth/logout", authSvc.Logout)
	// Sign-in with a provider: two browser redirects, so both are plain GETs
	// and neither can be a fetch from the app.
	mux.HandleFunc("GET /api/v1/auth/providers", authSvc.Providers)
	mux.HandleFunc("GET /api/v1/auth/oauth/{provider}/start", authSvc.OAuthStart)
	mux.HandleFunc("GET /api/v1/auth/oauth/{provider}/callback", authSvc.OAuthCallback)
	mux.HandleFunc("GET /api/v1/exercises", trainingSvc.ListExercises)
	mux.HandleFunc("GET /api/v1/protocols", trainingSvc.ListProtocols)
	mux.HandleFunc("GET /api/v1/parks", parksSvc.Nearby)

	// Authenticated
	protected := map[string]http.HandlerFunc{
		"GET /api/v1/me":                        authSvc.Me,
		"PATCH /api/v1/me":                      authSvc.UpdateProfile,
		"PUT /api/v1/me/password":               authSvc.SetPassword,
		"GET /api/v1/me/logins":                 authSvc.LoginMethods,
		"DELETE /api/v1/me/logins/{provider}":   authSvc.Unlink,
		"GET /api/v1/me/ai":                     aiHandler.Connection,
		"GET /api/v1/me/ai/usage":               aiHandler.Usage,
		"PUT /api/v1/me/ai":                     aiHandler.Connect,
		"DELETE /api/v1/me/ai":                  aiHandler.Disconnect,
		"GET /api/v1/workouts":                  trainingSvc.ListWorkouts,
		"POST /api/v1/workouts":                 trainingSvc.CreateWorkout,
		"DELETE /api/v1/workouts/{id}":          trainingSvc.DeleteWorkout,
		"GET /api/v1/level":                     trainingSvc.Level,
		"GET /api/v1/baseline":                  trainingSvc.GetBaseline,
		"PUT /api/v1/baseline":                  trainingSvc.PutBaseline,
		"GET /api/v1/plan/benchmarks":           planHandler.Benchmarks,
		"GET /api/v1/injuries":                  trainingSvc.ListInjuries,
		"POST /api/v1/injuries":                 trainingSvc.CreateInjury,
		"POST /api/v1/injuries/{id}/resolve":    trainingSvc.ResolveInjury,
		"GET /api/v1/calendar":                  trainingSvc.Calendar,
		"GET /api/v1/calendar.ics":              trainingSvc.CalendarICS,
		"POST /api/v1/sessions":                 trainingSvc.CreateSession,
		"PATCH /api/v1/sessions/{id}":           trainingSvc.UpdateSession,
		"DELETE /api/v1/sessions/{id}":          trainingSvc.DeleteSession,
		"POST /api/v1/sessions/{id}/complete":   trainingSvc.CompleteSession,
		"DELETE /api/v1/sessions/{id}/complete": trainingSvc.UncompleteSession,
		"GET /api/v1/routines":                  trainingSvc.ListRoutines,
		"POST /api/v1/routines":                 trainingSvc.CreateRoutine,
		"PATCH /api/v1/routines/{id}":           trainingSvc.UpdateRoutine,
		"DELETE /api/v1/routines/{id}":          trainingSvc.DeleteRoutine,
		"POST /api/v1/routines/{id}/apply":      trainingSvc.ApplyRoutine,
		"GET /api/v1/plans":                     trainingSvc.ListPlans,
		"POST /api/v1/plans":                    planHandler.SavePlan,
		"POST /api/v1/plans/generate":           planHandler.Generate,
		"DELETE /api/v1/plans/{id}":             trainingSvc.DeletePlan,
		"POST /api/v1/ai/skill-plan":            aiHandler.SkillPlan,
		"POST /api/v1/ai/review":                aiHandler.Review,
		"POST /api/v1/ai/recovery":              aiHandler.Recovery,
		"POST /api/v1/parks/refresh":            parksSvc.Refresh,
		"GET /api/v1/events":                    eventsSvc.List,
		"POST /api/v1/events/discover":          eventsSvc.Discover,
		"POST /api/v1/events/{id}/confirm":      eventsSvc.Confirm,
		"POST /api/v1/events/{id}/recheck":      eventsSvc.RecheckOne,
		"POST /api/v1/events/{id}/register":     eventsSvc.Register,
		"DELETE /api/v1/events/{id}/register":   eventsSvc.Unregister,
	}
	for pattern, handler := range protected {
		mux.Handle(pattern, authSvc.Required(handler))
	}

	handler := httpx.Logging(httpx.Recover(mux))

	server := &http.Server{
		Addr:              cfg.Addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		// Generous: plan generation waits on the model.
		WriteTimeout: 180 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		ticker := time.NewTicker(6 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-rootCtx.Done():
				return
			case <-ticker.C:
				authSvc.PurgeExpiredSessions(rootCtx)
				// Event pages move dates and disappear. Re-verify a slice of
				// the upcoming ones on every pass.
				eventsSvc.RecheckStale(rootCtx, 25)
			}
		}
	}()

	go func() {
		log.Printf("listening on %s", cfg.Addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server: %v", err)
		}
	}()

	<-rootCtx.Done()
	log.Print("shutting down")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer shutdownCancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown: %v", err)
	}
}
