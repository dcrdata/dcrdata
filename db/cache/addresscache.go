// Copyright (c) 2019, The Decred developers
// See LICENSE for details.

package cache

import (
	"fmt"
	"sync"

	"github.com/decred/dcrd/chaincfg/chainhash"
	apitypes "github.com/decred/dcrdata/v4/api/types"
	"github.com/decred/dcrdata/v4/db/dbtypes"
)

// CacheLock is a "try lock" for coordinating multiple accessors, while allowing
// only a single updater. Use NewCacheLock to create a CacheLock.
type CacheLock struct {
	sync.Mutex
	addrs map[string]chan struct{}
}

// NewCacheLock constructs a new CacheLock.
func NewCacheLock() *CacheLock {
	return &CacheLock{addrs: make(map[string]chan struct{})}
}

func (cl *CacheLock) done(addr string) {
	cl.Lock()
	delete(cl.addrs, addr)
	cl.Unlock()
}

func (cl *CacheLock) hold(addr string) func() {
	done := make(chan struct{})
	cl.addrs[addr] = done
	return func() {
		cl.done(addr)
		close(done)
	}
}

// TryLock will attempt to obtain either an exclusive updating lock. Trylock
// returns a bool, busy, indicating if another caller has already obtained the
// lock. When busy is false, the caller has obtained the exclusive lock, and the
// returned func(), done, should be called when ready to release the lock. When
// busy is true, the returned channel, wait, should be received from to block
// until the updater has released the lock.
func (cl *CacheLock) TryLock(addr string) (busy bool, wait chan struct{}, done func()) {
	cl.Lock()
	defer cl.Unlock()
	done = func() {}
	wait, busy = cl.addrs[addr]
	if !busy {
		done = cl.hold(addr)
	}
	return busy, wait, done
}

func countCreditDebitRows(rows []*dbtypes.AddressRow) (numCredit, numDebit int) {
	for _, r := range rows {
		if r.IsFunding {
			numCredit++
		} else {
			numDebit++
		}
	}
	return
}

func creditAddressRows(rows []*dbtypes.AddressRow, N, offset int) []*dbtypes.AddressRow {
	if offset >= len(rows) {
		return nil
	}

	numCreditRows, _ := countCreditDebitRows(rows)
	if numCreditRows < N {
		N = numCreditRows
	}
	if offset >= numCreditRows {
		return nil
	}

	var skipped int
	out := make([]*dbtypes.AddressRow, 0, N)
	for _, r := range rows {
		if !r.IsFunding {
			continue
		}
		if skipped < offset {
			skipped++
			continue
		}
		// Append this row, and break the loop if we have N rows.
		out = append(out, r)
		if len(out) == N {
			break
		}
	}
	return out
}

func debitAddressRows(rows []*dbtypes.AddressRow, N, offset int) []*dbtypes.AddressRow {
	_, numDebitRows := countCreditDebitRows(rows)
	if numDebitRows < N {
		N = numDebitRows
	}
	var skipped int
	out := make([]*dbtypes.AddressRow, 0, N)
	for _, r := range rows {
		if r.IsFunding {
			continue
		}
		if skipped < offset {
			skipped++
			continue
		}
		// Append this row, and break the loop if we have N rows.
		out = append(out, r)
		if len(out) == N {
			break
		}
	}
	return out
}

func allCreditAddressRows(rows []*dbtypes.AddressRow) []*dbtypes.AddressRow {
	numCreditRows, _ := countCreditDebitRows(rows)
	out := make([]*dbtypes.AddressRow, numCreditRows)
	for i, r := range rows {
		if r.IsFunding {
			out[i] = r
		}
	}
	return out
}

func allDebitAddressRows(rows []*dbtypes.AddressRow) []*dbtypes.AddressRow {
	_, numDebitRows := countCreditDebitRows(rows)
	out := make([]*dbtypes.AddressRow, numDebitRows)
	for i, r := range rows {
		if !r.IsFunding {
			out[i] = r
		}
	}
	return out
}

// AddressCacheItem is the unit of cached data pertaining to a certain address.
// The height and hash of the best block at the time the data was obtained is
// stored to determine validity of the cache item. Cached data for an address
// are: balance, all non-merged address table rows, all merged address table
// rows, all UTXOs, and address metrics.
type AddressCacheItem struct {
	sync.RWMutex
	balance    *dbtypes.AddressBalance
	rows       []*dbtypes.AddressRow // creditDebitQuery
	rowsMerged []*dbtypes.AddressRow // mergedQuery
	utxos      []apitypes.AddressTxnOutput
	metrics    *dbtypes.AddressMetrics
	height     int64
	hash       chainhash.Hash
}

