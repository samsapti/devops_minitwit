FROM docker.io/library/golang:1.18

ARG branch=main

WORKDIR /app
RUN git clone -b ${branch} "https://github.com/salsitu/minitwit_thesvindler.git"

ENV GOOS=linux CGO_ENABLED=1

WORKDIR /app/minitwit_thesvindler/src/api
RUN go mod download
RUN go build -o minitwit-api .

ENTRYPOINT [ "./minitwit-api" ]
