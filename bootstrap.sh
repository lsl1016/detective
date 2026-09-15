#!/usr/bin/env bash
set -euo pipefail

# Run from repository root. Environment variables are documented in .env.example.
exec go run ./cmd/bootstrap
