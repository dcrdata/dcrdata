module github.com/decred/dcrdata/db/dcrpg/v4

go 1.12

require (
	github.com/chappjc/trylock v1.0.0
	github.com/davecgh/go-spew v1.1.1
	github.com/decred/dcrd/blockchain/stake/v2 v2.0.1
	github.com/decred/dcrd/chaincfg/chainhash v1.0.1
	github.com/decred/dcrd/chaincfg/v2 v2.2.0
	github.com/decred/dcrd/dcrutil/v2 v2.0.0
	github.com/decred/dcrd/rpc/jsonrpc/types v1.0.0
	github.com/decred/dcrd/rpcclient/v4 v4.0.0
	github.com/decred/dcrd/txscript/v2 v2.0.0
	github.com/decred/dcrd/wire v1.2.0
	github.com/decred/dcrdata/api/types/v4 v4.0.2
	github.com/decred/dcrdata/blockdata/v4 v4.0.3
	github.com/decred/dcrdata/db/cache/v2 v2.2.2
	github.com/decred/dcrdata/db/dbtypes/v2 v2.1.2
	github.com/decred/dcrdata/explorer/types/v2 v2.0.2
	github.com/decred/dcrdata/mempool/v4 v4.0.3
	github.com/decred/dcrdata/rpcutils/v2 v2.0.3
	github.com/decred/dcrdata/semver v1.0.0
	github.com/decred/dcrdata/stakedb/v3 v3.0.3
	github.com/decred/dcrdata/testutil/dbconfig/v2 v2.0.0
	github.com/decred/dcrdata/txhelpers/v3 v3.0.2
	github.com/decred/slog v1.0.0
	github.com/dmigwi/go-piparser/proposals v0.0.0-20190426030541-8412e0f44f55
	github.com/dustin/go-humanize v1.0.0
	github.com/lib/pq v1.1.0
)

replace (
	github.com/decred/dcrdata/db/cache/v2 => ../cache
	github.com/decred/dcrdata/db/dbtypes/v2 => ../dbtypes
	github.com/decred/dcrdata/stakedb/v3 => ../../stakedb
)
