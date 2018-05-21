// Copyright (c) 2018, The Decred developers
// Copyright (c) 2017, The dcrdata developers
// See LICENSE for details.

package insight

import (
	"fmt"
	"strconv"

	"github.com/decred/dcrd/chaincfg"
	"github.com/decred/dcrd/chaincfg/chainhash"
	"github.com/decred/dcrd/dcrjson"
	"github.com/decred/dcrd/dcrutil"
	apitypes "github.com/decred/dcrdata/api/types"
	"github.com/decred/dcrdata/db/dbtypes"
)

// TxConverter converts dcrd-tx to insight tx
func (c *insightApiContext) TxConverter(txs []*dcrjson.TxRawResult) ([]apitypes.InsightTx, error) {

	newTxs := []apitypes.InsightTx{}

	for _, tx := range txs {

		vInSum := float64(0)
		vOutSum := float64(0)

		// Build new model. Based on the old api responses of
		txNew := apitypes.InsightTx{}
		txNew.Txid = tx.Txid
		txNew.Version = tx.Version
		txNew.Locktime = tx.LockTime
		//txNew.Expiry = tx.Expiry

		// Vins fill
		for vinID, vin := range tx.Vin {
			vinEmpty := &apitypes.InsightVin{}
			emptySS := &apitypes.InsightScriptSig{}
			txNew.Vins = append(txNew.Vins, vinEmpty)
			txNew.Vins[vinID].Txid = vin.Txid
			txNew.Vins[vinID].Vout = vin.Vout
			//txNew.Vins[vinID].Tree = vin.Tree
			txNew.Vins[vinID].Sequence = vin.Sequence
			//txNew.Vins[vinID].Amountin = vin.AmountIn
			vInSum += vin.AmountIn
			// txNew.Vins[vinID].Blockheight = vin.BlockHeight
			// txNew.Vins[vinID].Blockindex = vin.BlockIndex
			txNew.Vins[vinID].CoinBase = vin.Coinbase
			//txNew.Vins[vinID].Stakebase = vin.Stakebase
			// init ScriptPubKey
			txNew.Vins[vinID].ScriptSig = emptySS
			if vin.ScriptSig != nil {
				txNew.Vins[vinID].ScriptSig.Asm = vin.ScriptSig.Asm
				txNew.Vins[vinID].ScriptSig.Hex = vin.ScriptSig.Hex
			}
			txNew.Vins[vinID].N = vinID
			txNew.Vins[vinID].ValueSat = int64(vin.AmountIn * 100000000.0)
			txNew.Vins[vinID].Value = vin.AmountIn
		}

		// Vout fill
		for _, v := range tx.Vout {
			voutEmpty := &apitypes.InsightVout{}
			emptyPubKey := apitypes.InsightScriptPubKey{}
			txNew.Vouts = append(txNew.Vouts, voutEmpty)
			txNew.Vouts[v.N].Value = v.Value
			vOutSum += v.Value
			txNew.Vouts[v.N].N = v.N
			//txNew.Vouts[v.N].Version = v.Version
			// pk block
			txNew.Vouts[v.N].ScriptPubKey = emptyPubKey
			txNew.Vouts[v.N].ScriptPubKey.Asm = v.ScriptPubKey.Asm
			txNew.Vouts[v.N].ScriptPubKey.Hex = v.ScriptPubKey.Hex
			//txNew.Vouts[v.N].ScriptPubKey.ReqSigs = v.ScriptPubKey.ReqSigs
			txNew.Vouts[v.N].ScriptPubKey.Type = v.ScriptPubKey.Type
			txNew.Vouts[v.N].ScriptPubKey.Addresses = v.ScriptPubKey.Addresses
		}

		txNew.Blockhash = tx.BlockHash
		txNew.Blockheight = tx.BlockHeight
		txNew.Confirmations = tx.Confirmations
		txNew.Time = tx.Time
		txNew.Blocktime = tx.Blocktime

		txNew.ValueOut = vOutSum // vout value sum plus fees
		txNew.ValueIn = vInSum

		// Return true if coinbase value is not empty, return 0 at some fields
		if txNew.Vins != nil && len(txNew.Vins[0].CoinBase) > 0 {
			txNew.IsCoinBase = true
			txNew.ValueIn = 0
			for _, v := range txNew.Vins {
				v.Value = 0
				v.ValueSat = 0
				//v.UnconfirmedInput = 0
			}
		}

		// This block set addr value in tx vin
		for _, vin := range txNew.Vins {
			if vin.Txid != "" {
				vin.UnconfirmedInput = false
				vin.IsConfirmed = true
				vinsTx, err := c.BlockData.GetRawTransaction(vin.Txid)
				if err != nil {
					apiLog.Errorf("Tried to get transaction by vin tx %s", vin.Txid)
					return newTxs, err
				}
				vin.Confirmations = vinsTx.Confirmations
				for _, vinVout := range vinsTx.Vout {
					if vinVout.Value == vin.Value {
						if vinVout.ScriptPubKey.Addresses != nil {
							if vinVout.ScriptPubKey.Addresses[0] != "" {
								vin.Addr = vinVout.ScriptPubKey.Addresses[0]
							}
						}
					}
				}
			} else {
				vin.Confirmations = 0
				vin.UnconfirmedInput = true
				vin.IsConfirmed = false
				//txNew.IncompleteInputs++ // add 1 to incomplete inputs
			}
		}

		// set of unique addresses for db query
		uniqAddrs := make(map[string]string)

		for _, vout := range txNew.Vouts {
			for _, addr := range vout.ScriptPubKey.Addresses {
				uniqAddrs[addr] = txNew.Txid
			}
		}

		addresses := []string{}
		for addr := range uniqAddrs {
			addresses = append(addresses, addr)
		}

		addrFull := c.BlockData.ChainDB.GetAddressSpendByFunHash(addresses, txNew.Txid)
		for _, dbaddr := range addrFull {
			txNew.Vouts[dbaddr.FundingTxVoutIndex].SpentIndex = dbaddr.SpendingTxVinIndex
			txNew.Vouts[dbaddr.FundingTxVoutIndex].SpentTxID = dbaddr.SpendingTxHash
			txNew.Vouts[dbaddr.FundingTxVoutIndex].SpentHeight = dbaddr.BlockHeight
		}
		// create block hash
		bHash, err := chainhash.NewHashFromStr(txNew.Blockhash)
		if err != nil {
			apiLog.Errorf("Failed to gen block hash for Tx %s", txNew.Txid)
			return newTxs, err
		}

		// get block
		block, err := c.BlockData.Client.GetBlock(bHash)
		if err != nil {
			apiLog.Errorf("Unable to get block %s", bHash)
			return newTxs, err
		}

		// stakeTree 0: Tx, 1: stakeTx
		dbTransactions, _, _ := dbtypes.ExtractBlockTransactions(block, 0, &chaincfg.MainNetParams)

		sdbTransactions, _, _ := dbtypes.ExtractBlockTransactions(block, 1, &chaincfg.MainNetParams)

		// its cumbersome but easier than differentiate tx and stx at that point
		dbTransactions = append(dbTransactions, sdbTransactions...)

		for _, dbtx := range dbTransactions {
			if dbtx.TxID == txNew.Txid {
				txNew.Size = dbtx.Size
				txNew.Fees = dcrutil.Amount(dbtx.Fees).ToCoin()
				//if txNew.IsStakeGen || txNew.IsCoinBase {
				if txNew.IsCoinBase {
					txNew.Fees = 0
				}

				txNew.ValueOut, _ = strconv.ParseFloat(fmt.Sprintf("%.8f", txNew.ValueOut), 64)

				break
			}
		}

		newTxs = append(newTxs, txNew)
	}

	return newTxs, nil
}
