.PHONY: build frontend deps run clean install uninstall

BINARY := mikrotik-monitor
FRONTEND_DIST := internal/api/static

PREFIX ?= /opt/mikrotik-monitor
DATADIR ?= /var/lib/mikrotik-monitor
CONFDIR ?= /etc/mikrotik-monitor
LISTEN ?= :8081
SYSTEMD_UNIT ?= /etc/systemd/system/mikrotik-monitor.service
SERVICE_USER ?= mikrotik-monitor

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
	./$(BINARY) -listen :8081 -db data.db

clean:
	rm -f $(BINARY)
	rm -rf $(FRONTEND_DIST)
	rm -rf frontend/dist frontend/node_modules

install: build
	install -d $(DESTDIR)$(PREFIX)
	install -m 755 $(BINARY) $(DESTDIR)$(PREFIX)/$(BINARY)
	install -d $(DESTDIR)$(DATADIR)
	install -d $(DESTDIR)$(CONFDIR)
	if [ ! -f $(DESTDIR)$(CONFDIR)/env ]; then \
		install -m 600 deploy/env.example $(DESTDIR)$(CONFDIR)/env; \
	fi
	sed -e 's|@PREFIX@|$(PREFIX)|g' \
		-e 's|@DATADIR@|$(DATADIR)|g' \
		-e 's|@CONFDIR@|$(CONFDIR)|g' \
		-e 's|@LISTEN@|$(LISTEN)|g' \
		deploy/mikrotik-monitor.service > $(DESTDIR)$(SYSTEMD_UNIT)
	if ! id $(SERVICE_USER) >/dev/null 2>&1; then \
		useradd --system --no-create-home --shell /usr/sbin/nologin $(SERVICE_USER); \
	fi
	chown $(SERVICE_USER):$(SERVICE_USER) $(DESTDIR)$(DATADIR)
	systemctl daemon-reload
	@echo "Installed $(PREFIX)/$(BINARY) and $(SYSTEMD_UNIT)"
	@echo "Edit $(CONFDIR)/env then run: systemctl enable --now mikrotik-monitor"

uninstall:
	systemctl stop mikrotik-monitor 2>/dev/null || true
	systemctl disable mikrotik-monitor 2>/dev/null || true
	rm -f $(DESTDIR)$(SYSTEMD_UNIT)
	rm -f $(DESTDIR)$(PREFIX)/$(BINARY)
	rmdir $(DESTDIR)$(PREFIX) 2>/dev/null || true
	systemctl daemon-reload
	@echo "Removed service and binary (data in $(DATADIR) and config in $(CONFDIR) kept)"
