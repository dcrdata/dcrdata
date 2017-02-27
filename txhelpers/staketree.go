package txhelpers

import (
	"fmt"
	"os"

	"github.com/decred/dcrd/blockchain/stake"
	"github.com/decred/dcrd/chaincfg"
	"github.com/decred/dcrd/chaincfg/chainhash"
	"github.com/decred/dcrd/database"
	_ "github.com/decred/dcrd/database/ffldb"
	"github.com/decred/dcrrpcclient"
	"github.com/decred/dcrutil"
)

const (
	// dbType is the database backend type to use
	dbType = "ffldb"
	// DefaultStakeDbName is the default database name
	DefaultStakeDbName = "ffldb_stake"
)

//var netParams = &chaincfg.MainNetParams

func BuildStakeTree(blocks map[int64]*dcrutil.Block, netParams *chaincfg.Params,
	nodeClient *dcrrpcclient.Client, DBName ...string) (database.DB, []int64, error) {

	height := int64(len(blocks) - 1)

	// Create a new database to store the accepted stake node data into.
	dbName := DefaultStakeDbName
	if len(DBName) > 0 {
		dbName = DBName[0]
	}
	_ = os.RemoveAll(dbName)
	db, err := database.Create(dbType, dbName, netParams.Net)
	if err != nil {
		return db, nil, fmt.Errorf("error creating db: %v\n", err)
	}
	//defer db.Close()

	// Load the genesis block
	var bestNode *stake.Node
	err = db.Update(func(dbTx database.Tx) error {
		var errLocal error
		bestNode, errLocal = stake.InitDatabaseState(dbTx, netParams)
		return errLocal
	})
	if err != nil {
		db.Close()
		return nil, nil, err
	}

	// Cache all of our nodes so that we can check them when we start
	// disconnecting and going backwards through the blocks.
	poolValues := make([]int64, height+1)
	// a ticket treap would be nice, but a map will do for a cache
	liveTicketMap := make(map[chainhash.Hash]int64)
	err = db.Update(func(dbTx database.Tx) error {
		for i := int64(1); i <= height; i++ {
			block := blocks[i]
			header := block.MsgBlock().Header
			liveTickets := bestNode.LiveTickets()
			numLive := len(liveTickets)
			if int(header.PoolSize) != numLive {
				fmt.Printf("bad number of live tickets: want %v, got %v (%v)\n",
					header.PoolSize, numLive, numLive-int(header.PoolSize))
			}
			if header.FinalState != bestNode.FinalState() {
				fmt.Printf("bad final state: want %x, got %x\n",
					header.FinalState, bestNode.FinalState())
			}

			var amt int64
			for _, hash := range liveTickets {
				val, ok := liveTicketMap[hash]
				if !ok {
					txid, err := nodeClient.GetRawTransaction(&hash)
					if err != nil {
						fmt.Printf("Unable to get transaction %v: %v\n", hash, err)
						continue
					}
					// This isn't quite right for pool tickets where the small
					// pool fees are included in vout[0], but it's close.
					liveTicketMap[hash] = txid.MsgTx().TxOut[0].Value
				}
				amt += val
			}
			poolValues[i] = amt

			if i%100 == 0 {
				fmt.Printf("%d (%d, %d)\n", i, len(liveTicketMap), numLive)
			}

			var ticketsToAdd []chainhash.Hash
			if i >= netParams.StakeEnabledHeight {
				matureHeight := (i - int64(netParams.TicketMaturity))
				ticketsToAdd = TicketsInBlock(blocks[matureHeight])
			}

			spentTickets := TicketsSpentInBlock(block)
			for i := range spentTickets {
				delete(liveTicketMap, spentTickets[i])
			}
			revokedTickets := RevokedTicketsInBlock(block)
			for i := range revokedTickets {
				delete(liveTicketMap, revokedTickets[i])
			}

			bestNode, err = bestNode.ConnectNode(header,
				spentTickets, revokedTickets, ticketsToAdd)
			if err != nil {
				return fmt.Errorf("couldn't connect node: %v\n", err.Error())
			}

			// Write the new node to db.
			err = stake.WriteConnectedBestNode(dbTx, bestNode, *block.Hash())
			if err != nil {
				return fmt.Errorf("failure writing the best node: %v\n",
					err.Error())
			}
		}

		return nil
	})

	return db, poolValues, err
}

/// kang

// TicketsInBlock finds all the new tickets in the block.
func TicketsInBlock(bl *dcrutil.Block) []chainhash.Hash {
	tickets := make([]chainhash.Hash, 0)
	for _, stx := range bl.STransactions() {
		if stake.DetermineTxType(stx.MsgTx()) == stake.TxTypeSStx {
			h := stx.Hash()
			tickets = append(tickets, *h)
		}
	}

	return tickets
}

// TicketsSpentInBlock finds all the tickets spent in the block.
func TicketsSpentInBlock(bl *dcrutil.Block) []chainhash.Hash {
	tickets := make([]chainhash.Hash, 0)
	for _, stx := range bl.STransactions() {
		if stake.DetermineTxType(stx.MsgTx()) == stake.TxTypeSSGen {
			// Hash of the original STtx
			tickets = append(tickets, stx.MsgTx().TxIn[1].PreviousOutPoint.Hash)
		}
	}

	return tickets
}

// VotesInBlock finds all the votes in the block.
func VotesInBlock(bl *dcrutil.Block) []chainhash.Hash {
	votes := make([]chainhash.Hash, 0)
	for _, stx := range bl.STransactions() {
		if stake.DetermineTxType(stx.MsgTx()) == stake.TxTypeSSGen {
			h := stx.Hash()
			votes = append(votes, *h)
		}
	}

	return votes
}

// RevokedTicketsInBlock finds all the revoked tickets in the block.
func RevokedTicketsInBlock(bl *dcrutil.Block) []chainhash.Hash {
	tickets := make([]chainhash.Hash, 0)
	for _, stx := range bl.STransactions() {
		if stake.DetermineTxType(stx.MsgTx()) == stake.TxTypeSSRtx {
			tickets = append(tickets, stx.MsgTx().TxIn[0].PreviousOutPoint.Hash)
		}
	}

	return tickets
}
