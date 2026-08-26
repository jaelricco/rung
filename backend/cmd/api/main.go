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

	authSvc := auth.New(pool, cfg.SecureCookies)
	trainingSvc := training.New(pool)
	parksSvc := parks.New(pool)
	aiClient := ai.NewClient(pool, cfg.AnthropicKey, cfg.AnthropicModel, cfg.SearchToolVersion, cfg.AnthropicThinking)
	aiHandler := ai.NewHandler(aiClient, pool, trainingSvc)
	eventsSvc := events.New(pool, aiClient)

	if !aiClient.Configured() {
		log.Print("warning: ANTHROPIC_API_KEY is empty, coaching endpoints will return 503")
	}

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
	mux.HandleFunc("GET /api/v1/exercises", trainingSvc.ListExercises)
	mux.HandleFunc("GET /api/v1/protocols", trainingSvc.ListProtocols)
	mux.HandleFunc("GET /api/v1/parks", parksSvc.Nearby)

	// Authenticated
	protected := map[string]http.HandlerFunc{
		"GET /api/v1/me":                        authSvc.Me,
		"PATCH /api/v1/me":                      authSvc.UpdateProfile,
		"GET /api/v1/workouts":                  trainingSvc.ListWorkouts,
		"POST /api/v1/workouts":                 trainingSvc.CreateWorkout,
		"DELETE /api/v1/workouts/{id}":          trainingSvc.DeleteWorkout,
		"GET /api/v1/level":                     trainingSvc.Level,
		"GET /api/v1/injuries":                  trainingSvc.ListInjuries,
		"POST /api/v1/injuries":                 trainingSvc.CreateInjury,
		"POST /api/v1/injuries/{id}/resolve":    trainingSvc.ResolveInjury,
		"GET /api/v1/calendar":                  trainingSvc.Calendar,
		"GET /api/v1/calendar.ics":              trainingSvc.CalendarICS,
		"POST /api/v1/sessions/{id}/complete":   trainingSvc.CompleteSession,
		"DELETE /api/v1/sessions/{id}/complete": trainingSvc.UncompleteSession,
		"GET /api/v1/plans":                     trainingSvc.ListPlans,
		"POST /api/v1/plans":                    aiHandler.SavePlan,
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
