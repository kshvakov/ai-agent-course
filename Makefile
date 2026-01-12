.DEFAULT_GOAL := help

VENV_DIR := .venv
VENV_BIN := $(VENV_DIR)/bin
PYTHON := $(VENV_BIN)/python
PIP := $(VENV_BIN)/pip
MKDOCS := $(VENV_BIN)/mkdocs

.PHONY: help venv install prepare serve setup clean-venv

help: ## Show help for available targets
	@echo ""
	@echo "Targets:"
	@grep -E '^[a-zA-Z0-9_-]+:.*##' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*## "}; {printf "  %-14s %s\n", $$1, $$2}'
	@echo ""
	@echo "Examples:"
	@echo "  make setup    # create .venv and install dependencies"
	@echo "  make prepare  # build docs (.mkdocs/scripts/prepare_docs.py)"
	@echo "  make serve    # run mkdocs locally"
	@echo ""

venv: ## Create a virtual environment in .venv (if missing)
	@test -x "$(PYTHON)" || python -m venv "$(VENV_DIR)"

install: venv ## Install dependencies (requirements.txt) into the venv
	@"$(PIP)" install -r requirements.txt
	@"$(PIP)" install -e .

prepare: install ## Prepare documentation (python .mkdocs/scripts/prepare_docs.py)
	@"$(PYTHON)" .mkdocs/scripts/prepare_docs.py

serve: prepare ## Run local MkDocs server
	@"$(MKDOCS)" serve

setup: install ## Create .venv and install dependencies

clean-venv: ## Remove the .venv virtual environment
	@rm -rf "$(VENV_DIR)"


