FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -trimpath -ldflags="-s -w" -o /out/guardrail ./cmd/guardrail

FROM alpine:3.21
RUN addgroup -g 10001 guardrail && adduser -D -H -u 10001 -G guardrail guardrail \
    && mkdir -p /data && chown 10001:10001 /data
WORKDIR /app
COPY --from=build /out/guardrail /usr/local/bin/guardrail
COPY configs/guardrail.yaml /app/configs/guardrail.yaml
ENV GUARDRAIL_AUDIT_SQLITE_DSN="file:/data/audit.db?_pragma=busy_timeout(5000)"
USER guardrail
EXPOSE 8080
ENTRYPOINT ["guardrail"]
CMD ["-config", "/app/configs/guardrail.yaml"]
