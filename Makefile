.PHONY: test test-pretty test-watch test-ci test-summary cover

PKGS ?= ./...
JSON_OUT ?= test-output.json
JUNIT ?= test-report.xml
COVER ?= coverage.out

# Plain go test. Fastest, no extra deps loaded
test:
	go test $(PKGS)

# Live pretty output. Groups by test name, reruns flakes once.
# Hides gotestsum's plain-text summary; tparse renders failures instead.
test-pretty:
	@set +e; \
	go tool gotestsum \
		--format testname \
		--format-icons hivis \
		--hide-summary=skipped,output,errors,failed \
		--rerun-fails=1 \
		--jsonfile $(JSON_OUT) \
		--packages="$(PKGS)" \
		-- -count=1; \
	rc=$$?; \
	echo; \
	tparse -all -file=$(JSON_OUT) || true; \
	exit $$rc

# TDD loop: re-runs affected packages on file save.
test-watch:
	go tool gotestsum --watch --format testname --packages="$(PKGS)"

# CI / pre-push: race + coverage + JSON + JUnit + tparse summary table
test-ci:
	@set +e; \
	go tool gotestsum \
		--format pkgname \
		--format-icons hivis \
		--hide-summary=skipped,output,errors,failed \
		--jsonfile $(JSON_OUT) \
		--junitfile $(JUNIT) \
		--packages="$(PKGS)" \
		-- -count=1 -race -covermode=atomic -coverprofile=$(COVER); \
	rc=$$?; \
	echo; \
	tparse -all -slow 10 -file=$(JSON_OUT) || true; \
	exit $$rc

# Replay last summary without re-running tests
test-summary:
	tparse -all -file=$(JSON_OUT)

# Open HTML coverage report from last `test-ci` run
cover:
	go tool cover -html=$(COVER)
