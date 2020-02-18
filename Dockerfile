FROM golang:latest as build
WORKDIR /go/src/app
COPY . .
RUN go get -d -v ./...
RUN go install -v ./...

FROM debian:buster
COPY --from=build /go/bin/app /
ENTRYPOINT ["/app"]
