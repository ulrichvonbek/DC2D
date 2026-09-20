NAME    := dc2d
VNC_PW  ?= dc2d

.PHONY: help build run vnc stop start logs rm test local

help: ## Show available commands
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-12s\033[0m %s\n", $$1, $$2}'

build: ## Build the Docker image (pinned to Go 1.26.7)
	docker build -t $(NAME) .

run: build ## Build and run the game in a container
	docker rm -f $(NAME) >/dev/null 2>&1 || true
	docker run -d --name $(NAME) -p 5900:5900 -e VNC_PASSWORD=$(VNC_PW) $(NAME)
	@echo "Game running. Connect with: make vnc"

vnc: ## Open the macOS Screen Sharing viewer
	open vnc://localhost:5900

stop: ## Stop the container (game state is preserved)
	docker stop $(NAME)

start: ## Restart the stopped container
	docker start $(NAME)

logs: ## Follow the container logs
	docker logs -f $(NAME)

rm: ## Delete the container
	docker rm -f $(NAME)

test: ## Run the test suite with the host Go toolchain
	go test ./...

local: ## Run the game natively with the host Go toolchain
	go run .