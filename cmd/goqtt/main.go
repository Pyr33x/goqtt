package main

import (
	"context"
	"database/sql"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"gopkg.in/yaml.v3"

	"github.com/pyr33x/goqtt/internal/transport"
)

type Config struct {
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
	Server  Server `yaml:"server"`
}

type Server struct {
	Port string `yaml:"port"`
}

func gracefulShutdown(tcpServer *transport.TCPServer, cancel context.CancelFunc, done chan struct{}) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	<-ctx.Done()
	log.Println("Graceful shutdown has triggered...")

	defer cancel()
	if err := tcpServer.Stop(); err != nil {
		log.Println(err)
	}
	time.Sleep(1 * time.Second)

	close(done)
}

func main() {
	done := make(chan struct{}, 1)
	var cfg Config

	config, err := os.ReadFile("config.yml")
	if err != nil {
		log.Panicln("failed to read config from yaml file")
		return
	}

	err = yaml.Unmarshal([]byte(config), &cfg)
	if err != nil {
		log.Panicf("Failed to unmarshal yaml config: %v\n", err)
	}

	db, err := sql.Open("sqlite3", "./store/store.db")
	if err != nil {
		log.Panicf("Failed to open sqlite db: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	srv := transport.New(cfg.Server.Port, db)

	go func() {
		if err := srv.Start(ctx); err != nil {
			log.Fatalf("server error: %v", err)
		}
	}()
	log.Printf("Server started listening at %s\n", cfg.Server.Port)

	go gracefulShutdown(srv, cancel, done)

	<-done
	log.Println("Graceful shutdown complete.")
}
