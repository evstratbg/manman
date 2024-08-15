PYTHON := rye run python
PYTEST := rye run pytest
COVERAGE_THRESHOLD := 100

GREEN := \033[0;32m
RED := \033[0;31m
NC := \033[0m

.PHONY: all test lint clean help

all: test lint

run_server:
	@echo "$(GREEN)Starting the server...$(NC)"
	@$(PYTHON) -m src.app.app

test:
	@echo "$(GREEN)Running tests...$(NC)"
	@$(PYTEST) . --force-sugar --color=yes -ra \
		--cov=. --cov-report=xml:coverage.xml \
		--cov-report term-missing \
		--junitxml=pytest-junit.xml \
		--cov-fail-under=$(COVERAGE_THRESHOLD) \
		--no-cov-on-fail || (echo "$(RED)Tests failed!$(NC)"; exit 1)
	@diff-cover coverage.xml --compare-branch=origin/main --fail-under 100
	@echo "$(GREEN)Tests passed successfully!$(NC)"

retest:
	@echo "$(GREEN)Rerunning failed tests...$(NC)"
	@$(PYTEST) . --force-sugar --color=yes -ra --lf

lint:
	@echo "$(GREEN)Running linters and formatters...$(NC)"
	@rye lint --fix . && \
	rye fmt . && \
	rye run mypy . || (echo "$(RED)Linting failed!$(NC)"; exit 1)
	@echo "$(GREEN)Linting completed successfully!$(NC)"

clean:
	@echo "$(GREEN)Cleaning up...$(NC)"
	@find . -type f -name "*.pyc" -delete
	@find . -type d -name "__pycache__" -delete
	@rm -rf .pytest_cache
	@rm -f coverage.xml pytest-junit.xml
	@echo "$(GREEN)Cleanup completed!$(NC)"

help:
	@echo "Available commands:"
	@echo "  make test  - Run tests with coverage"
	@echo "  make retest  - Rerun last failed tests"
	@echo "  make lint  - Run linters and formatters"
	@echo "  make clean - Clean up temporary files"
	@echo "  make all   - Run both tests and linters"
	@echo "  make help  - Show this help message"