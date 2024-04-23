KIND_CLUSTER_NAME=playground
VERSION=0.0.5
VERSION_FRONTEND=0.0.1

.PHONY: all
all: backend kindload frontend kindload-frontend
- FORCE:

.PHONY: backend
backend:
	docker build -t file-server-go:${VERSION} -f Dockerfile .

.PHONY: kindload
kindload:
	kind load docker-image file-server-go:${VERSION} --name ${KIND_CLUSTER_NAME}

.PHONY: frontend
frontend:
	docker build -t file-server-go-frontend:${VERSION_FRONTEND} -f Dockerfile-frontend .

.PHONY: kindload-frontend
kindload-frontend:
	kind load docker-image file-server-go-frontend:${VERSION_FRONTEND} --name ${KIND_CLUSTER_NAME}
