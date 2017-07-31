# kernel-style V=1 build verbosity
ifeq ("$(origin V)", "command line")
	BUILD_VERBOSE = $(V)
endif

ifeq ($(BUILD_VERBOSE),1)
	Q =
else
	Q = @
endif

VERSION=$(shell git describe --dirty)
REPO=github.com/coreos/locksmith
LD_FLAGS="-w -s -extldflags -static"

.PHONY: all
all: bin/locksmithctl

GO_SOURCES := $(shell find . -type f -name "*.go")

bin/%: $(GO_SOURCES)
	go build -o $@ -ldflags $(LD_FLAGS) $(REPO)/$*

.PHONY: test
test:
	$(Q)./scripts/test

.PHONY: vendor
vendor:
	$(Q)glide update --strip-vendor
	$(Q)glide-vc --use-lock-file --no-tests --only-code

.PHONY: clean
clean:
	$(Q)rm -rf bin
