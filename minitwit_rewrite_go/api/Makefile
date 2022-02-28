.PHONY: build run stop

build:
	docker build -t minitwit-go .

run:
	docker run -d --rm -p 8000:8000 --name minitwit minitwit-go

stop:
	docker stop minitwit
