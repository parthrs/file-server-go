KIND_CLUSTER_NAME=playground
VERSION=0.0.1

.PHONY: all
all: dockerbuild kindload
- FORCE:

.PHONY: dockerbuild
dockerbuild:
	docker build -t file-server-go:${VERSION} .

.PHONY: kindload
kindload:
	kind load docker-image file-server-go:0.0.1 --name ${KIND_CLUSTER_NAME}
