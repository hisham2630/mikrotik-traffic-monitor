.PHONY: build frontend deps run clean

BINARY := mikrotik-monitor
FRONTEND_DIST := internal/api/static

frontend:
	cd frontend && npm install && npm run build
	rm -rf $(FRONTEND_DIST)
	mkdir -p $(FRONTEND_DIST)
	cp -r frontend/dist/* $(FRONTEND_DIST)/

deps:
	go mod tidy
	cd frontend && npm install

build: frontend
	go build -o $(BINARY) ./cmd/server

run: build
	./$(BINARY) -listen :8080 -db data.db

clean:
	rm -f $(BINARY)
	rm -rf $(FRONTEND_DIST)
	rm -rf frontend/dist frontend/node_modules
