# Copyright 2019 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

.PHONY: build-% build container-% container push-% push clean test

# A space-separated list of all commands in the repository, must be
# set in main Makefile of a repository.
# CMDS=

# This is the default. It can be overridden in the main Makefile after
# including build.make.
REGISTRY_NAME=cocainefarm

# Can be set to -mod=vendor to ensure that the "vendor" directory is used.
GOFLAGS_VENDOR=

# Revision that gets built into each binary via the main.version
# string. Uses the `git describe` output based on the most recent
# version tag with a short revision suffix or, if nothing has been
# tagged yet, just the revision.
#
# Beware that tags may also be missing in shallow clones as done by
# some CI systems (like TravisCI, which pulls only 50 commits).
REV=$(shell git describe --long --tags --match='v*' --dirty 2>/dev/null || git rev-list -n1 HEAD)

# A space-separated list of image tags under which the current build is to be pushed.
# Determined dynamically.
IMAGE_TAGS=

# A "canary" image gets built if the current commit is the head of the remote "master" branch.
# That branch does not exist when building some other branch in TravisCI.
IMAGE_TAGS+=$(shell if [ "$$(git rev-list -n1 HEAD)" = "$$(git rev-list -n1 origin/master 2>/dev/null)" ]; then echo "canary"; fi)

# A "X.Y.Z-canary" image gets built if the current commit is the head of a "origin/release-X.Y.Z" branch.
# The actual suffix does not matter, only the "release-" prefix is checked.
IMAGE_TAGS+=$(shell git branch -r --points-at=HEAD | grep 'origin/release-' | grep -v -e ' -> ' | sed -e 's;.*/release-\(.*\);\1-canary;')

# A release image "vX.Y.Z" gets built if there is a tag of that format for the current commit.
# --abbrev=0 suppresses long format, only showing the closest tag.
IMAGE_TAGS+=$(shell tagged="$$(git describe --tags --match='v*' --abbrev=0)"; if [ "$$tagged" ] && [ "$$(git rev-list -n1 HEAD)" = "$$(git rev-list -n1 $$tagged)" ]; then echo $$tagged; fi)

# Images are named after the command contained in them.
IMAGE_NAME=$(REGISTRY_NAME)/$*

ifdef V
# Adding "-alsologtostderr" assumes that all test binaries contain glog. This is not guaranteed.
TESTARGS = -v -args -alsologtostderr -v 5
else
TESTARGS =
endif

ARCH := $(if $(GOARCH),$(GOARCH),$(shell go env GOARCH))

# Specific packages can be excluded from each of the tests below by setting the *_FILTER_CMD variables
# to something like "| grep -v 'github.com/kubernetes-csi/project/pkg/foobar'". See usage below.

build-%: check-go-version-go
	mkdir -p bin
	CGO_ENABLED=0 GOOS=linux go build $(GOFLAGS_VENDOR) -a -ldflags '-X main.version=$(REV) -extldflags "-static"' -o ./bin/$* ./cmd/$*
	if [ "$$ARCH" = "amd64" ]; then \
		CGO_ENABLED=0 GOOS=windows go build $(GOFLAGS_VENDOR) -a -ldflags '-X main.version=$(REV) -extldflags "-static"' -o ./bin/$*.exe ./cmd/$* ; \
		CGO_ENABLED=0 GOOS=linux GOARCH=ppc64le go build $(GOFLAGS_VENDOR) -a -ldflags '-X main.version=$(REV) -extldflags "-static"' -o ./bin/$*-ppc64le ./cmd/$* ; \
	fi

container-%:
	docker build -t $*:latest -f $(shell if [ -e ./cmd/$*/Dockerfile ]; then echo ./cmd/$*/Dockerfile; else echo Dockerfile; fi) --label revision=$(REV) .

