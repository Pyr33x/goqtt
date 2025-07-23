package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/pyr33x/envy"

	"github.com/pyr33x/goqtt/internal/broker"
	"github.com/pyr33x/goqtt/internal/transport"
)

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
	done := make(chan struct{})
	port := envy.GetString("PORT", "1883")

	ctx, cancel := context.WithCancel(context.Background())

	broker := broker.New()
	srv := transport.New(port, broker)

	go func() {
		if err := srv.Start(ctx); err != nil {
			log.Fatalf("server error: %v", err)
		}
	}()
	log.Printf("Server started listening at %s\n", port)

	go gracefulShutdown(cancel, done)

	<-done
	log.Println("Graceful shutdown complete.")
}
