module github.com/decred/dcrdata/db/dcrsqlite/v4

go 1.11

require (
	github.com/decred/dcrd/blockchain/stake v1.1.0
	github.com/decred/dcrd/chaincfg/chainhash v1.0.1
	github.com/decred/dcrd/chaincfg/v2 v2.0.2
	github.com/decred/dcrd/dcrutil/v2 v2.0.0
	github.com/decred/dcrd/rpc/jsonrpc/types v1.0.0
	github.com/decred/dcrd/rpcclient/v4 v4.0.0
	github.com/decred/dcrd/wire v1.2.0
	github.com/decred/dcrdata/api/types/v4 v4.0.1
	github.com/decred/dcrdata/blockdata/v4 v4.0.2
	github.com/decred/dcrdata/db/cache/v2 v2.2.1
	github.com/decred/dcrdata/db/dbtypes/v2 v2.1.1
	github.com/decred/dcrdata/explorer/types/v2 v2.0.1
	github.com/decred/dcrdata/mempool/v4 v4.0.2
	github.com/decred/dcrdata/rpcutils/v2 v2.0.2
	github.com/decred/dcrdata/stakedb/v3 v3.0.2
	github.com/decred/dcrdata/testutil/dbconfig/v2 v2.0.0
	github.com/decred/dcrdata/txhelpers/v3 v3.0.1
	github.com/decred/slog v1.0.0
	github.com/dustin/go-humanize v1.0.0
	github.com/mattn/go-sqlite3 v1.10.0
)
