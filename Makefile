ARGS=$(filter-out $@,$(MAKECMDGOALS))

.PHONY: producer
producer: 
	docker compose exec kafka-local kafka-console-producer.sh --bootstrap-server kafka-local:9092 --topic topic

migrate-up:
	migrate -path ./internal/migration/schema -database 'postgres://postgres:1@0.0.0.0:5432/postgres?sslmode=disable' up
migrate-down:
	migrate -path ./internal/migration/schema -database 'postgres://postgres:1@0.0.0.0:5432/postgres?sslmode=disable' down	
topics:
	docker exec -it kafka-local kafka-topics.sh --bootstrap-server kafka-local:9092 --list
messages:
	docker exec -it kafka-local kafka-console-consumer.sh --bootstrap-server kafka-local:9092 --topic topic --from-beginning
topic:
	kafka-topics.sh --create --topic topic --bootstrap-server kafka-local:9092
testAll:
	go test -v -cover -coverpkg ./... ./...
dockerRun:
	docker build -t app . && docker compose up -d
dockerClear:
	docker compose down -v && docker rmi app