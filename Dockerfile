FROM golang:1.22.3 AS builder

WORKDIR /app/

COPY go.* ./

RUN go mod download

COPY cmd/main.go cmd/main.go
COPY config/ config/
COPY internal/ internal/
COPY pkg/ pkg/

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o vault-autounseal cmd/main.go

FROM alpine:3.20.2
WORKDIR /app
COPY --from=builder /app/vault-autounseal .

EXPOSE 8080
ENTRYPOINT ["/app/vault-autounseal"]




