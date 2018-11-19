// Copyright (c) 2018, The Decred developers
// Copyright (c) 2017, Jonathan Chappelow
// See LICENSE for details.

package stakedb

import (
	"context"
	"fmt"
	"sync"

	"github.com/decred/dcrd/chaincfg/chainhash"
	"github.com/decred/dcrd/dcrutil"
)

// ReorgData contains the information from a reoranization notification
type ReorgData struct {
	OldChainHead   chainhash.Hash
	OldChainHeight int32
	NewChainHead   chainhash.Hash
	NewChainHeight int32
	WG             *sync.WaitGroup
}

// ChainMonitor connects blocks to the stake DB as they come in.
type ChainMonitor struct {
	ctx            context.Context
	db             *StakeDatabase
	wg             *sync.WaitGroup
	blockChan      chan *chainhash.Hash
	reorgChan      chan *ReorgData
	syncConnect    sync.Mutex
	ConnectingLock chan struct{}
	DoneConnecting chan struct{}

	// reorg handling
	sync.Mutex
	reorgData    *ReorgData
	sideChain    []chainhash.Hash
	reorganizing bool
}

// NewChainMonitor creates a new ChainMonitor
func (db *StakeDatabase) NewChainMonitor(ctx context.Context, wg *sync.WaitGroup,
	blockChan chan *chainhash.Hash, reorgChan chan *ReorgData) *ChainMonitor {
	return &ChainMonitor{
		ctx:            ctx,
		db:             db,
		wg:             wg,
		blockChan:      blockChan,
		reorgChan:      reorgChan,
		ConnectingLock: make(chan struct{}, 1),
		DoneConnecting: make(chan struct{}),
	}
}

// BlockConnectedSync is the synchronous (blocking call) handler for the newly
// connected block given by the hash.
func (p *ChainMonitor) BlockConnectedSync(hash *chainhash.Hash) {
	// Connections go one at a time so signals cannot be mixed
	p.syncConnect.Lock()
	defer p.syncConnect.Unlock()
	// lock with buffered channel, accepting handoff in BlockConnectedHandler
	p.ConnectingLock <- struct{}{}
	p.blockChan <- hash
	// wait
	<-p.DoneConnecting
}

// BlockConnectedHandler handles block connected notifications, which trigger
// data collection and storage.
func (p *ChainMonitor) BlockConnectedHandler() {
	defer p.wg.Done()
out:
	for {
	keepon:
		select {
		case hash, ok := <-p.blockChan:
			p.Lock()
			release := func() { p.Unlock() }
			select {
			case <-p.ConnectingLock:
				// send on unbuffered channel
				release = func() { p.Unlock(); p.DoneConnecting <- struct{}{} }
			default:
			}

			if !ok {
				log.Warnf("Block connected channel closed.")
				release()
				break out
			}

			// Extend main chain
			block, err := p.db.ConnectBlockHash(hash)
			if err != nil {
				release()
				log.Error(err)
				break keepon
			}

			log.Infof("Connected block %d to stake DB.", block.Height())

			release()

		case <-p.ctx.Done():
			log.Debugf("Got quit signal. Exiting block connected handler.")
			break out
		}
	}

}

// switchToSideChain attempts to switch to a new side chain by: determining a
// common ancestor block, disconnecting blocks from the main chain back to this
// block, and connecting the side chain blocks onto the mainchain.
func (p *ChainMonitor) switchToSideChain() (int32, *chainhash.Hash, error) {
	if len(p.sideChain) == 0 {
		return 0, nil, fmt.Errorf("no side chain")
	}

	// Determine highest common ancestor of side chain and main chain
	msgBlock, err := p.db.NodeClient.GetBlock(&p.sideChain[0])
	if err != nil {
		return 0, nil, fmt.Errorf("unable to get block at root of side chain")
	}
	block := dcrutil.NewBlock(msgBlock)

	prevMsgBlock, err := p.db.NodeClient.GetBlock(&msgBlock.Header.PrevBlock)
	if err != nil {
		return 0, nil, fmt.Errorf("unable to get common ancestor on side chain")
	}
	prevBlock := dcrutil.NewBlock(prevMsgBlock)

	commonAncestorHeight := block.Height() - 1
	if prevBlock.Height() != commonAncestorHeight {
		panic("Failed to determine common ancestor.")
	}

	mainTip := int64(p.db.Height())

	// Disconnect blocks back to common ancestor
	log.Debugf("Disconnecting %d blocks", mainTip-commonAncestorHeight)
	if err = p.db.DisconnectBlocks(mainTip - commonAncestorHeight); err != nil {
		return 0, nil, err
	}

	mainTip = int64(p.db.Height())
	if mainTip != commonAncestorHeight {
		panic(fmt.Sprintf("disconnect blocks failed: tip height %d, expected %d",
			mainTip, commonAncestorHeight))
	}

	// Connect blocks in side chain onto main chain
	log.Debugf("Connecting %d blocks", len(p.sideChain))
	for i := range p.sideChain {
		if block, err = p.db.ConnectBlockHash(&p.sideChain[i]); err != nil {
			mainTip = int64(p.db.Height())
			currentBlockHdr, _ := p.db.DBTipBlockHeader()
			currentBlockHash := currentBlockHdr.BlockHash()
			return int32(mainTip), &currentBlockHash,
				fmt.Errorf("error connecting block %v", p.sideChain[i])
		}
		log.Infof("Connected block %v (height %d) from side chain.",
			block.Hash(), block.Height())
	}

	p.sideChain = nil

	mainTip = int64(p.db.Height())
	if mainTip != block.Height() {
		panic("connected block height not db tip height")
	}

	return int32(mainTip), block.Hash(), nil
}

// ReorgHandler receives notification of a chain reorganization and initiates a
// corresponding reorganization of the stakedb.StakeDatabase.
func (p *ChainMonitor) ReorgHandler() {
	defer p.wg.Done()
out:
	for {
	keepon:
		select {
		case reorgData, ok := <-p.reorgChan:
			p.Lock()
			if !ok {
				p.Unlock()
				log.Warnf("Reorg channel closed.")
				break out
			}

			newHeight, oldHeight := reorgData.NewChainHeight, reorgData.OldChainHeight
			newHash, oldHash := reorgData.NewChainHead, reorgData.OldChainHead

			if p.reorganizing {
				log.Errorf("Reorg notified for chain tip %v (height %v), but already "+
					"processing a reorg to block %v", newHash, newHeight,
					p.reorgData.NewChainHead)
				p.Unlock()
				break keepon
			}

			p.reorganizing = true
			p.reorgData = reorgData

			log.Infof("Reorganize started. NEW head block %v at height %d.",
				newHash, newHeight)
			log.Infof("Reorganize started. OLD head block %v at height %d.",
				oldHash, oldHeight)

			// Get the side chain.
			p.sideChain = []chainhash.Hash{}
			hash := newHash

			for {
				prevMsgBlock, err := p.db.NodeClient.GetBlock(&hash)
				if err != nil {
					log.Errorf("Unable to get block for reorg: %v", hash)
					p.Unlock()
					p.reorganizing = false
					break keepon
				}
				// prevBlock := dcrutil.NewBlock(prevMsgBlock)

				hash = prevMsgBlock.BlockHash()
			}

			// Success!
			p.reorganizing = false
			p.Unlock()

			reorgData.WG.Done()

		case <-p.ctx.Done():
			log.Debugf("Got quit signal. Exiting reorg notification handler.")
			break out
		}
	}
}
