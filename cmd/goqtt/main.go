package main

import (
	"context"
	"database/sql"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"gopkg.in/yaml.v3"

	"github.com/pyr33x/goqtt/internal/logger"
	"github.com/pyr33x/goqtt/internal/transport"
)

type Config struct {
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
	Server  Server `yaml:"server"`
}

type Server struct {
	Port        string `yaml:"port"`
	Environment string `yaml:"env"`
}

func gracefulShutdown(tcpServer *transport.TCPServer, cancel context.CancelFunc, done chan struct{}) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	<-ctx.Done()
	logger.Info("Graceful shutdown has triggered...")

	defer cancel()
	if err := tcpServer.Stop(); err != nil {
		logger.Error("Shutdown error", logger.String("error", err.Error()))
	}
	time.Sleep(1 * time.Second)

	close(done)
}

func main() {
	var cfg Config
	done := make(chan struct{}, 1)

	config, err := os.ReadFile("config.yml")
	if err != nil {
		logger.Fatal("failed to read config from yaml file")
		return
	}

	err = yaml.Unmarshal([]byte(config), &cfg)
	if err != nil {
		logger.Fatal("Failed to unmarshal yaml config", logger.String("error", err.Error()))
	}

	switch cfg.Server.Environment {
	case "production":
		logger.InitGlobalLogger(logger.ProductionConfig())
	case "development":
		logger.InitGlobalLogger(logger.DevelopmentConfig())
	default:
		logger.InitGlobalLogger(logger.DevelopmentConfig())
		logger.Warn("Invalid server environment config value, assigning default.")
	}

	if _, err := os.Stat("./store"); os.IsNotExist(err) {
		if err := os.Mkdir("./store", os.ModePerm); err != nil {
			logger.Fatal("Failed to create store directory", logger.String("error", err.Error()))
		}
	}

	db, err := sql.Open("sqlite3", filepath.Join("store", "store.db"))
	if err != nil {
		logger.Fatal("Failed to open sqlite db", logger.String("error", err.Error()))
	}

	if err := initSchema(db); err != nil {
		logger.Fatal("Failed to initialize schema", logger.String("error", err.Error()))
	}

	ctx, cancel := context.WithCancel(context.Background())

	srv := transport.New(cfg.Server.Port, db)

	go func() {
		if err := srv.Start(ctx); err != nil {
			logger.Fatal("server error", logger.String("error", err.Error()))
		}
	}()
	logger.Info("Server started listening", logger.String("port", cfg.Server.Port))

	go gracefulShutdown(srv, cancel, done)

	<-done
	logger.Info("Graceful shutdown complete.")
}

func initSchema(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS users (
		username TEXT PRIMARY KEY,
		secret TEXT NOT NULL
	);`
	_, err := db.Exec(schema)
	return err
}
