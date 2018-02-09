// Copyright (c) 2018, The dcrdata developers.
// Copyright (c) 2017, Jonathan Chappelow
// See LICENSE for details.

package stakedb

import (
	"encoding/binary"
	"fmt"
	"os"
	"strings"
	"sync"

	apitypes "github.com/dcrdata/dcrdata/dcrdataapi"
	"github.com/dcrdata/dcrdata/rpcutils"
	"github.com/dcrdata/dcrdata/txhelpers"
	"github.com/decred/dcrd/blockchain/stake"
	"github.com/decred/dcrd/chaincfg"
	"github.com/decred/dcrd/chaincfg/chainhash"
	"github.com/decred/dcrd/database"
	"github.com/decred/dcrd/dcrutil"
	"github.com/decred/dcrd/rpcclient"
	"github.com/decred/dcrd/wire"
	"github.com/oleiade/lane"
)

// PoolInfoCache contains a map of block hashes to ticket pool info data at that
// block height.
type PoolInfoCache struct {
	sync.RWMutex
	poolInfo    map[chainhash.Hash]*apitypes.TicketPoolInfo
	expireQueue *lane.Queue
	maxSize     int
}

// NewPoolInfoCache constructs a new PoolInfoCache, and is needed to initialize
// the internal map.
func NewPoolInfoCache(size int) *PoolInfoCache {
	return &PoolInfoCache{
		poolInfo:    make(map[chainhash.Hash]*apitypes.TicketPoolInfo),
		expireQueue: lane.NewQueue(),
		maxSize:     size,
	}
}

// Get attempts to fetch the ticket pool info for a given block hash, returning
// a *apitypes.TicketPoolInfo, and a bool indicating if the hash was found in
// the map.
func (c *PoolInfoCache) Get(hash chainhash.Hash) (*apitypes.TicketPoolInfo, bool) {
	c.RLock()
	defer c.RUnlock()
	tpi, ok := c.poolInfo[hash]
	return tpi, ok
}

// Set stores the ticket pool info for the given hash in the pool info cache.
func (c *PoolInfoCache) Set(hash chainhash.Hash, p *apitypes.TicketPoolInfo) {
	c.Lock()
	defer c.Unlock()
	c.poolInfo[hash] = p
	c.expireQueue.Enqueue(hash)
	if c.expireQueue.Size() >= c.maxSize {
		expireHash := c.expireQueue.Dequeue().(chainhash.Hash)
		delete(c.poolInfo, expireHash)
	}
}

func (c *PoolInfoCache) SetCapacity(cap int) error {
	c.Lock()
	defer c.Unlock()
	c.maxSize = cap
	for c.expireQueue.Size() >= c.maxSize {
		expireHash, ok := c.expireQueue.Dequeue().(chainhash.Hash)
		if !ok {
			return fmt.Errorf("failed to reduce pool cache capacity")
		}
		delete(c.poolInfo, expireHash)
	}
	return nil
}

// StakeDatabase models data for the stake database
type StakeDatabase struct {
	params          *chaincfg.Params
	NodeClient      *rpcclient.Client
	nodeMtx         sync.RWMutex
	StakeDB         database.DB
	BestNode        *stake.Node
	blkMtx          sync.RWMutex
	blockCache      map[int64]*dcrutil.Block
	liveTicketMtx   sync.Mutex
	liveTicketCache map[chainhash.Hash]int64
	poolInfo        *PoolInfoCache
	PoolDB          *TicketPool
}

const (
	// dbType is the database backend type to use
	dbType = "ffldb"
	// DefaultStakeDbName is the default database name
	DefaultStakeDbName = "ffldb_stake"
)

