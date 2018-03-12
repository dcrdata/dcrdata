// Copyright (c) 2017, The dcrdata developers
// See LICENSE for details.

package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/decred/dcrd/chaincfg/chainhash"
	"github.com/decred/dcrd/dcrjson"
	apitypes "github.com/decred/dcrdata/api/types"
	"github.com/go-chi/chi"
	"github.com/go-chi/docgen"
)

type contextKey int

const (
	ctxAPIDocs contextKey = iota
	ctxAPIStatus
	ctxAddress
	ctxBlockIndex0
	ctxBlockIndex
	ctxBlockStep
	ctxBlockHash
	ctxTxHash
	ctxTxInOutIndex
	ctxSearch
	ctxN
	ctxCount
	ctxOffset
	ctxBlockDate
	ctxLimit
	ctxGetStatus
	ctxStakeVersionLatest
	ctxRawHexTx
)

type DataSource interface {
	GetHeight() int
	GetBlockHeight(hash string) (int64, error)
	GetBlockHash(idx int64) (string, error)
}

// GetBlockHeight accepts a block hash
// returns block height
type GetStakeVersionsLatest func() (*dcrjson.StakeVersions, error)
type Status apitypes.Status

// GetBlockStepCtx accepts http request
// returns the blockstep index
func GetBlockStepCtx(r *http.Request) int {
	step, ok := r.Context().Value(ctxBlockStep).(int)
	if !ok {
		apiLog.Error("block step not set")
		return -1
	}
	return step
}

// GetBlockIndex0Ctx accepts http request returns
// the block index0
func GetBlockIndex0Ctx(r *http.Request) int {
	idx, ok := r.Context().Value(ctxBlockIndex0).(int)
	if !ok {
		apiLog.Error("block index0 not set")
		return -1
	}
	return idx
}

// GetTxIOIndexCtx accepts http request
// returns transaction in and out index
func GetTxIOIndexCtx(r *http.Request) int {
	index, ok := r.Context().Value(ctxTxInOutIndex).(int)
	if !ok {
		apiLog.Trace("txinoutindex not set")
		return -1
	}
	return index
}

func GetNCtx(r *http.Request) int {
	N, ok := r.Context().Value(ctxN).(int)
	if !ok {
		apiLog.Trace("N not set")
		return -1
	}
	return N
}

// GetRawHexTx accepts http request
// returns the transaction hex
func GetRawHexTx(r *http.Request) string {
	rawHexTx, ok := r.Context().Value(ctxRawHexTx).(string)
	if !ok {
		apiLog.Trace("hex transaction id not set")
		return ""
	}
	return rawHexTx
}

// GetTxIDCtx accepts http request
// returns transaction hash
func GetTxIDCtx(r *http.Request) string {
	hash, ok := r.Context().Value(ctxTxHash).(string)
	if !ok {
		apiLog.Trace("txid not set")
		return ""
	}
	return hash
}

// GetBlockHashCtx accepts http request
// returns block hash
func GetBlockHashCtx(r *http.Request) string {
	hash, ok := r.Context().Value(ctxBlockHash).(string)
	if !ok {
		apiLog.Trace("block hash not set")
	}
	return hash
}

// GetAddressCtx accepts http request
// returns address
func GetAddressCtx(r *http.Request) string {
	address, ok := r.Context().Value(ctxAddress).(string)
	if !ok {
		apiLog.Trace("address not set")
		return ""
	}
	return address
}

// GetCountCtx accepts http request
// returns count embedded in http request
func GetCountCtx(r *http.Request) int {
	count, ok := r.Context().Value(ctxCount).(int)
	if !ok {
		apiLog.Trace("count not set")
		return 20
	}
	return count
}

// GetOffsetCtx accepts http request
// returns offset embedded in http request
func GetOffsetCtx(r *http.Request) int {
	offset, ok := r.Context().Value(ctxOffset).(int)
	if !ok {
		apiLog.Trace("offset not set")
		return 0
	}
	return offset
}

// GetStatusInfoCtx accepts http request
// returns statusinfo embedded in http request
func GetStatusInfoCtx(r *http.Request) string {
	statusInfo, ok := r.Context().Value(ctxGetStatus).(string)
	if !ok {
		apiLog.Error("status info no set")
		return ""
	}
	return statusInfo
}

