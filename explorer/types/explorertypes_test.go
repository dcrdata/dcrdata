package types

import (
	"reflect"
	"sync"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestDeepCopys(t *testing.T) {
	tickets := []MempoolTx{
		MempoolTx{
			TxID:      "96e10d7ce108b1a357168b0a923d86d2744ba9777a2d81cbff71ffb982381c95",
			Fees:      0.0001,
			VinCount:  2,
			VoutCount: 5,
			Vin: []MempoolInput{
				MempoolInput{
					TxId:   "43f26841e744ce2f901e21400f275eda27ba2a3fa962110d52e7dd37f2193c78",
					Index:  0,
					Outdex: 0,
				},
				MempoolInput{
					TxId:   "43f26841e744ce2f901e21400f275eda27ba2a3fa962110d52e7dd37f2193c78",
					Index:  1,
					Outdex: 1,
				},
			},
			Hash:     "96e10d7ce108b1a357168b0a923d86d2744ba9777a2d81cbff71ffb982381c95",
			Size:     539,
			TotalOut: 106.39717461,
			Type:     "Ticket",
		},
		MempoolTx{
			TxID:      "8eb2f6c8f3a9cdc8d6de2ef3bfca9efcffed4484dd4fde2d01dc0fc0e415c75a",
			Fees:      0.0001,
			VinCount:  2,
			VoutCount: 5,
			Vin: []MempoolInput{
				MempoolInput{
					TxId:   "4e9221f790916b4d891b40ef82b8a6dc89f5c0719d5d5ddcf46ac3673d8446aa",
					Index:  0,
					Outdex: 0,
				},
				MempoolInput{
					TxId:   "4e9221f790916b4d891b40ef82b8a6dc89f5c0719d5d5ddcf46ac3673d8446aa",
					Index:  1,
					Outdex: 1,
				},
			},
			Hash:     "8eb2f6c8f3a9cdc8d6de2ef3bfca9efcffed4484dd4fde2d01dc0fc0e415c75a",
			Size:     538,
			TotalOut: 106.39717461,
			Type:     "Ticket",
		},
	}

	votes := []MempoolTx{
		MempoolTx{
			TxID:      "64ce0422cb6ba1aefa63c8df1d872250d181261ff3acd5a71bc1f521096207c9",
			Fees:      0,
			VinCount:  2,
			VoutCount: 3,
			Vin: []MempoolInput{
				MempoolInput{
					TxId:   "",
					Index:  0,
					Outdex: 0,
				},
				MempoolInput{
					TxId:   "7884ecd8fb5934e77708f82f0aa052ad86cccc5749602be14d93745d8272538e",
					Index:  1,
					Outdex: 0,
				},
			},
			Hash:     "64ce0422cb6ba1aefa63c8df1d872250d181261ff3acd5a71bc1f521096207c9",
			Size:     344,
			TotalOut: 102.86278351,
			Type:     "Vote",
		},
		MempoolTx{
			TxID:      "07aa38f10fe1a849a52b9d4812081854e4ac7268751a0ea661e8f499d7de91f1",
			Fees:      0,
			VinCount:  2,
			VoutCount: 3,
			Vin: []MempoolInput{
				MempoolInput{
					TxId:   "",
					Index:  0,
					Outdex: 0,
				},
				MempoolInput{
					TxId:   "99045541c481e7e694598a2d77967f5e4e053cee3265d77c5b49f1eb8b282176",
					Index:  1,
					Outdex: 0,
				},
			},
			Hash:     "07aa38f10fe1a849a52b9d4812081854e4ac7268751a0ea661e8f499d7de91f1",
			Size:     345,
			TotalOut: 105.29923146,
			Type:     "Vote",
		},
		MempoolTx{
			TxID:      "1df658e1b0de08112adcfb9b8b17dcc2b64f756b1e21f6b1f715fd2b86439955",
			Fees:      0,
			VinCount:  2,
			VoutCount: 3,
			Vin: []MempoolInput{
				MempoolInput{
					TxId:   "",
					Index:  0,
					Outdex: 0,
				},
				MempoolInput{
					TxId:   "28cc0b43bf79908115323f16dbd17d0e44a5366ca5d49e2d4f5a9c5f741e5699",
					Index:  1,
					Outdex: 0,
				},
			},
			Hash:     "1df658e1b0de08112adcfb9b8b17dcc2b64f756b1e21f6b1f715fd2b86439955",
			Size:     345,
			TotalOut: 111.28226529,
			Type:     "Vote",
		},
	}

	regular := []MempoolTx{
		MempoolTx{
			TxID:      "0572b2d121322d3a9b20fe5d5024c73d8bb817398948a167ddb668e52bbb21f6",
			Fees:      0.000585,
			VinCount:  3,
			VoutCount: 2,
			Vin: []MempoolInput{
				MempoolInput{
					TxId:   "4a9aaca49784586d3abc0dbd5d7d3dcdf70940c60bc5cbaa39379690d9ac5c6d",
					Index:  0,
					Outdex: 9,
				},
				MempoolInput{
					TxId:   "f621d45fb440307f151c1470619e37209aca7f8c12379e82f5c2ebcf882fb884",
					Index:  1,
					Outdex: 1,
				},
				MempoolInput{
					TxId:   "08b01afd1c252fbef8bbad933c1d7e3da1d3e3011ef3d4cdd532f5803ea173b9",
					Index:  2,
					Outdex: 0,
				},
			},
			Hash:     "0572b2d121322d3a9b20fe5d5024c73d8bb817398948a167ddb668e52bbb21f6",
			Size:     581,
			TotalOut: 139.11389736,
			Type:     "Regular",
		},
		MempoolTx{
			TxID:      "9e11deaae5ecd1d3288468a491f820b66adfb74be70eba582c0b13a25e76bb3b",
			Fees:      0.000585,
			VinCount:  3,
			VoutCount: 2,
			Vin: []MempoolInput{
				MempoolInput{
					TxId:   "bf9d371a9f3fd510ec5d6b485c0fd64ca1b6dac9c3b915973ba8fc86fc788e8c",
					Index:  0,
					Outdex: 1,
				},
				MempoolInput{
					TxId:   "a245d62d3916869f930afd80dce6f47c7291145c36fccae7ba73c0e462ff4cd5",
					Index:  1,
					Outdex: 1,
				},
				MempoolInput{
					TxId:   "b8274d92cac36a08cc28600fec66a09e9d429486506da1c8616c93544ce0f2ee",
					Index:  2,
					Outdex: 2,
				},
			},
			Hash:     "9e11deaae5ecd1d3288468a491f820b66adfb74be70eba582c0b13a25e76bb3b",
			Size:     580,
			TotalOut: 204.94920773,
			Type:     "Regular",
		},
	}

	allTxns := regular
	allTxns = append(allTxns, votes...)
	allTxns = append(allTxns, tickets...)

	allCopy := CopyMempoolTxSlice(allTxns)
	if !reflect.DeepEqual(allTxns, allCopy) {
		t.Errorf("MempoolTx slices not equal: %v\n\n%v\n", allTxns, allCopy)
	}

	latest := regular
	latest = append(latest, tickets...)
	latest = append(latest, votes[0])

	invRegular := make(map[string]struct{}, len(regular))
	for i := range regular {
		invRegular[regular[i].TxID] = struct{}{}
	}

	invStake := make(map[string]struct{}, len(votes)+len(tickets))
	for i := range votes {
		invStake[votes[i].TxID] = struct{}{}
	}
	for i := range tickets {
		invStake[tickets[i].TxID] = struct{}{}
	}

	mps := &MempoolShort{
		LastBlockHash:      "000000000000000043a1e65fe3309ab5b2a0f4fb3e46036bbee2be6294790c98",
		LastBlockHeight:    310278,
		LastBlockTime:      1547677417,
		Time:               1547677417 + 1e5,
		TotalOut:           1292.76211530,
		TotalSize:          2479,
		NumTickets:         2,
		NumVotes:           3,
		NumRegular:         2,
		NumRevokes:         0,
		NumAll:             7,
		LatestTransactions: latest,
		FormattedTotalSize: "134134 B",
		TicketIndexes: BlockValidatorIndex{
			"00000000000000003b6e0e24c75575911edabbbae181fc6e8e686c6aadcc2ce2": TicketIndex{
				"28cc0b43bf79908115323f16dbd17d0e44a5366ca5d49e2d4f5a9c5f741e5699": 0,
				"0969ba6af7da6b63b122e0c9d57e743397ca0bc0ad39ca1927422db0e70ec19b": 1,
				"99045541c481e7e694598a2d77967f5e4e053cee3265d77c5b49f1eb8b282176": 2,
				"a5f35e94af7945f06adba39598faa4c65d9063fc1c4e7d502eb349c924364a10": 3,
				"7884ecd8fb5934e77708f82f0aa052ad86cccc5749602be14d93745d8272538e": 4,
			},
		},
		VotingInfo: VotingInfo{
			TicketsVoted:     3,
			MaxVotesPerBlock: 5,
			VotedTickets: map[string]bool{
				"7884ecd8fb5934e77708f82f0aa052ad86cccc5749602be14d93745d8272538e": true,
				"28cc0b43bf79908115323f16dbd17d0e44a5366ca5d49e2d4f5a9c5f741e5699": true,
				"99045541c481e7e694598a2d77967f5e4e053cee3265d77c5b49f1eb8b282176": false,
			},
		},
		InvRegular: invRegular,
		InvStake:   invStake,
	}

	mps2 := mps.DeepCopy()

	if !reflect.DeepEqual(*mps, *mps2) {
		t.Errorf("MempoolShort structs not equal: %v\n\n%v\n", *mps, *mps2)
	}

	mpi := &MempoolInfo{
		MempoolShort: *mps,
		Transactions: regular,
		Tickets:      tickets,
		Votes:        votes,
		Revocations:  nil,
	}

	mpi2 := mpi.DeepCopy()

	copts := cmpopts.IgnoreTypes(sync.RWMutex{})
	if !cmp.Equal(mpi, mpi2, copts) {
		t.Errorf("MempoolInfo structs not equal: \n%s\n", cmp.Diff(mpi, mpi2, copts))
	}
}
