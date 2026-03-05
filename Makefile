GO           ?= go
COVERMODE    ?= atomic

BADGE_DIR    ?= .public
BADGE_LABEL  ?= coverage

COVERPROFILE_ALL ?= coverage.out

# Módulos definidos no Phase 1, 2 e 3 do Roadmap
WORK_MODULES   ?= adapters common controller core di exceptions guards interceptors pipes swagger validator
WORK_PACKAGES  := $(addsuffix /...,$(addprefix ./,$(WORK_MODULES)))

# Caminho para as ferramentas (agora tratadas como pacotes do módulo raiz)
BADGE_TOOL_PKG ?= ./_tools/badge
TAG_TOOL_PKG   ?= ./_tools/tag
CLEAN_TOOL_PKG ?= ./_tools/clean

.DEFAULT_GOAL := help

.PHONY: help
help:
	@echo "GoNest Framework - Targets:"
	@echo "  make build                # Compila todos os pacotes do framework"
	@echo "  make test                 # Executa testes em todos os módulos"
	@echo "  make cover                # Gera relatório de cobertura $(COVERPROFILE_ALL)"
	@echo "  make badge                # Gera o badge de cobertura geral"
	@echo "  make ci                   # Fluxo completo: build + test + badges"
	@echo "  make tag <version>        # Gerencia versões (ex: make tag v0.1.0)"

.PHONY: build
build:
	$(GO) build $(WORK_PACKAGES)

.PHONY: test
test:
	$(GO) test $(WORK_PACKAGES)

.PHONY: cover
cover:
	$(GO) test $(WORK_PACKAGES) -coverprofile=$(COVERPROFILE_ALL) -covermode=$(COVERMODE)

# Badge Geral
$(BADGE_DIR)/coverage.svg: $(COVERPROFILE_ALL)
	$(GO) run $(BADGE_TOOL_PKG) -in $< -out $@ -label $(BADGE_LABEL)

.PHONY: badge
badge: $(BADGE_DIR)/coverage.svg

# Regras dinâmicas por módulo
define module_rules
.PHONY: $(1).cover
$(1).cover:
	$(GO) test ./$(1)/... -coverprofile=$(1).coverage.out -covermode=$(COVERMODE)

$(BADGE_DIR)/$(1)-coverage.svg: $(1).cover
	$(GO) run $(BADGE_TOOL_PKG) -in $(1).coverage.out -out $$@ -label $(BADGE_LABEL)
endef

$(foreach m,$(WORK_MODULES),$(eval $(call module_rules,$(m))))

.PHONY: module-badges
module-badges: $(foreach m,$(WORK_MODULES),$(BADGE_DIR)/$(m)-coverage.svg)

.PHONY: badges
badges: badge module-badges

.PHONY: ci
ci: build cover badges

.PHONY: clean
clean:
	$(GO) clean -testcache
	$(GO) run $(CLEAN_TOOL_PKG) $(COVERPROFILE_ALL) "*.coverage.out" "$(BADGE_DIR)/*.svg"

# Gerenciamento de Tags e Versões (Phase 18)
.PHONY: tag
tag:
	$(GO) run $(TAG_TOOL_PKG) --create --push "$(filter-out $@,$(MAKECMDGOALS))"

.PHONY: tag-minor
tag-minor:
	$(GO) run $(TAG_TOOL_PKG) --bump patch

.PHONY: tag-major
tag-major:
	$(GO) run $(TAG_TOOL_PKG) --bump minor

%:
	@: