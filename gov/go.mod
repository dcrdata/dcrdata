module github.com/decred/dcrdata/gov/v2

go 1.11

require (
	github.com/asdine/storm v2.2.0+incompatible
	github.com/decred/dcrd/chaincfg/v2 v2.0.2
	github.com/decred/dcrd/rpc/jsonrpc/types v1.0.0
	github.com/decred/dcrdata/db/dbtypes/v2 v2.1.1
	github.com/decred/dcrdata/semver v1.0.0
	github.com/decred/politeia v0.0.0-20190716200512-2a928f2c2084
	github.com/decred/slog v1.0.0
)

replace github.com/golang/lint => golang.org/x/lint v0.0.0-20190301231843-5614ed5bae6f

replace sourcegraph.com/sourcegraph/go-diff => github.com/sourcegraph/go-diff v0.5.1
