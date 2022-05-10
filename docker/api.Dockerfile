FROM docker.io/library/golang:1.18 AS builder

COPY src /minitwit
WORKDIR /minitwit/api

RUN go mod download
RUN GOOS=linux CGO_ENABLED=1 go build -o api .

FROM docker.io/library/golang:1.18

COPY --from=builder /minitwit/api /minitwit
WORKDIR /minitwit

USER 1000
EXPOSE 8000

ENTRYPOINT [ "./api" ]
