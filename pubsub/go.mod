module github.com/decred/dcrdata/pubsub

go 1.11

replace github.com/decred/dcrdata/pubsub/types => ./types

require (
	github.com/decred/dcrd/chaincfg v1.3.0
	github.com/decred/dcrd/dcrjson/v2 v2.0.0
	github.com/decred/dcrd/dcrutil v1.2.1-0.20190118223730-3a5281156b73
	github.com/decred/dcrd/txscript v1.0.2
	github.com/decred/dcrd/wire v1.2.0
	github.com/decred/dcrdata/blockdata v1.0.1
	github.com/decred/dcrdata/db/dbtypes v1.0.1
	github.com/decred/dcrdata/explorer/types v1.0.0
	github.com/decred/dcrdata/mempool v1.0.0
	github.com/decred/dcrdata/pubsub/types v1.0.0
	github.com/decred/dcrdata/txhelpers v1.0.1
	github.com/decred/slog v1.0.0
	golang.org/x/net v0.0.0-20190326090315-15845e8f865b
)
