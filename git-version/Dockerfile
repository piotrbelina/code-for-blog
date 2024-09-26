FROM golang:1.23 AS build-stage

ARG COMMIT
ARG BUILD_DATE

WORKDIR /app

COPY go.mod ./

RUN go mod download

COPY *.go ./

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-X main.GitCommit=$COMMIT -X main.BuildDate=$BUILD_DATE" -o /git-version


FROM alpine

WORKDIR /

COPY --from=build-stage /git-version /git-version

ENTRYPOINT ["/git-version"]