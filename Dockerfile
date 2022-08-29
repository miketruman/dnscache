FROM golang:1.17 as builder

WORKDIR /app/
COPY go.sum .
COPY go.mod .
COPY main.go .
COPY main_test.go .
RUN go test
RUN CGO_ENABLED=0 go build -o /main

FROM scratch
COPY --from=builder /main /main
ENTRYPOINT ["/main"]
