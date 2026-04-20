FROM golang:1.24-bookworm AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 go build -o resumectl .

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y ca-certificates curl xz-utils && \
    curl -fsSL https://github.com/tectonic-typesetting/tectonic/releases/download/tectonic%400.14.1/tectonic-0.14.1-x86_64-unknown-linux-musl.tar.gz | tar xz -C /usr/local/bin/ && \
    chmod +x /usr/local/bin/tectonic && \
    rm -rf /var/lib/apt/lists/*
COPY --from=build /app/resumectl /usr/local/bin/
COPY resume.cls /root/.resumectl/resume.cls
COPY resume.template*.tex /root/.resumectl/
RUN mkdir -p /data
CMD ["resumectl", "serve", "--port", "8080"]
