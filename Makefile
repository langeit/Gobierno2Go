.PHONY: all build run test clean docker-up docker-down

all: build

build:
	@echo "Compilando la API de Gobierno2Go..."
	go build -o bin/Gobierno2Go cmd/api/main.go

run: build
	@echo "Ejecutando Gobierno2Go..."
	./bin/Gobierno2Go

test:
	@echo "Ejecutando pruebas..."
	go test ./... -v

docker-up:
	@echo "Iniciando servicios Docker..."
	docker compose up -d

docker-down:
	@echo "Deteniendo servicios Docker..."
	docker compose down

clean:
	@echo "Limpiando binarios..."
	rm -rf bin/
