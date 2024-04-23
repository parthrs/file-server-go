KIND_CLUSTER_NAME=playground
VERSION=0.0.5
VERSION_FRONTEND=0.0.1

# Newer ver of docker doesn't print
# output of each step/layer
BACKEND_BUILDOUTPUT=
FRONTEND_BUILDOUTPUT=DOCKER_BUILDKIT=0
# Enable/disable using cache
BACKEND_BUILDCACHE=
FRONTEND_BUILDCACHE=--no-cache

.PHONY: all
all: backend kindload frontend kindload-frontend
- FORCE:

.PHONY: backend
backend:
	${BACKEND_BUILDOUTPUT} docker build ${BACKEND_BUILDCACHE} -t file-server-go:${VERSION} -f Dockerfile .

.PHONY: kindload
kindload:
	kind load docker-image file-server-go:${VERSION} --name ${KIND_CLUSTER_NAME}

.PHONY: frontend
frontend:
	${FRONTEND_BUILDOUTPUT} docker build ${FRONTEND_BUILDCACHE} -t file-server-go-frontend:${VERSION_FRONTEND} -f Dockerfile-frontend .

.PHONY: kindload-frontend
kindload-frontend:
	kind load docker-image file-server-go-frontend:${VERSION_FRONTEND} --name ${KIND_CLUSTER_NAME}
