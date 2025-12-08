#!/bin/bash

# quality_check.sh
# Runs all CI quality checks locally before pushing
# Mirrors the GitHub Actions CI workflow: "CI - Go Full Quality Check"
#
# Usage: ./scripts/quality_check.sh
#
# Exit codes:
#   0 - All checks passed
#   1 - One or more checks failed

set -e  # Exit on first error

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Minimum coverage threshold (matches CI)
COVERAGE_THRESHOLD=80

# Track overall status
FAILED=0

# Print a stage header
print_stage() {
    echo ""
    echo -e "${BLUE}PPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPP${NC}"
    echo -e "${BLUE}  Stage $1: $2${NC}"
    echo -e "${BLUE}PPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPP${NC}"
    echo ""
}

# Print success message
print_success() {
    echo -e "${GREEN} $1${NC}"
}

# Print failure message
print_failure() {
    echo -e "${RED} $1${NC}"
}

# Print warning message
print_warning() {
    echo -e "${YELLOW}� $1${NC}"
}

# Function to run a stage and handle errors
run_stage() {
    local stage_num=$1
    local stage_name=$2
    local stage_cmd=$3

    print_stage "$stage_num" "$stage_name"

    if eval "$stage_cmd"; then
        print_success "$stage_name passed"
        return 0
    else
        print_failure "$stage_name FAILED"
        echo ""
        echo -e "${RED}Build halted at stage $stage_num: $stage_name${NC}"
        exit 1
    fi
}

echo ""
echo -e "${BLUE}TPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPW${NC}"
echo -e "${BLUE}Q           Go Full Quality Check - Local Runner                Q${NC}"
echo -e "${BLUE}Q     Mirrors CI workflow: CI - Go Full Quality Check           Q${NC}"
echo -e "${BLUE}ZPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPP]${NC}"
echo ""

# ============================================================================
# Stage 1: Download dependencies
# ============================================================================
run_stage 1 "Download Go modules" "go mod download"

# ============================================================================
# Stage 2: Check formatting
# ============================================================================
print_stage 2 "Check formatting (gofmt)"

unformatted=$(gofmt -l . 2>&1 || true)
if [ -n "$unformatted" ]; then
    print_failure "Files not properly formatted:"
    echo "$unformatted"
    echo ""
    echo -e "${YELLOW}To fix, run: gofmt -w .${NC}"
    echo ""
    echo -e "${RED}Build halted at stage 2: Check formatting${NC}"
    exit 1
else
    print_success "All files properly formatted"
fi

# ============================================================================
# Stage 3: Verify build (compilation)
# ============================================================================
run_stage 3 "Verify build" "go build ./..."

# ============================================================================
# Stage 4: Run go vet
# ============================================================================
run_stage 4 "Go vet" "go vet ./..."

# ============================================================================
# Stage 5: Install and run golint
# ============================================================================
print_stage 5 "Lint (golint)"

# Check if golint is installed, install if not
if ! command -v golint &> /dev/null; then
    print_warning "golint not found, installing..."
    go install golang.org/x/lint/golint@latest
fi

lint_issues=$(golint ./... 2>&1 || true)
if [ -n "$lint_issues" ]; then
    print_failure "Lint issues found:"
    echo "$lint_issues"
    echo ""
    echo -e "${RED}Build halted at stage 5: Lint${NC}"
    exit 1
else
    print_success "No lint issues found"
fi

# ============================================================================
# Stage 6: Install and run staticcheck
# ============================================================================
print_stage 6 "Static analysis (staticcheck)"

# Check if staticcheck is installed, install if not
if ! command -v staticcheck &> /dev/null; then
    print_warning "staticcheck not found, installing..."
    go install honnef.co/go/tools/cmd/staticcheck@latest
fi

if staticcheck ./... 2>&1; then
    print_success "Static analysis passed"
else
    print_failure "Static analysis FAILED"
    echo ""
    echo -e "${RED}Build halted at stage 6: Static analysis${NC}"
    exit 1
fi

# ============================================================================
# Stage 7: Run tests with coverage
# ============================================================================
print_stage 7 "Run tests with coverage"

# Coverage file (use simple name in current directory to avoid path issues)
COVERAGE_FILE="coverage.out"

if CI=true go test ./... -tags=test -coverprofile="$COVERAGE_FILE" -v 2>&1; then
    print_success "All tests passed"
else
    print_failure "Tests FAILED"
    echo ""
    echo -e "${RED}Build halted at stage 7: Run tests${NC}"
    exit 1
fi

# ============================================================================
# Stage 8: Check coverage threshold
# ============================================================================
print_stage 8 "Check coverage threshold (minimum ${COVERAGE_THRESHOLD}%)"

# Extract total coverage percentage
coverage_output=$(go tool cover -func="$COVERAGE_FILE" | grep total: || echo "total: 0.0%")
coverage=$(echo "$coverage_output" | awk '{print $3}' | sed 's/%//')

echo "Coverage report:"
echo "$coverage_output"
echo ""

# Compare coverage to threshold using awk (works on both Linux and macOS)
if awk "BEGIN {exit !($coverage < $COVERAGE_THRESHOLD)}"; then
    print_failure "Coverage ${coverage}% is below ${COVERAGE_THRESHOLD}% threshold"
    echo ""
    echo -e "${YELLOW}To view detailed coverage report, run:${NC}"
    echo "  go tool cover -html=$COVERAGE_FILE"
    echo ""
    echo -e "${RED}Build halted at stage 8: Coverage check${NC}"
    exit 1
else
    print_success "Coverage ${coverage}% meets threshold of ${COVERAGE_THRESHOLD}%"
fi

# ============================================================================
# Stage 9: Install and run govulncheck
# ============================================================================
print_stage 9 "Vulnerability check (govulncheck)"

# Check if govulncheck is installed, install if not
if ! command -v govulncheck &> /dev/null; then
    print_warning "govulncheck not found, installing..."
    go install golang.org/x/vuln/cmd/govulncheck@latest
fi

if govulncheck ./... 2>&1; then
    print_success "No vulnerabilities found"
else
    print_failure "Vulnerability check FAILED"
    echo ""
    echo -e "${RED}Build halted at stage 9: Vulnerability check${NC}"
    exit 1
fi

# ============================================================================
# All checks passed!
# ============================================================================
echo ""
echo -e "${GREEN}TPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPW${NC}"
echo -e "${GREEN}Q                    ALL CHECKS PASSED!                        Q${NC}"
echo -e "${GREEN}ZPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPPP]${NC}"
echo ""
echo "Coverage: ${coverage}%"
echo "Coverage report saved to: $COVERAGE_FILE"
echo ""
echo -e "${YELLOW}To view HTML coverage report:${NC}"
echo "  go tool cover -html=$COVERAGE_FILE"
echo ""

exit 0