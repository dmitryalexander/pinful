FROM golang:1.25.6-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /pinful .

FROM build AS test
RUN apk add --no-cache ffmpeg
RUN go test ./...

FROM alpine:3.23
RUN apk add --no-cache ffmpeg ca-certificates
COPY --from=build /pinful /usr/local/bin/pinful
EXPOSE 3000
CMD ["pinful"]