// NewStakeDatabase creates a StakeDatabase instance, opening or creating a new
// ffldb-backed stake database, and loads all live tickets into a cache.
func NewStakeDatabase(client *rpcclient.Client, params *chaincfg.Params,
	dbNameOpt ...string) (*StakeDatabase, error) {
	log.Infof("Loading ticket pool DB...")
	poolDB, err := NewTicketPool("stakedb_ticket_pool.db")
	if err != nil {
		return nil, fmt.Errorf("unable to open ticket pool DB: %v", err)
	}
	sDB := &StakeDatabase{
		params:          params,
		NodeClient:      client,
		blockCache:      make(map[int64]*dcrutil.Block),
		liveTicketCache: make(map[chainhash.Hash]int64),
		poolInfo:        NewPoolInfoCache(513),
		PoolDB:          poolDB,
	}

	// Put the genesis block in the pool info cache since stakedb starts with
	// genesis. Hence it will never be connected, how TPI is usually cached.
	sDB.poolInfo.Set(*params.GenesisHash, &apitypes.TicketPoolInfo{})

	dbName := DefaultStakeDbName
	if len(dbNameOpt) > 0 {
		dbName = dbNameOpt[0]
	}

	if err := sDB.Open(dbName); err != nil {
		return nil, err
	}

	nodeHeight, err := client.GetBlockCount()
	if err != nil {
		log.Errorf("Unable to get best block height: %v", err)
	}

	if int64(sDB.Height()) != sDB.PoolDB.Tip() {
		return nil, fmt.Errorf("StakeDB height (%d) and TicketPool (%d) height not equal."+
			"Delete both and try again", sDB.Height(), sDB.PoolDB.Tip())
	}

	log.Infof("Advancing ticket pool DB to tip via diffs...")
	if err = poolDB.AdvanceToTip(); err != nil {
		return nil, fmt.Errorf("failed to advance ticket pool DB to tip: %v", err)
	}

	if int64(sDB.Height()) >= nodeHeight-int64(params.TicketPoolSize)/4 {

		liveTickets, err := sDB.NodeClient.LiveTickets()
		if err != nil {
			return sDB, err
		}

		log.Info("Pre-populating live ticket cache...")

		type promiseGetRawTransaction struct {
			result rpcclient.FutureGetRawTransactionResult
			ticket *chainhash.Hash
		}
		promisesGetRawTransaction := make([]promiseGetRawTransaction, 0, len(liveTickets))

		// Send all the live ticket requests
		for _, hash := range liveTickets {
			promisesGetRawTransaction = append(promisesGetRawTransaction, promiseGetRawTransaction{
				result: sDB.NodeClient.GetRawTransactionAsync(hash),
				ticket: hash,
			})
		}

		// Receive the live ticket tx results
		for _, p := range promisesGetRawTransaction {
			ticketTx, err := p.result.Receive()
			if err != nil {
				log.Error(err)
				continue
			}
			if !ticketTx.Hash().IsEqual(p.ticket) {
				panic(fmt.Sprintf("Failed to receive Tx details for requested ticket hash: %v, %v", p.ticket, ticketTx.Hash()))
			}

			sDB.liveTicketCache[*p.ticket] = ticketTx.MsgTx().TxOut[0].Value

			// txHeight := ticketTx.BlockHeight
			// unconfirmed := (txHeight == 0)
			// immature := (tipHeight-int32(txHeight) < int32(w.ChainParams().TicketMaturity))
		}

		// Old synchronous way
		// for _, hash := range liveTickets {
		// 	var txid *dcrutil.Tx
		// 	txid, err = sDB.NodeClient.GetRawTransaction(hash)
		// 	if err != nil {
		// 		log.Errorf("Unable to get transaction %v: %v\n", hash, err)
		// 		continue
		// 	}
		// 	// This isn't quite right for pool tickets where the small
		// 	// pool fees are included in vout[0], but it's close.
		// 	sDB.liveTicketCache[*hash] = txid.MsgTx().TxOut[0].Value
		// }
	}

	return sDB, nil
}

// Height gets the block height of the best stake node.  It is thread-safe,
// unlike using db.BestNode.Height(), and checks that the stake database is
// opened first.
func (db *StakeDatabase) Height() uint32 {
	db.nodeMtx.RLock()
	defer db.nodeMtx.RUnlock()
	if db == nil || db.BestNode == nil {
		log.Error("Stake database not yet opened")
		return 0
	}
	return db.BestNode.Height()
}

// block first tries to find the block at the input height in cache, and if that
// fails it will request it from the node RPC client. Don't use this casually
// since reorganization may redefine a block at a given height.
func (db *StakeDatabase) block(ind int64) (*dcrutil.Block, bool) {
	db.blkMtx.RLock()
	block, ok := db.blockCache[ind]
	db.blkMtx.RUnlock()
	//log.Info(ind, block, ok)
	if !ok {
		var err error
		block, _, err = rpcutils.GetBlock(ind, db.NodeClient)
		if err != nil {
			log.Error(err)
			return nil, false
		}
	}
	return block, ok
}

