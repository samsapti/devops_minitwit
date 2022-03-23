FROM docker.io/library/golang:1.18

WORKDIR /app
RUN git clone "https://github.com/salsitu/minitwit_thesvindler.git"
RUN rm /app/minitwit_thesvindler/tmp/minitwit.db

ENV GOOS=linux CGO_ENABLED=1

WORKDIR /app/minitwit_thesvindler/minitwit_rewrite_go/api
RUN go mod download
RUN go build -o minitwit .

ENTRYPOINT [ "./minitwit" ]
