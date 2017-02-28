package dcrsqlite

import (
	"database/sql"

	apitypes "github.com/dcrdata/dcrdata/dcrdataapi"
	"github.com/dcrdata/dcrdata/rpcutils"
	"github.com/decred/dcrd/chaincfg"
	"github.com/decred/dcrd/dcrjson"
	"github.com/decred/dcrrpcclient"
)

// wiredDB is intended to satisfy APIDataSource interface. The block header is
// not stored in the DB, so the RPC client is used to get it on demand.
type wiredDB struct {
	*DB
	client *dcrrpcclient.Client
	params *chaincfg.Params
}

func NewWiredDB(db *sql.DB, cl *dcrrpcclient.Client, p *chaincfg.Params) wiredDB {
	return wiredDB{
		DB:     NewDB(db),
		client: cl,
		params: p,
	}
}

func InitWiredDB(dbInfo *DBInfo, cl *dcrrpcclient.Client, p *chaincfg.Params) (wiredDB, error) {
	db, err := InitDB(dbInfo)
	if err != nil {
		return wiredDB{}, err
	}
	return wiredDB{
		DB:     db,
		client: cl,
		params: p,
	}, nil
}

func (db *wiredDB) GetHeight() int {
	return int(db.dbSummaryHeight)
}

func (db *wiredDB) GetHeader(idx int) *dcrjson.GetBlockHeaderVerboseResult {
	return rpcutils.GetBlockHeaderVerbose(db.client, db.params, int64(idx))
}

func (db *wiredDB) GetStakeDiffEstimates() *apitypes.StakeDiff {
	return rpcutils.GetStakeDiffEstimates(db.client)
}

func (db *wiredDB) GetFeeInfo(idx int) *dcrjson.FeeInfoBlock {
	stakeInfo, err := db.RetrieveStakeInfoExtended(int64(idx))
	if err != nil {
		log.Errorf("Unable to retrieve stake info: %v", err)
		return nil
	}

	return &stakeInfo.Feeinfo
}

func (db *wiredDB) GetStakeInfoExtended(idx int) *apitypes.StakeInfoExtended {
	stakeInfo, err := db.RetrieveStakeInfoExtended(int64(idx))
	if err != nil {
		log.Errorf("Unable to retrieve stake info: %v", err)
		return nil
	}

	return stakeInfo
}

func (db *wiredDB) GetSummary(idx int) *apitypes.BlockDataBasic {
	blockSummary, err := db.RetrieveBlockSummary(int64(idx))
	if err != nil {
		log.Errorf("Unable to retrieve block summary: %v", err)
		return nil
	}

	return blockSummary
}

func (db *wiredDB) GetBestBlockSummary() *apitypes.BlockDataBasic {
	blockSummary, err := db.RetrieveBlockSummary(db.dbSummaryHeight)
	if err != nil {
		log.Errorf("Unable to retrieve block summary: %v", err)
		return nil
	}

	return blockSummary
}