// ForgetBlock deletes the block with the input height from the block cache.
func (db *StakeDatabase) ForgetBlock(ind int64) {
	db.blkMtx.Lock()
	defer db.blkMtx.Unlock()
	delete(db.blockCache, ind)
}

// ConnectBlockHash is a wrapper for ConnectBlock. For the input block hash, it
// gets the block from the node RPC client and calls ConnectBlock.
func (db *StakeDatabase) ConnectBlockHash(hash *chainhash.Hash) (*dcrutil.Block, error) {
	msgBlock, err := db.NodeClient.GetBlock(hash)
	if err != nil {
		return nil, err
	}
	block := dcrutil.NewBlock(msgBlock)
	return block, db.ConnectBlock(block)
}

// ConnectBlock connects the input block to the tip of the stake DB and updates
// the best stake node. This exported function gets any revoked and spend
// tickets from the input block, and any maturing tickets from the past block in
// which those tickets would be found, and passes them to connectBlock.
func (db *StakeDatabase) ConnectBlock(block *dcrutil.Block) error {
	height := block.Height()
	maturingHeight := height - int64(db.params.TicketMaturity)

	var maturingTickets []chainhash.Hash
	if maturingHeight >= 0 {
		maturingBlock, wasCached := db.block(maturingHeight)
		if wasCached {
			db.ForgetBlock(maturingHeight)
		}
		maturingTickets, _ = txhelpers.TicketsInBlock(maturingBlock)
	}

	db.blkMtx.Lock()
	db.blockCache[block.Height()] = block
	db.blkMtx.Unlock()

	revokedTickets := txhelpers.RevokedTicketsInBlock(block)
	votedTickets := txhelpers.TicketsSpentInBlock(block)

	db.nodeMtx.Lock()
	bestNodeHeight := int64(db.BestNode.Height())
	db.nodeMtx.Unlock()
	if height <= bestNodeHeight {
		return fmt.Errorf("cannot connect block height %d at height %d", height, bestNodeHeight)
	}

	// who is supposed to vote on this block
	winners := db.BestNode.Winners()

	// connect it
	err := db.connectBlock(block, votedTickets, revokedTickets, maturingTickets)
	if err != nil {
		return err
	}

	// Update the ticket pool db. Determine newly missed and expired tickets.
	justMissed := db.BestNode.MissedByBlock() // includes expired
	var expiring []chainhash.Hash
	for i := range justMissed {
		if db.BestNode.ExistsExpiredTicket(justMissed[i]) {
			if !db.BestNode.ExistsRevokedTicket(justMissed[i]) {
				expiring = append(expiring, justMissed[i])
			}
		}
	}

	// Tickets leaving the live ticket pool = winners + expires
	liveOut := append(winners, expiring...)
	// Tickets entering the pool = maturing tickets
	poolDiff := &PoolDiff{
		In:  maturingTickets,
		Out: liveOut,
	}

	// Append this ticket pool diff
	return db.PoolDB.Append(poolDiff, bestNodeHeight+1)
}

func (db *StakeDatabase) connectBlock(block *dcrutil.Block, spent []chainhash.Hash,
	revoked []chainhash.Hash, maturing []chainhash.Hash) error {
	db.nodeMtx.Lock()

	cleanLiveTicketCache := func() {
		db.liveTicketMtx.Lock()
		for i := range spent {
			delete(db.liveTicketCache, spent[i])
		}
		for i := range revoked {
			delete(db.liveTicketCache, revoked[i])
		}
		db.liveTicketMtx.Unlock()
	}
	defer cleanLiveTicketCache()

	var err error
	db.BestNode, err = db.BestNode.ConnectNode(block.MsgBlock().Header,
		spent, revoked, maturing)
	if err != nil {
		return err
	}

	if err = db.StakeDB.Update(func(dbTx database.Tx) error {
		return stake.WriteConnectedBestNode(dbTx, db.BestNode, *block.Hash())
	}); err != nil {
		return err
	}

	db.nodeMtx.Unlock()

	// Get ticket pool info at current best (just connected in stakedb) block,
	// and store it in the StakeDatabase's PoolInfoCache.
	db.poolInfo.Set(*block.Hash(), db.PoolInfoBest())

	return err
}

