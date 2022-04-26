FROM docker.io/library/golang:1.18 AS builder

COPY src /minitwit
WORKDIR /minitwit/app

RUN go mod download
RUN GOOS=linux CGO_ENABLED=1 go build -o app .

FROM docker.io/library/golang:1.18

COPY --from=builder /minitwit/app /minitwit
WORKDIR /minitwit

USER 1000

ENTRYPOINT [ "./app" ]