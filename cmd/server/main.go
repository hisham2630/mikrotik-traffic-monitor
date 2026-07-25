package main

import (
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"mikrotik-monitor/internal/alerter"
	"mikrotik-monitor/internal/api"
	"mikrotik-monitor/internal/auth"
	"mikrotik-monitor/internal/config"
	"mikrotik-monitor/internal/models"
	"mikrotik-monitor/internal/poller"
	"mikrotik-monitor/internal/rebootsched"
	"mikrotik-monitor/internal/wshub"
)

func main() {
	cfg := config.Load()

	db, err := models.Open(cfg.DBPath, cfg.SecretKey)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	samples := make(chan models.TrafficSample, 2048)
	alerterCh := make(chan models.TrafficSample, 512)
	writerCh := make(chan models.TrafficSample, 512)
	go func() {
		for s := range samples {
			sCopy := s
			select {
			case alerterCh <- sCopy:
			default:
			}
			select {
			case writerCh <- s:
			default:
			}
		}
	}()

	hub := wshub.New()
	pm := poller.New(db, hub, samples)
	pm.Start()
	poller.RunPruner(db)

	sw := poller.NewSampleWriter(db, writerCh)
	go sw.Run()

	ae := alerter.New(db, alerterCh)
	go ae.Run()

	rebootsched.Start(db, pm, ae)

	authMgr := auth.NewManager(db)
	go runSessionPruner(db)
	srv := &api.Server{DB: db, Auth: authMgr, Poller: pm, Alerter: ae}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/ws", authMgr.WSAuth(hub.HandleWS))
	r.Mount("/api", srv.Routes())
	r.Handle("/*", api.StaticHandler())

	srvHTTP := &http.Server{Addr: cfg.ListenAddr, Handler: r}
	ln, err := net.Listen("tcp", cfg.ListenAddr)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("listening on %s", cfg.ListenAddr)
	if err := srvHTTP.Serve(ln); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func runSessionPruner(db *models.DB) {
	ticker := time.NewTicker(time.Hour)
	for range ticker.C {
		if err := db.PruneExpiredSessions(); err != nil {
			log.Printf("session prune: %v", err)
		}
	}
}

func init() {
	if len(os.Args) > 0 {
		log.SetFlags(log.LstdFlags | log.Lshortfile)
	}
}
