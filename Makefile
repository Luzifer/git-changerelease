SHARNESS_VERSION=v1.0.0

publish:
	curl -sSLo golang.sh https://raw.githubusercontent.com/Luzifer/github-publish/master/golang.sh
	bash golang.sh

update-sharness:
	curl -sSLo ./integration/sharness.sh https://cdn.rawgit.com/chriscool/sharness/$(SHARNESS_VERSION)/sharness.sh
	curl -sSLo ./integration/aggregate-results.sh https://cdn.rawgit.com/chriscool/sharness/$(SHARNESS_VERSION)/aggregate-results.sh
	curl -sSLo ./integration/Makefile https://cdn.rawgit.com/chriscool/sharness/$(SHARNESS_VERSION)/test/Makefile

test:
	go test .
	go vet .

integration: install
	cd integration && make all

install: test
	go install -a -ldflags="-X main.version=$(shell git describe --tags)"

.PHONY: integration
