FROM golang:1.21-alpine AS build

ARG SERVICE
WORKDIR /src

COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal

RUN test -n "$SERVICE"
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/service ./cmd/${SERVICE}

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /out/service /service
EXPOSE 8081 8082
ENTRYPOINT ["/service"]