// GetLimitCtx accepts http request
// returns limit embedded in http request
func GetLimitCtx(r *http.Request) int {
	limit, ok := r.Context().Value(ctxLimit).(string)
	fmt.Println("limit ", limit)
	if !ok {
		fmt.Println(ok)
		apiLog.Trace("limit not set")
		return 1
	}
	intValue, err := strconv.Atoi(limit)
	if err != nil {
		return 1
	}
	return intValue
}

// GetBlockDateCtx accepts http request
// returns blockDate embedded in http request
func GetBlockDateCtx(r *http.Request) string {
	blockDate, _ := r.Context().Value(ctxBlockDate).(string)
	return blockDate
}

// GetBlockHashOnlyCtx accepts http request
// returns block hash embedded in http request
func GetBlockHashOnlyCtx(r *http.Request) string {
	hash, ok := r.Context().Value(ctxBlockHash).(string)
	if !ok {
		apiLog.Trace("block hash not set")
		return ""
	}
	return hash
}

// GetBlockIndexCtx accepts http request
// returns block index embedded in http request
func GetBlockIndexCtx(r *http.Request) int {
	idx, ok := r.Context().Value(ctxBlockIndex).(int)
	if !ok {
		apiLog.Trace("block index not set")
		return -1
	}
	return idx
}

// CacheControl creates a new middleware to set the HTTP response header with
// "Cache-Control: max-age=maxAge" where maxAge is in seconds.
func CacheControl(maxAge int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "max-age="+strconv.FormatInt(maxAge, 10))
			next.ServeHTTP(w, r)
		})
	}
}