// BlockID provides basic identifying information about a block.
type BlockID struct {
	Hash   chainhash.Hash
	Height int64
}

// NewBlockID constructs a new BlockID.
func NewBlockID(hash *chainhash.Hash, height int64) *BlockID {
	return &BlockID{
		Hash:   *hash,
		Height: height,
	}
}

// blockID generates a BlockID for the AddressCacheItem.
func (d *AddressCacheItem) blockID() *BlockID {
	return &BlockID{d.hash, d.height}
}

// BlockHash is a thread-safe accessor for the block hash.
func (d *AddressCacheItem) BlockHash() chainhash.Hash {
	d.RLock()
	defer d.RUnlock()
	return d.hash
}

// BlockHeight is a thread-safe accessor for the block height.
func (d *AddressCacheItem) BlockHeight() int64 {
	d.RLock()
	defer d.RUnlock()
	return d.height
}

// Balance is a thread-safe accessor for the *dbtypes.AddressBalance.
func (d *AddressCacheItem) Balance() (*dbtypes.AddressBalance, *BlockID) {
	d.RLock()
	defer d.RUnlock()
	return d.balance, d.blockID()
}

// UTXOs is a thread-safe accessor for the []apitypes.AddressTxnOutput.
func (d *AddressCacheItem) UTXOs() ([]apitypes.AddressTxnOutput, *BlockID) {
	d.RLock()
	defer d.RUnlock()
	return d.utxos, d.blockID()
}

// Metrics is a thread-safe accessor for the *dbtypes.AddressMetrics.
func (d *AddressCacheItem) Metrics() (*dbtypes.AddressMetrics, *BlockID) {
	d.RLock()
	defer d.RUnlock()
	return d.metrics, d.blockID()
}

// Rows is a thread-safe accessor for the []*dbtypes.AddressRow.
func (d *AddressCacheItem) Rows() ([]*dbtypes.AddressRow, *BlockID) {
	d.RLock()
	defer d.RUnlock()
	return d.rows, d.blockID()
}

// RowsMerged is a thread-safe accessor for the []*dbtypes.AddressRow.
func (d *AddressCacheItem) RowsMerged() ([]*dbtypes.AddressRow, *BlockID) {
	d.RLock()
	defer d.RUnlock()
	return d.rowsMerged, d.blockID()
}

// Transactions attempts to retrieve transaction data for the given view (merged
// or not, debit/credit/all). Like the DB queries, the number of transactions to
// retrieve, N, and the number of transactions to skip, offset, are also
// specified.
func (d *AddressCacheItem) Transactions(N, offset int, txnView dbtypes.AddrTxnViewType) ([]*dbtypes.AddressRow, *BlockID, error) {
	if N == 0 {
		return nil, d.blockID(), nil
	}
	if offset < 0 || N < 0 {
		return nil, nil, fmt.Errorf("invalid offset (%d) or N (%d)", offset, N)
	}

	if d == nil {
		return nil, nil, fmt.Errorf("uninitialized AddressCacheItem")
	}

	d.RLock()
	defer d.RUnlock()
	merged, err := txnView.IsMerged()
	if err != nil {
		return nil, nil, fmt.Errorf("invalid transaction view")
	}
	if merged && d.rowsMerged == nil {
		return nil, nil, nil // cache miss is not an error
	}
	if !merged && d.rows == nil {
		return nil, nil, nil // cache miss is not an error
	}

	endRange := func(l int) int {
		end := offset + N
		if end > l {
			end = l
		}
		return end
	}

	var rows []*dbtypes.AddressRow
	switch txnView {
	case dbtypes.AddrTxnAll:
		end := endRange(len(d.rows))
		if offset < end {
			rows = d.rows[offset:end]
		}
	case dbtypes.AddrTxnCredit:
		rows = creditAddressRows(d.rows, N, offset)
	case dbtypes.AddrTxnDebit:
		rows = debitAddressRows(d.rows, N, offset)
	case dbtypes.AddrMergedTxn:
		end := endRange(len(d.rowsMerged))
		if offset < end {
			rows = d.rowsMerged[offset:end]
		}
	case dbtypes.AddrMergedTxnCredit:
		rows = creditAddressRows(d.rowsMerged, N, offset)
	case dbtypes.AddrMergedTxnDebit:
		rows = debitAddressRows(d.rowsMerged, N, offset)
	default:
		return nil, nil, fmt.Errorf("unrecognized address transaction view: %v", txnView)
	}

	return rows, d.blockID(), nil
}

