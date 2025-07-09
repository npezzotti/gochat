package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	_ "github.com/lib/pq"
	"github.com/npezzotti/go-chatroom/internal/api"
	"github.com/npezzotti/go-chatroom/internal/config"
	"github.com/npezzotti/go-chatroom/internal/database"
	"github.com/npezzotti/go-chatroom/internal/server"
	"github.com/npezzotti/go-chatroom/internal/stats"
)

const defaultSigningKey = "wT0phFUusHZIrDhL9bUKPUhwaxKhpi/SaI6PtgB+MgU="

type stringSliceFlag []string

func (s *stringSliceFlag) String() string {
	return strings.Join(*s, ",")
}

func (s *stringSliceFlag) Set(value string) error {
	*s = append(*s, strings.Split(value, ",")...)
	return nil
}

var (
	addr           string
	dsn            string
	signingKey     string
	allowedOrigins stringSliceFlag
)

func main() {
	flag.StringVar(&addr, "addr", "localhost:8000", "server address")
	flag.StringVar(&dsn, "dsn", "host=localhost user=postgres password=postgres dbname=postgres sslmode=disable", "database connection string")
	flag.StringVar(&signingKey, "signing-key", defaultSigningKey, "base64 encoded signing key")
	flag.Var(&allowedOrigins, "allowed-origins", "comma-separated list of allowed origins for CORS")
	flag.Parse()

	logger := log.New(os.Stderr, "[go-chat] ", log.LstdFlags)

	cfg, err := config.NewConfig(addr, dsn, signingKey, allowedOrigins)
	if err != nil {
		logger.Fatal("config:", err)
	}

	dbConn, err := database.NewPgGoChatRepository(cfg.DatabaseDSN)
	if err != nil {
		logger.Fatal("db open:", err)
	}
	defer func() {
		if err := dbConn.Close(); err != nil {
			logger.Fatal("db close:", err)
		}
	}()

	mux := http.NewServeMux()

	statsUpdater := stats.NewStatsUpdater(mux)

	chatServer, err := server.NewChatServer(logger, dbConn, statsUpdater)
	if err != nil {
		logger.Fatal("new chat server:", err)
	}

	srv := api.NewGoChatApp(mux, logger, chatServer, dbConn, statsUpdater, cfg)

	statsUpdater.Run()
	defer statsUpdater.Stop()

	go chatServer.Run()

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Start()
	}()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigs:
		logger.Printf("received signal: %s\n", sig)
	case err := <-errCh:
		logger.Println("server:", err)
	}

	shutDownCtx, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer cancel()

	if err := srv.Shutdown(shutDownCtx); err != nil {
		logger.Fatalln("HTTP server shutdown:", err)
	}

	logger.Println("shutting down chat server...")
	if err := chatServer.Shutdown(shutDownCtx); err != nil {
		logger.Fatalln("chat server shutdown:", err)
	}

	logger.Println("shutdown complete")
}