// BlockStepPathCtx returns a http.HandlerFunc that embeds the value at the url
// part {step} into the request context
func BlockStepPathCtx(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		stepIdxStr := chi.URLParam(r, "step")
		step, err := strconv.Atoi(stepIdxStr)
		if err != nil {
			apiLog.Infof("No/invalid step value (int64): %v", err)
			http.NotFound(w, r)
			return
		}
		ctx := context.WithValue(r.Context(), ctxBlockStep, step)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// BlockIndexPathCtx returns a http.HandlerFunc that embeds the value at the url
// part {idx} into the request context
func BlockIndexPathCtx(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pathIdxStr := chi.URLParam(r, "idx")
		idx, err := strconv.Atoi(pathIdxStr)
		if err != nil {
			apiLog.Infof("No/invalid idx value (int64): %v", err)
			http.NotFound(w, r)
			return
		}
		ctx := context.WithValue(r.Context(), ctxBlockIndex, idx)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// BlockIndexOrHashPathCtx returns a http.HandlerFunc that embeds the value at the url
// part {idxorhash} into the request context
func BlockIndexOrHashPathCtx(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var ctx context.Context
		pathIdxOrHashStr := chi.URLParam(r, "idxorhash")
		if len(pathIdxOrHashStr) == 2*chainhash.HashSize {
			ctx = context.WithValue(r.Context(), ctxBlockHash, pathIdxOrHashStr)
		} else {
			idx, err := strconv.Atoi(pathIdxOrHashStr)
			if err != nil {
				apiLog.Infof("No/invalid idx value (int64): %v", err)
				http.NotFound(w, r)
				return
			}
			ctx = context.WithValue(r.Context(), ctxBlockIndex, idx)
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// BlockIndex0PathCtx returns a http.HandlerFunc that embeds the value at the url
// part {idx0} into the request context
func BlockIndex0PathCtx(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pathIdxStr := chi.URLParam(r, "idx0")
		idx, err := strconv.Atoi(pathIdxStr)
		if err != nil {
			apiLog.Infof("No/invalid idx0 value (int64): %v", err)
			http.NotFound(w, r)
			return
		}
		ctx := context.WithValue(r.Context(), ctxBlockIndex0, idx)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// NPathCtx returns a http.HandlerFunc that embeds the value at the url
// part {N} into the request context
func NPathCtx(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pathNStr := chi.URLParam(r, "N")
		N, err := strconv.Atoi(pathNStr)
		if err != nil {
			apiLog.Infof("No/invalid numeric value (uint64): %v", err)
			http.NotFound(w, r)
			return
		}
		ctx := context.WithValue(r.Context(), ctxN, N)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// BlockHashPathCtx returns a http.HandlerFunc that embeds the value at the url
// part {blockhash} into the request context
func BlockHashPathCtx(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hash := chi.URLParam(r, "blockhash")
		ctx := context.WithValue(r.Context(), ctxBlockHash, hash)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// TransactionHashCtx returns a http.HandlerFunc that embeds the value at the url
// part {txid} into the request context
func TransactionHashCtx(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		txid := chi.URLParam(r, "txid")
		ctx := context.WithValue(r.Context(), ctxTxHash, txid)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// TransactionIOIndexCtx returns a http.HandlerFunc that embeds the value at the url
// part {txinoutindex} into the request context
func TransactionIOIndexCtx(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		idxStr := chi.URLParam(r, "txinoutindex")
		idx, err := strconv.Atoi(idxStr)
		if err != nil {
			apiLog.Infof("No/invalid numeric value (%v): %v", idxStr, err)
			http.NotFound(w, r)
			return
		}
		ctx := context.WithValue(r.Context(), ctxTxInOutIndex, idx)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// AddressPathCtx returns a http.HandlerFunc that embeds the value at the url part
// {address} into the request context.
func AddressPathCtx(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		address := chi.URLParam(r, "address")
		ctx := context.WithValue(r.Context(), ctxAddress, address)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// apiDocs generates a middleware with a "docs" in the context containing a
// map of the routers handlers, etc.
func apiDocs(mux *chi.Mux) func(next http.Handler) http.Handler {
	var buf bytes.Buffer
	json.Indent(&buf, []byte(docgen.JSONRoutesDoc(mux)), "", "\t")
	docs := buf.String()
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), ctxAPIDocs, docs)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// SearchPathCtx returns a http.HandlerFunc that embeds the value at the url part
// {search} into the request context (Still need this for the error page)
// TODO: make new error system
func SearchPathCtx(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		str := chi.URLParam(r, "search")
		ctx := context.WithValue(r.Context(), ctxSearch, str)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// APIDirectory is the actual handler used with apiDocs
// (e.g. mux.With(apiDocs(mux)).HandleFunc("/help", APIDirectory))
func APIDirectory(w http.ResponseWriter, r *http.Request) {
	docs := r.Context().Value(ctxAPIDocs).(string)
	io.WriteString(w, docs)
}

// TransactionsCtx returns a http.Handlerfunc that embeds the {address, blockhash} value
// in the request into the request context
func TransactionsCtx(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		address := r.FormValue("address")
		if address != "" {
			ctx := context.WithValue(r.Context(), ctxAddress, address)
			next.ServeHTTP(w, r.WithContext(ctx))
		}

		hash := r.FormValue("block")
		if hash != "" {
			ctx := context.WithValue(r.Context(), ctxBlockHash, hash)
			next.ServeHTTP(w, r.WithContext(ctx))
		}
	})
}

// PaginationCtx returns a http.Handlerfunc that embeds the {to,from} value
// in the request into the request context
func PaginationCtx(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		to, from := r.FormValue("to"), r.FormValue("from")
		if to == "" {
			to = "20"
		}

		if from == "" {
			from = "0"
		}

		offset, err := strconv.Atoi(from)
		if err != nil {
			http.Error(w, "invalid from value", 422)
			return
		}
		count, err := strconv.Atoi(to)
		if err != nil {
			http.Error(w, "invalid to value", 422)
			return
		}

		ctx := context.WithValue(r.Context(), ctxCount, count)
		ctx = context.WithValue(ctx, ctxOffset, offset)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RawTransactionCtx returns a http.HandlerFunc that embeds the value at the url
// part {rawtx} into the request context
func RawTransactionCtx(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rawHexTx := r.PostFormValue("rawtx")
		// txid := chi.URLParam(r, "rawtx")
		ctx := context.WithValue(r.Context(), ctxRawHexTx, rawHexTx)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// AddressPostCtx returns a http.HandlerFunc that embeds the {addrs} value in the post request
// into the request context
func AddressPostCtx(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		address := r.PostFormValue("addrs")
		ctx := context.WithValue(r.Context(), ctxAddress, address)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// BlockDateQueryCtx returns a http.Handlerfunc that embeds the {blockdate, limit} value
// in the request into the request context
func BlockDateQueryCtx(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		blockDate := r.FormValue("blockDate")
		limit := r.FormValue("limit")
		if blockDate == "" {
			http.Error(w, "invalid block date", 422)
			return
		}
		fmt.Println("limit in block query ", limit)
		ctx := context.WithValue(r.Context(), ctxBlockDate, blockDate)
		ctx = context.WithValue(ctx, ctxLimit, limit)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// BlockHashPathAndIndexCtx returns a http.HandlerFunc that embeds the value at the url
// part {idx} into the request context and the block height
func BlockHashPathAndIndexCtx(r *http.Request, source DataSource) context.Context {
	hash := chi.URLParam(r, "blockhash")
	height, err := source.GetBlockHeight(hash)
	if err != nil {
		apiLog.Errorf("Unable to GetBlockHeight(%d): %v", height, err)
	}
	ctx := context.WithValue(r.Context(), ctxBlockHash, hash)
	ctx = context.WithValue(ctx, ctxBlockIndex, height)
	return ctx
}

// StatusInfoCtx returns a http.HandlerFunc that embeds the value at the url
// part {idx} into the request context
func StatusInfoCtx(r *http.Request, source DataSource) context.Context {
	idx := -1
	if source.GetHeight() >= 0 {
		idx = source.GetHeight()
	}
	ctx := context.WithValue(r.Context(), ctxBlockIndex, idx)

	q := r.FormValue("q")

	ctx = context.WithValue(ctx, ctxGetStatus, q)

	return ctx
}

// BlockHashLatestCtx returns a http.HandlerFunc
// embeds the current block height and block hash into the
// request context
func BlockHashLatestCtx(r *http.Request, source DataSource) context.Context {
	var idx int
	hash := ""

	var err error
	// if hash, err = c.BlockData.GetBestBlockHash(int64(idx)); err != nil {
	// 	apiLog.Errorf("Unable to GetBestBlockHash: %v", idx, err)
	// }
	if idx = source.GetHeight(); idx >= 0 {
		if hash, err = source.GetBlockHash(int64(idx)); err != nil {
			apiLog.Errorf("Unable to GetBlockHash(%d): %v", idx, err)
		}
	}
	ctx := context.WithValue(r.Context(), ctxBlockIndex, idx)
	ctx = context.WithValue(ctx, ctxBlockHash, hash)

	return ctx
}

// StakeVersionLatestCtx returns a http.HandlerFunc that embeds
// latest stake version into the request context
func StakeVersionLatestCtx(
	r *http.Request,
	getStakeVersionsLatest GetStakeVersionsLatest) context.Context {
	ver := -1
	stkVers, err := getStakeVersionsLatest()
	if err == nil && stkVers != nil {
		ver = int(stkVers.StakeVersion)
	}

	ctx := context.WithValue(r.Context(), ctxStakeVersionLatest, ver)
	return ctx
}

// BlockIndexLatestCtx returns a context
// embeds the current block height into the request context
func BlockIndexLatestCtx(r *http.Request, source DataSource) context.Context {
	idx := -1
	if source.GetHeight() >= 0 {
		idx = source.GetHeight()
	}
	ctx := context.WithValue(r.Context(), ctxBlockIndex, idx)
	return ctx
}

// StatusCtx returns a context embeds the
// api status into the request context
func StatusCtx(r *http.Request, status Status) context.Context {
	// Set API status context
	ctx := context.WithValue(r.Context(), ctxAPIStatus, status)
	return ctx
}

// GetBlockHeightCtx returns the current block
// height
func GetBlockHeightCtx(r *http.Request, source DataSource) int64 {
	idxI, ok := r.Context().Value(ctxBlockIndex).(int)
	idx := int64(idxI)
	if !ok || idx < 0 {
		var err error
		idx, err = source.GetBlockHeight(GetBlockHashOnlyCtx(r))
		if err != nil {
			apiLog.Errorf("Unable to GetBlockHeight: %v", err)
		}
	}
	return idx
}

// GetLatestVoteVersionCtx returns the stakeversionlatest
// embedded into the request context
func GetLatestVoteVersionCtx(r *http.Request) int {
	ver, ok := r.Context().Value(ctxStakeVersionLatest).(int)
	if !ok {
		apiLog.Error("latest stake version not set")
		return -1
	}
	return ver
}