func (db *StakeDatabase) SetPoolInfo(blockHash chainhash.Hash, tpi *apitypes.TicketPoolInfo) {
	db.poolInfo.Set(blockHash, tpi)
}

func (db *StakeDatabase) SetPoolCacheCapacity(cap int) error {
	return db.poolInfo.SetCapacity(cap)
}

// DisconnectBlock attempts to disconnect the current best block from the stake
// DB and updates the best stake node.
func (db *StakeDatabase) DisconnectBlock() error {
	db.nodeMtx.Lock()
	defer db.nodeMtx.Unlock()

	return db.disconnectBlock()
}

// disconnectBlock is the non-thread-safe version of DisconnectBlock.
func (db *StakeDatabase) disconnectBlock() error {
	childHeight := db.BestNode.Height()
	parentBlock, err := db.dbPrevBlock()
	if err != nil {
		return err
	}
	if parentBlock.Height() != int64(childHeight)-1 {
		panic("BestNode and stake DB are inconsistent")
	}

	// Trim the best block from the ticket pool db
	poolDBTip := db.PoolDB.Trim()
	if poolDBTip != parentBlock.Height() {
		log.Warnf("Pool DB tip (%d) not equal to stakeDB height (%d)!",
			poolDBTip, parentBlock.Height())
		// Try to recover by trimming further if a previous disconnect failed
		for poolDBTip > parentBlock.Height() {
			poolDBTip = db.PoolDB.Trim()
		}
		if poolDBTip != parentBlock.Height() {
			log.Errorf("Unable to trim pool DB!")
		}
	}

	childUndoData := append(stake.UndoTicketDataSlice(nil), db.BestNode.UndoData()...)

	log.Debugf("Disconnecting block %d.", childHeight)

	// previous best node
	var parentStakeNode *stake.Node
	err = db.StakeDB.View(func(dbTx database.Tx) error {
		var errLocal error
		parentStakeNode, errLocal = db.BestNode.DisconnectNode(
			parentBlock.MsgBlock().Header, nil, nil, dbTx)
		return errLocal
	})
	if err != nil {
		return err
	}
	db.BestNode = parentStakeNode

	return db.StakeDB.Update(func(dbTx database.Tx) error {
		return stake.WriteDisconnectedBestNode(dbTx, parentStakeNode,
			*parentBlock.Hash(), childUndoData)
	})
}

// DisconnectBlocks disconnects N blocks from the head of the chain.
func (db *StakeDatabase) DisconnectBlocks(count int64) error {
	db.nodeMtx.Lock()
	defer db.nodeMtx.Unlock()

	for i := int64(0); i < count; i++ {
		if err := db.disconnectBlock(); err != nil {
			return err
		}
	}

	return nil
}

// Open attempts to open an existing stake database, and will create a new one
// if one does not exist.
func (db *StakeDatabase) Open(dbName string) error {
	db.nodeMtx.Lock()
	defer db.nodeMtx.Unlock()

	// Create a new database to store the accepted stake node data into.
	var isFreshDB bool
	var err error
	db.StakeDB, err = database.Open(dbType, dbName, db.params.Net)
	if err != nil {
		if strings.Contains(err.Error(), "resource temporarily unavailable") ||
			strings.Contains(err.Error(), "is being used by another process") {
			return fmt.Errorf("Stake DB already opened. dcrdata running?")
		}
		if strings.Contains(err.Error(), "does not exist") {
			log.Info("Creating new stake DB.")
		} else {
			log.Infof("Unable to open stake DB (%v). Removing and creating new.", err)
			_ = os.RemoveAll(dbName)
		}

		db.StakeDB, err = database.Create(dbType, dbName, db.params.Net)
		if err != nil {
			// do not return nil interface, but interface of nil DB
			return fmt.Errorf("error creating database.DB: %v", err)
		}
		isFreshDB = true
	}

	// Load the best block from stake db
	err = db.StakeDB.View(func(dbTx database.Tx) error {
		v := dbTx.Metadata().Get([]byte("stakechainstate"))
		if v == nil {
			return fmt.Errorf("missing key for chain state data")
		}

		var stakeDBHash chainhash.Hash
		copy(stakeDBHash[:], v[:chainhash.HashSize])
		offset := chainhash.HashSize
		stakeDBHeight := binary.LittleEndian.Uint32(v[offset : offset+4])

		var errLocal error
		msgBlock, errLocal := db.NodeClient.GetBlock(&stakeDBHash)
		if errLocal != nil {
			return fmt.Errorf("GetBlock failed (%s): %v", stakeDBHash, errLocal)
		}
		header := msgBlock.Header

		db.BestNode, errLocal = stake.LoadBestNode(dbTx, stakeDBHeight,
			stakeDBHash, header, db.params)
		return errLocal
	})
	if err != nil {
		if !isFreshDB {
			log.Errorf("Error reading from database (%v).  Reinitializing.", err)
		}
		err = db.StakeDB.Update(func(dbTx database.Tx) error {
			var errLocal error
			db.BestNode, errLocal = stake.InitDatabaseState(dbTx, db.params)
			return errLocal
		})
		log.Debug("Initialized new stake db.")
	} else {
		log.Debug("Opened existing stake db.")
	}

	return err
}

