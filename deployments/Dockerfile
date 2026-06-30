FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -trimpath -ldflags="-s -w" -o /out/guardrail ./cmd/guardrail

FROM alpine:3.21
RUN adduser -D -H guardrail
WORKDIR /app
COPY --from=build /out/guardrail /usr/local/bin/guardrail
COPY configs/guardrail.yaml /app/configs/guardrail.yaml
USER guardrail
EXPOSE 8080
ENTRYPOINT ["guardrail"]
CMD ["-config", "/app/configs/guardrail.yaml"]
