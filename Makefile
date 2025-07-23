all: build start

build:
	@echo "🔨 Building..."
	@go build -o ./bin/goqtt ./cmd/goqtt/main.go

run:
	@echo "⚙️ Running..."
	@go run ./cmd/goqtt/main.go

start:
	@echo "🛠️ Starting..."
	@./bin/goqtt


watch:
	@if command -v air > /dev/null; then \
            air; \
            echo "👀 Watching...";\
        else \
            read -p "Go's 'air' is not installed on your machine. Do you want to install it? [Y/n] " choice; \
            if [ "$$choice" != "n" ] && [ "$$choice" != "N" ]; then \
                go install github.com/air-verse/air@latest; \
                air; \
                echo "👀 Watching...";\
            else \
                echo "Exiting..."; \
                exit 1; \
            fi; \
        fi
