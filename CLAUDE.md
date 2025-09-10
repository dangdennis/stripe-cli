# Claude Code Project Guide - Stripe CLI

## Project Overview
This is the official Stripe CLI - a command-line interface for Stripe's payment platform written in Go. It helps developers build, test, and manage Stripe integrations from the terminal.

## Project Structure
- `cmd/stripe/main.go` - Main entry point
- `pkg/` - Core packages organized by functionality
- `pkg/cmd/` - CLI command implementations
- `pkg/cmd/tui/` - Terminal UI components
- `rpc/` - gRPC protocol definitions
- `api/` - OpenAPI specifications
- `scripts/` - Build and deployment scripts

## Key Commands
- `make build` - Build the CLI binary
- `make test` - Run tests
- `make lint` - Run linting (uses golangci-lint)
- `go mod tidy` - Clean up dependencies

## Development Workflow
1. The project uses Go modules for dependency management
2. Main branch is `master`
3. Current working branch: `dangdennis/tui` (TUI improvements)
4. Follow Go conventions and existing code patterns
5. Always run tests and linting before committing

## Important Notes
- This is a defensive security tool for payment processing
- Handle API keys and authentication securely
- Follow Stripe's security best practices
- The TUI package is currently being enhanced on this branch

## Testing
- Use `go test ./...` to run all tests
- Individual package tests available in `*_test.go` files
- Integration tests may require Stripe API keys

## Architecture
- Modular design with clear separation of concerns
- gRPC for internal service communication
- Cobra CLI framework for command structure
- Supports multiple output formats and interactive modes