// setBlock ensures that the AddressCacheItem pertains to the given BlockID,
// clearing any cached data if the previously set block is not equal to the
// given block.
func (d *AddressCacheItem) setBlock(block BlockID) {
	if block.Hash == d.hash {
		return
	}
	d.hash = block.Hash
	d.height = block.Height
	d.utxos = nil
	d.metrics = nil
	d.balance = nil
	d.rows = nil
	d.rowsMerged = nil
}

// SetRows updates the cache item for the given non-merged AddressRow slice
// valid at the given BlockID.
func (d *AddressCacheItem) SetRows(block BlockID, rows []*dbtypes.AddressRow) {
	d.Lock()
	defer d.Unlock()
	d.setBlock(block)
	d.rows = rows
}

// SetRowsMerged updates the cache item for the given merged AddressRow slice
// valid at the given BlockID.
func (d *AddressCacheItem) SetRowsMerged(block BlockID, rows []*dbtypes.AddressRow) {
	d.Lock()
	defer d.Unlock()
	d.setBlock(block)
	d.rowsMerged = rows
}

// SetUTXOs updates the cache item for the given AddressTxnOutput slice valid at
// the given BlockID.
func (d *AddressCacheItem) SetUTXOs(block BlockID, utxos []apitypes.AddressTxnOutput) {
	d.Lock()
	defer d.Unlock()
	d.setBlock(block)
	d.utxos = utxos
}

// SetBalance updates the cache item for the given AddressBalance valid at the
// given BlockID.
func (d *AddressCacheItem) SetBalance(block BlockID, balance *dbtypes.AddressBalance) {
	d.Lock()
	defer d.Unlock()
	d.setBlock(block)
	d.balance = balance
}

// AddressCache maintains a store of address data. Use NewAddressCache to create
// a new AddressCache with initialized internal data structures.
type AddressCache struct {
	sync.RWMutex
	a          map[string]*AddressCacheItem
	cap        int
	DevAddress string
}

// NewAddressCache constructs a AddressCache.
func NewAddressCache(cap int) *AddressCache {
	if cap < 2 {
		cap = 2
	}
	return &AddressCache{
		a:   make(map[string]*AddressCacheItem, cap),
		cap: cap,
	}
}

// addressCacheItem safely accesses any AddressCacheItem for the given address.
func (ac *AddressCache) addressCacheItem(addr string) *AddressCacheItem {
	ac.RLock()
	defer ac.RUnlock()
	return ac.a[addr]
}

// ClearAll resets AddressCache, purging all cached data.
func (ac *AddressCache) ClearAll() {
	ac.Lock()
	defer ac.Unlock()
	ac.a = make(map[string]*AddressCacheItem, ac.cap)
}

// Balance attempts to retrieve an AddressBalance for the given address. The
// BlockID for the block at which the cached data is valid is also returned. In
// the event of a cache miss, both returned pointers will be nil.
func (ac *AddressCache) Balance(addr string) (*dbtypes.AddressBalance, *BlockID) {
	aci := ac.addressCacheItem(addr)
	if aci == nil {
		return nil, nil
	}
	return aci.Balance()
}

// UTXOs attempts to retrieve an []AddressTxnOutput for the given address. The
// BlockID for the block at which the cached data is valid is also returned. In
// the event of a cache miss, the slice and the *BlockID will be nil.
func (ac *AddressCache) UTXOs(addr string) ([]apitypes.AddressTxnOutput, *BlockID) {
	aci := ac.addressCacheItem(addr)
	if aci == nil {
		return nil, nil
	}
	return aci.UTXOs()
}

// Metrics attempts to retrieve an AddressMetrics for the given address. The
// BlockID for the block at which the cached data is valid is also returned. In
// the event of a cache miss, both returned pointers will be nil.
func (ac *AddressCache) Metrics(addr string) (*dbtypes.AddressMetrics, *BlockID) {
	aci := ac.addressCacheItem(addr)
	if aci == nil {
		return nil, nil
	}
	return aci.Metrics()
}

