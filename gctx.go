package cliflagutils

import (
	"context"

	MU "github.com/fbaube/miscutils"

	"github.com/jmoiron/sqlx"
)

// GCtx carries background info thru a thread or goroutine.
// Golang best practices for `Context` say that
// 1) It should not be embedded anywhere, but rather should
// be passed as the first argument to func calls, and
// 2) It should contain only request-scoped data.
// But we won't abide by those rules for Context because
// 1) We only use it for cancellation, and
// 2) We want zero-argument method calls, and
// 3) We also store data that is NOT request-scoped.
//
type GCtx struct {
	// Context, used mainly for cancellation
	context.Context // = context.Context.TODO
	// Database connection
	*sqlx.DB
	// Database transaction
	*sqlx.Tx
	// XML configuration
	*XmlAppConfiguration
	// OwnLogPfx is own log prefix (should include a blank if needed)
	OwnLogPfx string
	// OwnLog is own logger
	OwnLog *MU.Logger
	// SsnLog is session logger (shared by all input files)
	SsnLog *MU.Logger
}
