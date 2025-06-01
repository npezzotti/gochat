package main

import (
	"context"
	"encoding/base64"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/lib/pq"
	"github.com/npezzotti/go-chatroom/internal/api"
	"github.com/npezzotti/go-chatroom/internal/database"
	"github.com/npezzotti/go-chatroom/internal/server"
)

var (
	addr = flag.String("addr", "localhost:8000", "server address")
)

func decodeSigningSecret() ([]byte, error) {
	return base64.StdEncoding.DecodeString("wT0phFUusHZIrDhL9bUKPUhwaxKhpi/SaI6PtgB+MgU=")
}

func main() {
	logger := log.New(os.Stderr, "", 0)
	flag.Parse()

	var err error
	api.SecretKey, err = decodeSigningSecret()
	if err != nil {
		logger.Fatal("get signing secret: %w", err)
	}

	dbConn, err := database.NewDatabaseConnection("host=localhost user=postgres password=postgres dbname=postgres sslmode=disable")
	if err != nil {
		logger.Fatal("db open:", err)
	}

	defer func() {
		if err := dbConn.Close(); err != nil {
			logger.Fatal("db close:", err)
		}
	}()

	chatServer, err := server.NewChatServer(logger, dbConn)
	if err != nil {
		logger.Fatal("new chat server:", err)
	}

	srv := api.NewServer(*addr, logger, chatServer, dbConn)
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

	logger.Println("stopping server")

	shutDownCtx, cancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer cancel()

	if err := srv.Shutdown(shutDownCtx); err != nil {
		logger.Fatalln("shutdown:", err)
	}
	logger.Println("stopped server")

	logger.Println("shutting down chat server")
	chatServer.Shutdown()

	logger.Println("shutdown complete")
}