// Rows attempts to retrieve an []*AddressRow for the given address. The BlockID
// for the block at which the cached data is valid is also returned. In the
// event of a cache miss, the slice and the *BlockID will be nil.
func (ac *AddressCache) Rows(addr string) ([]*dbtypes.AddressRow, *BlockID) {
	aci := ac.addressCacheItem(addr)
	if aci == nil {
		return nil, nil
	}
	return aci.Rows()
}

// RowsMerged attempts to retrieve an []*AddressRow for the given address. The
// BlockID for the block at which the cached data is valid is also returned. In
// the event of a cache miss, the slice and the *BlockID will be nil.
func (ac *AddressCache) RowsMerged(addr string) ([]*dbtypes.AddressRow, *BlockID) {
	aci := ac.addressCacheItem(addr)
	if aci == nil {
		return nil, nil
	}
	return aci.RowsMerged()
}

// Transactions attempts to retrieve transaction data for the given address and
// view (merged or not, debit/credit/all). Like the DB queries, the number of
// transactions to retrieve, N, and the number of transactions to skip, offset,
// are also specified.
func (ac *AddressCache) Transactions(addr string, N, offset int64, txnType dbtypes.AddrTxnViewType) ([]*dbtypes.AddressRow, *BlockID, error) {
	aci := ac.addressCacheItem(addr)
	if aci == nil {
		return nil, nil, nil /*fmt.Errorf("uninitialized address cache")*/
	}
	return aci.Transactions(int(N), int(offset), txnType)
}

func (ac *AddressCache) addCacheItem(addr string, aci *AddressCacheItem) {
	for len(ac.a) >= ac.cap {
		for a := range ac.a {
			if a == ac.DevAddress {
				continue
			}
			delete(ac.a, a)
		}
	}
	ac.a[addr] = aci
}

// StoreRows stores the non-merged AddressRow slice for the given address in
// cache. The current best block data is required to determine cache freshness.
func (ac *AddressCache) StoreRows(addr string, rows []*dbtypes.AddressRow, block *BlockID) {
	ac.Lock()
	defer ac.Unlock()
	aci := ac.a[addr]

	if aci == nil || aci.BlockHash() != block.Hash {
		ac.addCacheItem(addr, &AddressCacheItem{
			rows:   rows,
			height: block.Height,
			hash:   block.Hash,
		})
		return
	}

	// cache is current, so just set the rows.
	aci.Lock()
	aci.rows = rows
	aci.Unlock()
}

// StoreRowsMerged stores the merged AddressRow slice for the given address in
// cache. The current best block data is required to determine cache freshness.
func (ac *AddressCache) StoreRowsMerged(addr string, rows []*dbtypes.AddressRow, block *BlockID) {
	ac.Lock()
	defer ac.Unlock()
	aci := ac.a[addr]

	if aci == nil || aci.BlockHash() != block.Hash {
		ac.addCacheItem(addr, &AddressCacheItem{
			rowsMerged: rows,
			height:     block.Height,
			hash:       block.Hash,
		})
		return
	}

	// cache is current, so just set the rows.
	aci.Lock()
	aci.rowsMerged = rows
	aci.Unlock()
}

// StoreBalance stores the AddressBalance for the given address in cache. The
// current best block data is required to determine cache freshness.
func (ac *AddressCache) StoreBalance(addr string, balance *dbtypes.AddressBalance, block *BlockID) {
	ac.Lock()
	defer ac.Unlock()
	aci := ac.a[addr]

	if aci == nil || aci.BlockHash() != block.Hash {
		ac.addCacheItem(addr, &AddressCacheItem{
			balance: balance,
			height:  block.Height,
			hash:    block.Hash,
		})
		return
	}

	// cache is current, so just set the balance.
	aci.Lock()
	aci.balance = balance
	aci.Unlock()
}

// StoreUTXOs stores the AddressTxnOutput slice for the given address in cache.
// The current best block data is required to determine cache freshness.
func (ac *AddressCache) StoreUTXOs(addr string, utxos []apitypes.AddressTxnOutput, block *BlockID) {
	ac.Lock()
	defer ac.Unlock()
	aci := ac.a[addr]

	if aci == nil || aci.BlockHash() != block.Hash {
		ac.addCacheItem(addr, &AddressCacheItem{
			utxos:  utxos,
			height: block.Height,
			hash:   block.Hash,
		})
		return
	}

	// cache is current, so just set the utxos.
	aci.Lock()
	aci.utxos = utxos
	aci.Unlock()
}
