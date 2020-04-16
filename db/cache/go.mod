module github.com/decred/dcrdata/db/cache/v3

go 1.12

replace (
	github.com/decred/dcrdata/db/dbtypes/v2 v2.2.1 => ../dbtypes
	github.com/decred/dcrdata/txhelpers/v4 v4.0.1 => ../../txhelpers
)

require (
	github.com/decred/dcrd/chaincfg/chainhash v1.0.2
	github.com/decred/dcrd/chaincfg/v2 v2.3.0
	github.com/decred/dcrdata/db/dbtypes/v2 v2.2.1
	github.com/decred/dcrdata/semver v1.0.0
	github.com/decred/dcrdata/txhelpers/v4 v4.0.1
	github.com/decred/slog v1.0.0
)
