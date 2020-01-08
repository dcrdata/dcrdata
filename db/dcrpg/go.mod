module github.com/decred/dcrdata/db/dcrpg/v5

go 1.12

replace (
	github.com/decred/dcrdata/db/cache/v3 => ../cache
	github.com/decred/dcrdata/db/dbtypes/v2 => ../dbtypes
	github.com/decred/dcrdata/explorer/types/v2 => ../../explorer/types
	github.com/decred/dcrdata/mempool/v5 => ../../mempool
	github.com/decred/dcrdata/txhelpers/v4 => ../../txhelpers
)

require (
	github.com/chappjc/trylock v1.0.0
	github.com/davecgh/go-spew v1.1.1
	github.com/decred/dcrd/blockchain/stake/v2 v2.0.2
	github.com/decred/dcrd/chaincfg/chainhash v1.0.2
	github.com/decred/dcrd/chaincfg/v2 v2.3.0
	github.com/decred/dcrd/dcrutil/v2 v2.0.1
	github.com/decred/dcrd/rpc/jsonrpc/types/v2 v2.0.0
	github.com/decred/dcrd/rpcclient/v5 v5.0.0
	github.com/decred/dcrd/txscript/v2 v2.1.0
	github.com/decred/dcrd/wire v1.3.0
	github.com/decred/dcrdata/api/types/v5 v5.0.1
	github.com/decred/dcrdata/blockdata/v5 v5.0.1
	github.com/decred/dcrdata/db/cache/v3 v3.0.1
	github.com/decred/dcrdata/db/dbtypes/v2 v2.2.1
	github.com/decred/dcrdata/explorer/types/v2 v2.1.1
	github.com/decred/dcrdata/mempool/v5 v5.0.1
	github.com/decred/dcrdata/rpcutils/v3 v3.0.1
	github.com/decred/dcrdata/semver v1.0.0
	github.com/decred/dcrdata/stakedb/v3 v3.1.1
	github.com/decred/dcrdata/testutil/dbconfig/v2 v2.0.0
	github.com/decred/dcrdata/txhelpers/v4 v4.0.1
	github.com/decred/dcrwallet/wallet/v3 v3.1.1-0.20191230143837-6a86dc4676f0
	github.com/decred/slog v1.0.0
	github.com/dmigwi/go-piparser/proposals v0.0.0-20191219171828-ae8cbf4067e1
	github.com/dustin/go-humanize v1.0.0
	github.com/lib/pq v1.2.0
)
