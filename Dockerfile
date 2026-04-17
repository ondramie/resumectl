FROM golang:1.24-bookworm AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 go build -o resumectl .

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y ca-certificates && rm -rf /var/lib/apt/lists/*
COPY --from=build /app/resumectl /usr/local/bin/
COPY resume.cls /root/.resumectl/resume.cls
RUN mkdir -p /data
CMD ["resumectl", "serve", "--port", "8080"]
