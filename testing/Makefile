all: run
.PHONY: all

build:
	docker build -t a2d . --progress plain
.PHONY: build

run: build
	docker run --dns=127.0.0.1 --dns=8.8.8.8 -it --rm --name a2d --privileged a2d
.PHONY: run

shell:
	docker exec -it a2d ash
.PHONY: shell
