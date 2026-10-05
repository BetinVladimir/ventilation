.PHONY: up down logs test firmware app
up:
	docker compose up -d --build
down:
	docker compose down
logs:
	docker compose logs -f
test:
	cd bakend && go test ./...
firmware:
	cd iots/damper && pio run
app:
	cd application && npm start