// Close will close the ticket pool and stake databases.
func (db *StakeDatabase) Close() error {
	err1 := db.PoolDB.Close()
	err2 := db.StakeDB.Close()
	if err1 == nil {
		return err2
	}
	if err2 == nil {
		return err1
	}
	return fmt.Errorf("%v + %v", err1, err2)
}

func (db *StakeDatabase) expires() ([]chainhash.Hash, []bool) {
	// revoked includes expired ticket and missed votes that were revoked
	revoked := db.BestNode.RevokedTickets()
	// unrevoked includes expired and missed that have not been revoked
	unrevoked := db.BestNode.MissedTickets()

	var expires []chainhash.Hash
	var spent []bool
	for _, tkt := range revoked {
		if db.BestNode.ExistsExpiredTicket(*tkt) {
			expires = append(expires, *tkt)
			spent = append(spent, true)
		}
	}
	for _, tkt := range unrevoked {
		if db.BestNode.ExistsExpiredTicket(tkt) {
			expires = append(expires, tkt)
			spent = append(spent, false)
		}
	}
	return expires, spent
}

// PoolInfoBest computes ticket pool value using the database and, if needed, the
// node RPC client to fetch ticket values that are not cached. Returned are a
// structure including ticket pool value, size, and average value.
func (db *StakeDatabase) PoolInfoBest() *apitypes.TicketPoolInfo {
	db.nodeMtx.RLock()
	poolSize := db.BestNode.PoolSize()
	liveTickets := db.BestNode.LiveTickets()
	winningTickets := db.BestNode.Winners()
	height := db.BestNode.Height()
	// expiredTickets, expireRevoked := db.expires()
	db.nodeMtx.RUnlock()

	db.liveTicketMtx.Lock()
	var poolValue int64
	for _, hash := range liveTickets {
		val, ok := db.liveTicketCache[hash]
		if !ok {
			tx, err := db.NodeClient.GetRawTransaction(&hash)
			if err != nil {
				log.Errorf("Unable to get transaction %v: %v\n", hash, err)
				continue
			}
			// This isn't quite right for pool tickets where the small
			// pool fees are included in vout[0], but it's close.
			val = tx.MsgTx().TxOut[0].Value
			db.liveTicketCache[hash] = val
		}
		poolValue += val
	}
	db.liveTicketMtx.Unlock()

	// header, _ := db.DBTipBlockPoolInfoBestHeader()
	// if int(header.PoolSize) != len(liveTickets) {
	// 	log.Infof("Header at %d, DB at %d.", header.Height, db.BestNode.Height())
	// 	log.Warnf("Inconsistent pool sizes: %d, %d", header.PoolSize, len(liveTickets))
	// }

	poolCoin := dcrutil.Amount(poolValue).ToCoin()
	valAvg := 0.0
	if len(liveTickets) > 0 {
		valAvg = poolCoin / float64(poolSize)
	}

	winners := make([]string, 0, len(winningTickets))
	for _, winner := range winningTickets {
		winners = append(winners, winner.String())
	}

	return &apitypes.TicketPoolInfo{
		Height:  height,
		Size:    uint32(poolSize),
		Value:   poolCoin,
		ValAvg:  valAvg,
		Winners: winners,
	}
}

// PoolInfo attempts to fetch the ticket pool info for the specified block hash
// from an internal pool info cache. If it is not found, you should attempt to
// use PoolInfoBest if the target block is at the tip of the chain.
func (db *StakeDatabase) PoolInfo(hash chainhash.Hash) (*apitypes.TicketPoolInfo, bool) {
	return db.poolInfo.Get(hash)
}

