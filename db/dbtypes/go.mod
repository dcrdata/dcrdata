module github.com/decred/dcrdata/db/dbtypes/v2

go 1.12

replace github.com/decred/dcrdata/txhelpers/v4 => ../../txhelpers

require (
	github.com/decred/dcrd/blockchain/stake/v2 v2.0.2
	github.com/decred/dcrd/chaincfg/chainhash v1.0.2
	github.com/decred/dcrd/chaincfg/v2 v2.3.0
	github.com/decred/dcrd/dcrutil/v2 v2.0.1
	github.com/decred/dcrd/txscript/v2 v2.1.0
	github.com/decred/dcrd/wire v1.3.0
	github.com/decred/dcrdata/txhelpers/v4 v4.0.1
	github.com/decred/dcrwallet/wallet/v3 v3.1.1-0.20191230143837-6a86dc4676f0
)
