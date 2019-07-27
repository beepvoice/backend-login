FROM golang:1.11-rc-alpine as build

RUN apk add --no-cache git=2.18.1-r0

WORKDIR /src
COPY go.mod go.sum .env *.go ./
RUN go get -d -v ./...
RUN CGO_ENABLED=0 go build -ldflags "-s -w"

FROM scratch

COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /src/login /login
COPY --from=build /src/.env /.env

ENTRYPOINT ["/login"]
