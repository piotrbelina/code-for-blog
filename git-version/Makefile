.PHONY: docker

GIT_COMMIT=$(shell git rev-parse HEAD)
GIT_DIRTY=$(shell test -n "`git status --porcelain`" && echo "+CHANGES" || true)
BUILD_DATE=$(shell TZ=UTC0 git show --quiet --date='format-local:%Y-%m-%dT%T%z' --format="%cd")

docker:
	docker build -f Dockerfile -t git-version --build-arg COMMIT=${GIT_COMMIT}${GIT_DIRTY} --build-arg BUILD_DATE=${BUILD_DATE} .