# The build context is this example's own directory. It consumes the published
# provider module from the proxy, so the build needs no parent directories.
FROM golang:1.26-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -o /out/blog .

FROM alpine:3.20
RUN adduser -D -u 10001 app
COPY --from=build /out/blog /usr/local/bin/blog
USER app
EXPOSE 8080
ENTRYPOINT ["blog"]
