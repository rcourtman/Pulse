FROM golang:1.26.8-alpine@sha256:ce864e7223ac17b1775e6fd0b4c0db580c2eb50e7953a427916379e4b92a1628 AS builder

WORKDIR /build
COPY go.mod ./
RUN go mod download

COPY main.go ./
RUN CGO_ENABLED=0 GOOS=linux go build -o mock-github-server .

FROM alpine:3.24@sha256:294b683cb724975bec92580e1e685676bd4b50bda910ddb8c51d4cabeaec77e6
RUN apk --no-cache add ca-certificates

WORKDIR /app
COPY --from=builder /build/mock-github-server .

EXPOSE 8080

CMD ["./mock-github-server"]
