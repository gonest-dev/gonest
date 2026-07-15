// Package shared holds cross-cutting pieces every domain module (user,
// post, comment) depends on: the shared *sql.DB, the auth Guard, the
// timing Interceptor, the request-id Middleware, and the DB-error Filter.
package shared

import (
	"database/sql"

	_ "modernc.org/sqlite"

	"github.com/gonest-dev/gonest"
)

const schema = `
CREATE TABLE IF NOT EXISTS users (
	id    INTEGER PRIMARY KEY AUTOINCREMENT,
	name  TEXT NOT NULL,
	email TEXT NOT NULL UNIQUE
);

CREATE TABLE IF NOT EXISTS posts (
	id      INTEGER PRIMARY KEY AUTOINCREMENT,
	user_id INTEGER NOT NULL REFERENCES users(id),
	title   TEXT NOT NULL,
	body    TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS comments (
	id      INTEGER PRIMARY KEY AUTOINCREMENT,
	post_id INTEGER NOT NULL REFERENCES posts(id),
	user_id INTEGER NOT NULL REFERENCES users(id),
	body    TEXT NOT NULL
);
`

// openDB opens an in-memory SQLite database shared across every pooled
// connection via "file::memory:?cache=shared" -- a bare ":memory:" DSN
// gives each pooled connection its OWN throwaway database (a well-known
// SQLite+Go gotcha), and forcing db.SetMaxOpenConns(1) to work around that
// (an earlier version of this example did exactly that) deadlocks under
// Fiber/fasthttp's concurrent request handling -- a second request's
// Exec blocks forever waiting for the single connection while it's
// (apparently) still considered in use. The shared-cache DSN gives every
// pooled connection a real, independently-usable handle onto the SAME
// underlying in-memory database, with no artificial single-connection
// bottleneck.
func openDB() *sql.DB {
	db, err := sql.Open("sqlite", "file::memory:?cache=shared")
	if err != nil {
		panic(err)
	}

	if _, err := db.Exec(schema); err != nil {
		panic(err)
	}
	return db
}

// DBProvider resolves the single shared *sql.DB every domain module's own
// Provider depends on.
var DBProvider = gonest.NewProvider(func(provider *gonest.Provider) {
	provider.Constructor(func() *sql.DB { return openDB() })
})

// DBModule owns DBProvider and exports it so user/post/comment's own
// modules can import it.
var DBModule = gonest.NewModule(func(module *gonest.Module) {
	module.Providers(DBProvider)
	module.Exports(DBProvider)
})