// PoolSize returns the ticket pool size in the best node of the stake database
func (db *StakeDatabase) PoolSize() int {
	return db.BestNode.PoolSize()
}

// PoolAtHeight gets the entire list of live tickets at the given chain height.
func (db *StakeDatabase) PoolAtHeight(height int64) ([]chainhash.Hash, error) {
	return db.PoolDB.Pool(height)
}

// PoolAtHash gets the entire list of live tickets at the given block hash.
func (db *StakeDatabase) PoolAtHash(hash chainhash.Hash) ([]chainhash.Hash, error) {
	header, err := db.NodeClient.GetBlockHeader(&hash)
	if err != nil {
		return nil, fmt.Errorf("GetBlockHeader failed: %v", err)
	}
	return db.PoolDB.Pool(int64(header.Height))
}

// DBState queries the stake database for the best block height and hash.
func (db *StakeDatabase) DBState() (uint32, *chainhash.Hash, error) {
	db.nodeMtx.RLock()
	defer db.nodeMtx.RUnlock()

	return db.dbState()
}

func (db *StakeDatabase) dbState() (uint32, *chainhash.Hash, error) {
	var stakeDBHeight uint32
	var stakeDBHash chainhash.Hash
	err := db.StakeDB.View(func(dbTx database.Tx) error {
		v := dbTx.Metadata().Get([]byte("stakechainstate"))
		if v == nil {
			return fmt.Errorf("missing key for chain state data")
		}

		copy(stakeDBHash[:], v[:chainhash.HashSize])
		offset := chainhash.HashSize
		stakeDBHeight = binary.LittleEndian.Uint32(v[offset : offset+4])

		return nil
	})
	return stakeDBHeight, &stakeDBHash, err
}

// DBTipBlockHeader gets the block header for the current best block in the
// stake database. It used DBState to get the best block hash, and the node RPC
// client to get the header.
func (db *StakeDatabase) DBTipBlockHeader() (*wire.BlockHeader, error) {
	_, hash, err := db.DBState()
	if err != nil {
		return nil, err
	}

	return db.NodeClient.GetBlockHeader(hash)
}

// DBPrevBlockHeader gets the block header for the previous best block in the
// stake database. It used DBState to get the best block hash, and the node RPC
// client to get the header.
func (db *StakeDatabase) DBPrevBlockHeader() (*wire.BlockHeader, error) {
	_, hash, err := db.DBState()
	if err != nil {
		return nil, err
	}

	parentHeader, err := db.NodeClient.GetBlockHeader(hash)
	if err != nil {
		return nil, err
	}

	return db.NodeClient.GetBlockHeader(&parentHeader.PrevBlock)
}

// DBTipBlock gets the dcrutil.Block for the current best block in the stake
// database. It used DBState to get the best block hash, and the node RPC client
// to get the block itself.
func (db *StakeDatabase) DBTipBlock() (*dcrutil.Block, error) {
	_, hash, err := db.DBState()
	if err != nil {
		return nil, err
	}

	return db.getBlock(hash)
}

// DBPrevBlock gets the dcrutil.Block for the previous best block in the stake
// database. It used DBState to get the best block hash, and the node RPC client
// to get the block itself.
func (db *StakeDatabase) DBPrevBlock() (*dcrutil.Block, error) {
	_, hash, err := db.DBState()
	if err != nil {
		return nil, err
	}

	parentHeader, err := db.NodeClient.GetBlockHeader(hash)
	if err != nil {
		return nil, err
	}

	return db.getBlock(&parentHeader.PrevBlock)
}

// dbPrevBlock is the non-thread-safe version of DBPrevBlock.
func (db *StakeDatabase) dbPrevBlock() (*dcrutil.Block, error) {
	_, hash, err := db.dbState()
	if err != nil {
		return nil, err
	}

	parentHeader, err := db.NodeClient.GetBlockHeader(hash)
	if err != nil {
		return nil, err
	}

	return db.getBlock(&parentHeader.PrevBlock)
}

func (db *StakeDatabase) getBlock(hash *chainhash.Hash) (*dcrutil.Block, error) {
	msgBlock, err := db.NodeClient.GetBlock(hash)
	if err == nil {
		return dcrutil.NewBlock(msgBlock), nil
	}
	return nil, err
}
