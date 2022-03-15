FROM docker.io/library/golang:1.17 AS builder

WORKDIR /builder
RUN git clone "https://github.com/salsitu/minitwit_thesvindler.git"
RUN rm /builder/minitwit_thesvindler/tmp/minitwit.db

ENV GOOS=linux CGO_ENABLED=1

WORKDIR /builder/minitwit_thesvindler/minitwit_rewrite_go/api
RUN go mod download
RUN go build -o minitwit .

ENTRYPOINT [ "./minitwit" ]
