FROM docker.io/library/golang:1.18

ARG branch=main

WORKDIR /app
RUN git clone -b ${branch} --depth=1 "https://github.com/salsitu/minitwit_thesvindler.git"

WORKDIR /app/minitwit_thesvindler/src/app
RUN go mod download
RUN GOOS=linux CGO_ENABLED=1 go build -o minitwit-app .

ENTRYPOINT [ "./minitwit-app" ]
