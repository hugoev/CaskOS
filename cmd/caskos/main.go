package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/caskos/caskos/internal/api"
	"github.com/caskos/caskos/internal/hashring"
	"github.com/caskos/caskos/internal/metadata"
	"github.com/caskos/caskos/internal/storage"
)

const (
	defaultPort         = "8080"
	defaultReplication  = 2
	defaultVirtualNodes = 150
)

func main() {
	// Parse command line flags
	port := flag.String("port", defaultPort, "HTTP server port")
	dataDir := flag.String("data-dir", "./data", "Base directory for data storage")
	metadataDir := flag.String("metadata-dir", "./metadata", "Directory for metadata storage")
	nodeCount := flag.Int("nodes", 3, "Number of storage nodes")
	replication := flag.Int("replication", defaultReplication, "Replication factor")
	virtualNodes := flag.Int("virtual-nodes", defaultVirtualNodes, "Number of virtual nodes per physical node")
	flag.Parse()

	// Setup structured logging
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	logger.Info("starting CaskOS", "port", *port, "nodes", *nodeCount, "replication", *replication)

	// Create metadata store
	metadataStore, err := metadata.NewStore(*metadataDir)
	if err != nil {
		logger.Error("failed to create metadata store", "error", err)
		os.Exit(1)
	}

	// Create hash ring
	ring := hashring.NewHashRing(*virtualNodes)

	// Create storage nodes
	storageManager := storage.NewManager(ring, *replication, logger)
	for i := 0; i < *nodeCount; i++ {
		nodeID := fmt.Sprintf("node%d", i+1)
		nodePath := filepath.Join(*dataDir, nodeID)

		node, err := storage.NewNode(nodeID, nodePath)
		if err != nil {
			logger.Error("failed to create storage node", "node_id", nodeID, "error", err)
			os.Exit(1)
		}

		ring.AddNode(nodeID)
		storageManager.AddNode(nodeID, node)
		logger.Info("created storage node", "node_id", nodeID, "path", nodePath)
	}

	// Create API server
	server := api.NewServer(storageManager, metadataStore, logger, *replication)

	// Setup HTTP routes
	mux := http.NewServeMux()

	// API endpoints
	mux.HandleFunc("POST /upload", server.UploadHandler)
	mux.HandleFunc("GET /object/{id}", server.GetObjectHandler)
	mux.HandleFunc("GET /metadata/{id}", server.GetMetadataHandler)

	// Health check endpoint
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Serve static files for web UI (must be before root handler)
	fs := http.FileServer(http.Dir("web/static"))
	mux.Handle("/static/", http.StripPrefix("/static/", fs))

	// Serve index.html for root path (catch-all for unmatched routes)
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "web/static/index.html")
	})

	// Start HTTP server
	addr := fmt.Sprintf(":%s", *port)
	httpServer := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	// Graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		logger.Info("server starting", "address", addr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	<-sigChan
	logger.Info("shutting down server")
	if err := httpServer.Shutdown(context.Background()); err != nil {
		logger.Error("error shutting down server", "error", err)
	}
}
