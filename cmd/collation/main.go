// Command collation is the entry point of the ancient-text collation and
// definitive-text workbench. It supports --smoke-test for the offline
// end-to-end self check used by the Docker build gates, and a long-running
// HTTP mode serving the JSON API and the collation UI.
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"task165-collation/internal/config"
	"task165-collation/internal/demo"
	"task165-collation/internal/httpapi"
	"task165-collation/internal/service"
	"task165-collation/internal/store"
)

func main() {
	cfg, err := config.Parse()
	if err != nil {
		log.Print(err)
		os.Exit(1)
	}
	if cfg.SmokeTest {
		res, err := demo.Seed(context.Background())
		if err != nil {
			log.Printf("smoke test failed: %v", err)
			os.Exit(1)
		}
		fmt.Printf("collation smoke test OK: project=%s witnesses=%d passages=%d anchors=%d variants=%d snapshots=%d persisted=%v\n",
			res.ProjectID, res.Witnesses, res.Passages, res.Anchors, res.Variants, res.Snapshots, res.Persisted)
		return
	}
	if err := serve(cfg); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}

func serve(cfg *config.Config) error {
	s, err := store.Open(cfg.DBPath)
	if err != nil {
		return err
	}
	defer s.Close()
	svc := service.New(s)
	if err := svc.Recover(context.Background()); err != nil {
		return err
	}
	api := httpapi.New(svc)
	log.Printf("collation service listening on %s (db=%s)", cfg.Addr, cfg.DBPath)
	return http.ListenAndServe(cfg.Addr, api)
}
