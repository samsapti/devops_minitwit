FROM docker.io/library/golang:1.17 AS builder

WORKDIR /builder
RUN git clone "https://github.com/nissemand243/Gruppe_A.git"

ENV GOOS=linux CGO_ENABLED=1

WORKDIR /builder/Gruppe_A/minitwit_rewrite_go/api
RUN go mod download
RUN go build -o minitwit .

ENTRYPOINT [ "./minitwit" ]
