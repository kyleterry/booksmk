FROM golang:1.26-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go tool templ generate
RUN CGO_ENABLED=0 go build -o /out/booksmk ./cmd/booksmk
RUN CGO_ENABLED=0 go build -o /out/booksmkctl ./cmd/booksmkctl

FROM alpine:3.21

RUN apk add --no-cache ca-certificates

COPY --from=build /out/booksmk /usr/local/bin/booksmk
COPY --from=build /out/booksmkctl /usr/local/bin/booksmkctl

ENTRYPOINT ["booksmk"]
