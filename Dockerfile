FROM golang:1.23.4-alpine AS build-stage

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /intellicord cmd/main.go

FROM gcr.io/distroless/base-debian11 AS build-release-stage

WORKDIR /

COPY --from=build-stage /intellicord /intellicord

EXPOSE 8080

USER nonroot:nonroot

ENTRYPOINT [ "/intellicord" ]