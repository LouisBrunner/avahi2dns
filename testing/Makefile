all: run
.PHONY: all

run:
	touch .ash_history
	docker compose up --build
.PHONY: run

shell-release:
	docker compose exec avahi2dns-release ash
.PHONY: shell-release

shell-development:
	docker compose exec avahi2dns-development ash
.PHONY: shell-development

ips:
	docker ps
	docker compose exec avahi2dns-release ip addr show dev eth0 | grep inet
	docker compose exec avahi2dns-development ip addr show dev eth0 | grep inet
	# docker compose exec somehost ip addr show dev eth0 | grep inet
	# docker compose exec somehost4 ip addr show dev eth0 | grep inet
	# docker compose exec somehost6 ip addr show dev eth0 | grep inet
.PHONY: ips
