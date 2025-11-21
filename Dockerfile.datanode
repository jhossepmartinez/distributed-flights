FROM golang:1.24-alpine

WORKDIR /app

# Copiar dependencias
COPY go.mod go.sum ./
RUN go mod download

# Copiar código fuente
COPY . .

# Compilar
RUN go build -o server ./datanode/main.go

# Ejecutar
CMD ["./server"]
