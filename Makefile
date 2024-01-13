VERSION=0.0.1

.PHONY: all
all: dockerbuild
- FORCE:

.PHONY: dockerbuild
dockerbuild:
	docker build -t file-server-go:${VERSION} .
