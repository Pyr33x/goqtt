package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/pyr33x/goqtt/internal/broker"
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

func gracefulShutdown(cancel context.CancelFunc, done chan struct{}) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	<-ctx.Done()
	log.Println("Graceful shutdown has triggered...")

	cancel()
	time.Sleep(1 * time.Second)

	close(done)
}

func main() {
	done := make(chan struct{}, 1)
	var cfg Config

	data, _ := os.ReadFile("config.yml")

	err := yaml.Unmarshal([]byte(data), &cfg)
	if err != nil {
		log.Panicf("Failed to unmarshal yaml config: %v\n", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	broker := broker.New()
	srv := transport.New(cfg.Server.Port, broker)

	go func() {
		if err := srv.Start(ctx); err != nil {
			log.Fatalf("server error: %v", err)
		}
	}()
	log.Printf("Server started listening at %s\n", cfg.Server.Port)

	go gracefulShutdown(cancel, done)

	<-done
	log.Println("Graceful shutdown complete.")
}
