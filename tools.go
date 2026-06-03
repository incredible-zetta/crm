//go:build tools

// Package tools pins build/runtime dependencies that are not yet imported by
// real code. Remove entries here as packages get used by internal/ code.
package tools

import (
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/mysql"
	_ "github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/modelcontextprotocol/go-sdk/mcp"
)
