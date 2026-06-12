# nomagique — common developer targets

.PHONY: help test bench profile profile-cpu profile-mem profile-block profile-open clean-profiles physics-metallib build

PROFILE_DIR := .profiles
PKG ?= .
BENCH ?= BenchmarkNumber_stressSeries
BENCH_TIME ?= 5s
PPROF_ADDR ?= localhost:8080
KIND ?= cpu

help:
	@echo "Targets:"
	@echo "  make test              Run all tests"
	@echo "  make bench             Run root-package benchmarks"
	@echo "  make profile           Generate pprof ($(KIND)) and open in browser"
	@echo "  make profile-cpu       CPU profile only (override BENCH, BENCH_TIME)"
	@echo "  make profile-mem       heap profile only"
	@echo "  make profile-block     block profile only"
	@echo "  make profile-open      Open an existing profile (FILE=...)"
	@echo "  make clean-profiles    Remove $(PROFILE_DIR)/"
	@echo ""
	@echo "Variables:"
	@echo "  BENCH=$(BENCH)"
	@echo "  BENCH_TIME=$(BENCH_TIME)"
	@echo "  PKG=$(PKG)"
	@echo "  KIND=cpu|mem|block"

test:
	go test ./...

bench:
	go test -bench=. -benchmem -run=^$$ $(PKG)

$(PROFILE_DIR):
	mkdir -p $(PROFILE_DIR)

profile: $(PROFILE_DIR)
	$(MAKE) profile-$(KIND)

profile-cpu: $(PROFILE_DIR)
	go test -bench=$(BENCH) -benchtime=$(BENCH_TIME) -run=^$$ \
		-cpuprofile=$(PROFILE_DIR)/cpu.prof $(PKG)
	$(MAKE) profile-open FILE=$(PROFILE_DIR)/cpu.prof

profile-mem: $(PROFILE_DIR)
	go test -bench=$(BENCH) -benchtime=$(BENCH_TIME) -run=^$$ \
		-memprofile=$(PROFILE_DIR)/mem.prof $(PKG)
	$(MAKE) profile-open FILE=$(PROFILE_DIR)/mem.prof

profile-block: $(PROFILE_DIR)
	go test -bench=$(BENCH) -benchtime=$(BENCH_TIME) -run=^$$ \
		-blockprofile=$(PROFILE_DIR)/block.prof $(PKG)
	$(MAKE) profile-open FILE=$(PROFILE_DIR)/block.prof

profile-open:
	@test -n "$(FILE)" || (echo "FILE is required, e.g. FILE=$(PROFILE_DIR)/cpu.prof" && exit 1)
	@test -f "$(FILE)" || (echo "profile not found: $(FILE)" && exit 1)
	go tool pprof -http=$(PPROF_ADDR) "$(FILE)"

clean-profiles:
	rm -rf $(PROFILE_DIR)

physics-metallib:
	cd physics/manifold && go run ./metallibgen

build: physics-metallib
	@mkdir -p $(LOG_DIR)
	go build -o $(SYMM_BIN) .