push-%: container-%
	set -ex; \
	push_image () { \
		docker tag $*:latest $(IMAGE_NAME):$$tag; \
		docker push $(IMAGE_NAME):$$tag; \
	}; \
	for tag in $(IMAGE_TAGS); do \
		if [ "$$tag" = "canary" ] || echo "$$tag" | grep -q -e '-canary$$'; then \
			: "creating or overwriting canary image"; \
			push_image; \
		elif docker pull $(IMAGE_NAME):$$tag 2>&1 | tee /dev/stderr | grep -q "manifest for $(IMAGE_NAME):$$tag not found"; then \
			: "creating release image"; \
			push_image; \
		else \
			: "release image $(IMAGE_NAME):$$tag already exists, skipping push"; \
		fi; \
	done

build: $(CMDS:%=build-%)
container: $(CMDS:%=container-%)
push: $(CMDS:%=push-%)

clean:
	-rm -rf bin

test: check-go-version-go

.PHONY: test-go
test: test-go
test-go:
	@ echo; echo "### $@:"
	go test $(GOFLAGS_VENDOR) `go list $(GOFLAGS_VENDOR) ./... | grep -v -e 'vendor' -e '/test/e2e$$' $(TEST_GO_FILTER_CMD)` $(TESTARGS)

.PHONY: test-vet
test: test-vet
test-vet:
	@ echo; echo "### $@:"
	go test $(GOFLAGS_VENDOR) `go list $(GOFLAGS_VENDOR) ./... | grep -v vendor $(TEST_VET_FILTER_CMD)`

.PHONY: test-fmt
test: test-fmt
test-fmt:
	@ echo; echo "### $@:"
	files=$$(find . -name '*.go' | grep -v './vendor' $(TEST_FMT_FILTER_CMD)); \
	if [ $$(gofmt -d $$files | wc -l) -ne 0 ]; then \
		echo "formatting errors:"; \
		gofmt -d $$files; \
		false; \
	fi

# This test only runs when dep >= 0.5 is installed, which is the case for the CI setup.
# When using 'go mod', we allow the test to be skipped in the Prow CI under some special
# circumstances, because it depends on accessing all remote repos and thus
# running it all the time would defeat the purpose of vendoring:
# - not handling a PR or
# - the fabricated merge commit leaves go.mod, go.sum and vendor dir unchanged
# - release-tools also didn't change (changing rules or Go version might lead to
#   a different result and thus must be tested)
# - import statements not changed (because if they change, go.mod might have to be updated)
#
# "git diff" is intelligent enough to annotate changes inside the "import" block in
# the start of the diff hunk:
#
# diff --git a/rpc/common.go b/rpc/common.go
# index bb4a5c4..5fa4271 100644
# --- a/rpc/common.go
# +++ b/rpc/common.go
# @@ -21,7 +21,6 @@ import (
#         "fmt"
#         "time"
#
# -       "google.golang.org/grpc"
#         "google.golang.org/grpc/codes"
#         "google.golang.org/grpc/status"
#
# We rely on that to find such changes.
#
# Vendoring is optional when using go.mod.
.PHONY: test-vendor
test: test-vendor
test-vendor:
	@ echo; echo "### $@:"
	@ ./release-tools/verify-vendor.sh

.PHONY: test-subtree
test: test-subtree
test-subtree:
	@ echo; echo "### $@:"
	./release-tools/verify-subtree.sh release-tools

# Components can extend the set of directories which must pass shellcheck.
# The default is to check only the release-tools directory itself.
TEST_SHELLCHECK_DIRS=release-tools
.PHONY: test-shellcheck
test: test-shellcheck
test-shellcheck:
	@ echo; echo "### $@:"
	@ ret=0; \
	if ! command -v docker; then \
		echo "skipped, no Docker"; \
		exit 0; \
        fi; \
	for dir in $(abspath $(TEST_SHELLCHECK_DIRS)); do \
		echo; \
		echo "$$dir:"; \
		./release-tools/verify-shellcheck.sh "$$dir" || ret=1; \
	done; \
	exit $$ret

# Targets in the makefile can depend on check-go-version-<path to go binary>
# to trigger a warning if the x.y version of that binary does not match
# what the project uses. Make ensures that this is only checked once per
# invocation.
.PHONY: check-go-version-%
check-go-version-%:
	./release-tools/verify-go-version.sh "$*"
