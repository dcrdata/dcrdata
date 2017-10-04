// Copyright (c) 2017, The Dcrdata developers
// See LICENSE for details.

package explorer

import (
	"github.com/dcrdata/dcrdata/txhelpers"
	"github.com/decred/dcrutil"
)

// BlockBasic models data for the explorer's explorer page
type BlockBasic struct {
	Height          int64
	Size            int32
	Voters          uint16
	Transactions    int
	FreshStake      uint8
	Revocations     uint8
	BlockTime       int64
	FormattedTime   string
	FormattedBytes  string
	FormattedHeight string
}

// TxBasic models data for transactions on the block page
type TxBasic struct {
	TxID           string
	FormattedSize  string
	FormattedTotal string
	Total          float64
	Fee            dcrutil.Amount
	FeeRate        dcrutil.Amount
	VoteInfo       *VoteInfo
}

//AddressTx models data for transactions on the address page
type AddressTx struct {
	TxID           string
	FormattedSize  string
	FormattedTotal string
	Confirmations  uint64
	Time           int64
	FormattedTime  string
}

// TxInfo models data needed for display on the tx page
type TxInfo struct {
	*TxBasic
	Type          string
	Vin           []Vin
	Vout          []Vout
	BlockHeight   int64
	BlockIndex    uint32
	Confirmations int64
	Time          int64
	FormattedTime string
}

// VoteInfo models data about a SSGen transaction (vote)
type VoteInfo struct {
	Version uint32                  `json:"vote_version"`
	Bits    uint16                  `json:"vote_bits"`
	Choices []*txhelpers.VoteChoice `json:"vote_choices"`
}

// Vin models basic data about a tx input for display
type Vin struct {
	TxID            string
	CoinBase        string
	Addresses       []string
	Vout            uint32
	Amount          float64
	BlockHeight     uint32
	FormattedAmount string
}

// Vout models basic data about a tx ouput for display
type Vout struct {
	Addresses       []string
	Amount          float64
	FormattedAmount string
	Type            string
}

// BlockInfo models data for display on the block page
type BlockInfo struct {
	*BlockBasic
	Hash          string
	Version       int32
	Confirmations int64
	StakeRoot     string
	MerkleRoot    string
	Tx            []*TxBasic
	Tickets       []*TxBasic
	Revs          []*TxBasic
	Votes         []*TxBasic
	Nonce         uint32
	VoteBits      uint16
	FinalState    string
	PoolSize      uint32
	Bits          string
	SBits         float64
	Difficulty    float64
	ExtraData     string
	StakeVersion  uint32
	PreviousHash  string
	NextHash      string
	TotalSent     float64
	TotalSpent    float64
	MiningFee     float64
}

// AddressInfo models data for display on the address page
type AddressInfo struct {
	Address      string
	Transactions []*AddressTx
}
