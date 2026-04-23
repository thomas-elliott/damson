// Package damson is the module root. It exists primarily so that embedded
// resources (SQL migrations, built SPA assets) can live at paths relative to
// the project root without breaking //go:embed's "cannot traverse upward"
// rule.
package damson

import "embed"

// MigrationsFS holds the SQL migration files applied by internal/db.Migrate.
//
//go:embed database/migrations/*.sql
var MigrationsFS embed.FS

// WebStaticFS holds the built SPA served by internal/web.Mount. The `all:`
// prefix is required so the embed succeeds before the SPA has been built
// (when only a placeholder .gitkeep exists in web/static).
//
//go:embed all:web/static
var WebStaticFS embed.FS
