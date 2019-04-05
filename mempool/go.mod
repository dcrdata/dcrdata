module github.com/decred/dcrdata/mempool

go 1.12

require (
	github.com/AndreasBriese/bbloom v0.0.0-20190306092124-e2d15f34fcf9 // indirect
	github.com/DataDog/zstd v1.3.5 // indirect
	github.com/decred/dcrd/blockchain v1.1.1
	github.com/decred/dcrd/blockchain/stake v1.1.0
	github.com/decred/dcrd/chaincfg v1.4.0
	github.com/decred/dcrd/chaincfg/chainhash v1.0.1
	github.com/decred/dcrd/dcrjson/v2 v2.0.0
	github.com/decred/dcrd/dcrutil v1.2.1-0.20190118223730-3a5281156b73
	github.com/decred/dcrd/rpcclient/v2 v2.0.0
	github.com/decred/dcrdata/api/types v1.0.6
	github.com/decred/dcrdata/db/dbtypes v1.0.1
	github.com/decred/dcrdata/db/dcrpg v1.0.0 // indirect
	github.com/decred/dcrdata/explorer/types v1.0.0
	github.com/decred/dcrdata/pubsub/types v1.0.0
	github.com/decred/dcrdata/rpcutils v1.0.1
	github.com/decred/dcrdata/txhelpers v1.0.1
	github.com/decred/slog v1.0.0
	github.com/dgryski/go-farm v0.0.0-20190323231341-8198c7b169ec // indirect
	github.com/dustin/go-humanize v1.0.0
	github.com/google/go-cmp v0.2.0 // indirect
	github.com/pkg/errors v0.8.1 // indirect
)

replace github.com/decred/dcrdata/txhelpers => ../txhelpers
