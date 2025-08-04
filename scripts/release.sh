#!/bin/bash

# =============================================================================
# db-kit Release Script
# =============================================================================
# This script handles the release process with safety checks and confirmations

set -e  # Exit on any error

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored output
print_info() {
    echo -e "${BLUE}ℹ️  $1${NC}"
}

print_success() {
    echo -e "${GREEN}✅ $1${NC}"
}

print_warning() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

print_error() {
    echo -e "${RED}❌ $1${NC}"
}

# Function to check if command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Check required tools
check_requirements() {
    print_info "Checking requirements..."

    if ! command_exists git; then
        print_error "git is not installed"
        exit 1
    fi

    if ! command_exists semver-generator; then
        print_error "semver-generator is not installed"
        exit 1
    fi

    print_success "All requirements met"
}

# Check for uncommitted changes
check_uncommitted_changes() {
    print_info "Checking for uncommitted changes..."

    if [ -n "$(git status --porcelain)" ]; then
        print_error "You have uncommitted changes. Please commit or stash them first."
        git status --short
        exit 1
    fi

    print_success "Working directory is clean"
}

# Check if we're on main branch
check_branch() {
    print_info "Checking current branch..."

    CURRENT_BRANCH=$(git branch --show-current)
    if [ "$CURRENT_BRANCH" != "main" ]; then
        print_error "You must be on the main branch to release."
        print_error "Current branch: $CURRENT_BRANCH"
        exit 1
    fi

    print_success "On main branch"
}

# Function to validate semantic version format
validate_semver() {
    local version=$1

    # Regex pattern for semantic versioning (x.y.z[-prerelease][+build])
    # Matches: 1.0.0, 1.0.0-alpha, 1.0.0-alpha.1, 1.0.0+20130313144700, etc.
    local semver_pattern='^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?(\+[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?$'

    if [[ ! $version =~ $semver_pattern ]]; then
        return 1
    fi
    return 0
}

# Generate version
generate_version() {
    print_info "Generating version..."

    if [ ! -f "./config/semver.yaml" ]; then
        print_error "semver.yaml configuration file not found"
        exit 1
    fi

    # Get raw output from semver-generator
    local raw_output
    raw_output=$(semver-generator -c ./config/semver.yaml generate -l 2>/dev/null)

    if [ $? -ne 0 ]; then
        print_error "Failed to execute semver-generator command"
        exit 1
    fi

    # Extract version using awk (keeping the original logic as fallback)
    VERSION=$(echo "$raw_output" | awk '{print $2}')

    # Validate that version is not empty
    if [ -z "$VERSION" ]; then
        print_error "Failed to extract version from semver-generator output"
        print_error "Raw output: $raw_output"
        exit 1
    fi

    # Validate semantic version format
    if ! validate_semver "$VERSION"; then
        print_error "Invalid semantic version format: '$VERSION'"
        print_error "Expected format: x.y.z[-prerelease][+build] (e.g., 1.0.0, 1.0.0-alpha, 1.0.0+20130313144700)"
        print_error "Raw output from semver-generator: $raw_output"
        exit 1
    fi

    print_success "Generated version: v$VERSION"
    echo "$VERSION"
}

# Check if tag already exists
check_tag_exists() {
    local version=$1
    local tag="v$version"

    if git tag -l | grep -q "^$tag$"; then
        print_error "Tag $tag already exists"
        exit 1
    fi

    print_success "Tag v$version is available"
}

# Confirm release
confirm_release() {
    local version=$1

    echo ""
    print_warning "This will:"
    echo "  1. Create git tag: v$version"
    echo "  2. Push to origin/main"
    echo "  3. Push tags to origin"
    echo ""

    read -p "🤔 Are you sure you want to release v$version? (y/N): " -n 1 -r
    echo

    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        print_warning "Release cancelled."
        exit 0
    fi
}

# Create tag
create_tag() {
    local version=$1
    local tag="v$version"

    print_info "Creating tag: $tag"
    git tag "$tag"
    print_success "Tag created: $tag"
}

# Push to main
push_main() {
    print_info "Pushing to origin/main..."
    git push origin main
    print_success "Pushed to origin/main"
}

# Push tags
push_tags() {
    print_info "Pushing tags..."
    git push origin --tags
    print_success "Tags pushed to origin"
}

# Main release function
release() {
    print_info "Starting release process..."

    # Run all checks
    check_requirements
    check_uncommitted_changes
    check_branch

    # Generate version
    VERSION=$(generate_version)
    check_tag_exists "$VERSION"

    # Confirm with user
    confirm_release "$VERSION"

    # Execute release
    create_tag "$VERSION"
    push_main
    push_tags

    # Success message
    echo ""
    print_success "Release v$VERSION completed successfully!"
    echo "📋 Tag: v$VERSION"
    echo "🔗 You can now create a GitHub release from this tag."
    echo ""
    print_info "Next steps:"
    echo "  1. Go to GitHub repository"
    echo "  2. Click 'Releases'"
    echo "  3. Click 'Create a new release'"
    echo "  4. Select tag v$VERSION"
    echo "  5. Add release notes and publish"
}

# Help function
show_help() {
    echo "Usage: $0 [OPTIONS]"
    echo ""
    echo "Options:"
    echo "  -h, --help     Show this help message"
    echo "  -v, --version  Show script version"
    echo ""
    echo "This script handles the release process with safety checks:"
    echo "  - Checks for uncommitted changes"
    echo "  - Verifies you're on main branch"
    echo "  - Generates version using semver-generator"
    echo "  - Confirms release with user"
    echo "  - Creates git tag and pushes to origin"
}

# Version function
show_version() {
    echo "db-kit release script v1.0.0"
}

# Parse command line arguments
case "${1:-}" in
    -h|--help)
        show_help
        exit 0
        ;;
    -v|--version)
        show_version
        exit 0
        ;;
    "")
        # No arguments, run release
        release
        ;;
    *)
        print_error "Unknown option: $1"
        show_help
        exit 1
        ;;
esac
