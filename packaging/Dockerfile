FROM golang:1.27.3 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X main.version=$(tr -d '\r\n' < CONSOLE_VERSION)" -o /console ./cmd/greflow-console
RUN mkdir /helpers && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -o /helpers/greflow-agent-linux-amd64 ./cmd/greflow-agent && CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -o /helpers/greflow-agent-linux-arm64 ./cmd/greflow-agent && printf '{"greflow-agent-linux-amd64":"%s","greflow-agent-linux-arm64":"%s"}\n' "$(sha256sum /helpers/greflow-agent-linux-amd64 | cut -d ' ' -f1)" "$(sha256sum /helpers/greflow-agent-linux-arm64 | cut -d ' ' -f1)" > /helpers/manifest.json
FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && rm -rf /var/lib/apt/lists/* && useradd -u 10001 -m console
COPY --from=build /console /usr/local/bin/greflow-console
COPY --from=build /helpers /usr/local/bin/helpers
USER 10001:10001
EXPOSE 8787
ENTRYPOINT ["/usr/local/bin/greflow-console"]
