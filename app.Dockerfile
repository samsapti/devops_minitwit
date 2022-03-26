FROM docker.io/library/golang:1.18

WORKDIR /app
RUN git clone "https://github.com/salsitu/minitwit_thesvindler.git"

ENV GOOS=linux CGO_ENABLED=1

WORKDIR /app/minitwit_thesvindler/src/app
RUN go mod download
RUN go build -o minitwit-app .

ENTRYPOINT [ "./minitwit-app" ]
