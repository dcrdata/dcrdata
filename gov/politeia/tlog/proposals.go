// Copyright (c) 2019-2021, The Decred developers
// See LICENSE for details.

// Tlog package is meant to be a rewrite that adapts politeia/proposals
// package to the new tlog backend proposals. This will replace the current
// proposals code that deals with the git backend proposals. Investigate
// how to take and maintain a snapshot of the git proposals DB
package tlog

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/asdine/storm/v3"
	"github.com/asdine/storm/v3/q"
	pitypes "github.com/decred/dcrdata/gov/v4/politeia/types"
	"github.com/decred/dcrdata/v6/semver"
	commentsv1 "github.com/decred/politeia/politeiawww/api/comments/v1"
	recordsv1 "github.com/decred/politeia/politeiawww/api/records/v1"
	ticketvotev1 "github.com/decred/politeia/politeiawww/api/ticketvote/v1"
	piclient "github.com/decred/politeia/politeiawww/client"
)

var (
	// errDef defines the default error returned if the proposals db was not
	// initialized correctly.
	errDef = fmt.Errorf("ProposalDB was not initialized correctly")

	// dbVersion is the current requirxed version of the proposals.db.
	dbVersion = semver.NewSemver(2, 0, 0)
)

// dbinfo defines the property that holds the db version.
const dbinfo = "_proposals.db_"

// ProposalsDB defines the object that interacts with the local proposals
// db, and with decred's politeia server.
type ProposalsDB struct {
	sync.Mutex

	lastSync int64 // atomic
	dbP      *storm.DB
	client   *piclient.Client
	APIPath  string
}

func NewProposalsDB(politeiaURL, dbPath string) (*ProposalsDB, error) {
	// Validate arguments
	if politeiaURL == "" {
		return nil, fmt.Errorf("missing politeia API URL")
	}
	if dbPath == "" {
		return nil, fmt.Errorf("missing db path")
	}

	// Check path and open storm DB
	_, err := os.Stat(dbPath)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	db, err := storm.Open(dbPath)
	if err != nil {
		return nil, err
	}

	// Checks if the correct db version has been set.
	var version string
	err = db.Get(dbinfo, "version", &version)
	if err != nil && err != storm.ErrNotFound {
		return nil, err
	}

	if version != dbVersion.String() {
		// Attempt to delete the ProposalInfo bucket.
		if err = db.Drop(&pitypes.ProposalInfo{}); err != nil {
			// If error due bucket not found was returned, ignore it.
			if !strings.Contains(err.Error(), "not found") {
				return nil, fmt.Errorf("delete bucket struct failed: %v", err)
			}
		}

		// Set the required db version.
		err = db.Set(dbinfo, "version", dbVersion.String())
		if err != nil {
			return nil, err
		}
		log.Infof("proposals.db version %v was set", dbVersion)
	}

	pc, err := piclient.New(politeiaURL, piclient.Opts{})
	if err != nil {
		return nil, err
	}

	proposalDB := &ProposalsDB{
		dbP:     db,
		client:  pc,
		APIPath: politeiaURL,
	}

	return proposalDB, nil
}

// Close closes the proposal DB instance.
func (db *ProposalsDB) Close() error {
	if db == nil || db.dbP == nil {
		return nil
	}

	return db.dbP.Close()
}

// ProposalsLastSync reads the last sync timestamp from the atomic db.
//
// Satisfies the PoliteiaBackend interface.
func (db *ProposalsDB) ProposalsLastSync() int64 {
	return atomic.LoadInt64(&db.lastSync)
}

// ProposalsSync is responsible for keeping an up-to-date database synced
// with politeia's latest updates. It first updates the proposals stored
// in stormdb if there status is considered to be in progress. Then it
// fetches possible new proposals since last sync, and saves them to stormdb.
//
// Satisfies the PoliteiaBackend interface.
func (db *ProposalsDB) ProposalsSync() error {
	// Sanity check
	if db == nil || db.dbP == nil {
		return errDef
	}

	// Save the timestamp of the last update check
	defer atomic.StoreInt64(&db.lastSync, time.Now().UTC().Unix())

	// Update all current proposals whose vote statuses is either
	// unauthorized, authorized and started, and that has undergone
	// some data change
	updatedCount, err := db.proposalsInProgressUpdate()
	if err != nil {
		return err
	}

	// Retrieve proposals from stormdb
	var proposals []*pitypes.ProposalInfo
	err = db.dbP.All(&proposals)
	if err != nil {
		return err
	}

	var tokens []string
	if len(proposals) == 0 {
		// Empty db so first time fetching proposals, fetch all vetted tokens
		vettedTokens, err := db.fetchVettedTokensInventory()
		if err != nil {
			return err
		}
		tokens = vettedTokens
	} else {
		// Fetch inventory to search for new proposals
		inventoryReq := recordsv1.InventoryOrdered{
			State: recordsv1.RecordStateVetted,
			Page:  1,
		}
		reply, err := db.client.RecordInventoryOrdered(inventoryReq)
		if err != nil {
			return err
		}

		// Create proposals map
		proposalsMap := make(map[string]*pitypes.ProposalInfo, len(proposals))
		for _, prop := range proposals {
			proposalsMap[prop.Token] = prop
		}

		// Filter new proposals to be fetched
		var tokensProposalsNew []string
		for _, token := range reply.Tokens {
			if _, ok := proposalsMap[token]; ok {
				// All in progress proposals were already updated, continue
				continue
			}
			// New proposal found
			tokensProposalsNew = append(tokensProposalsNew, token)
		}
		tokens = tokensProposalsNew
	}

	// Insert proposals to db, if any
	var prs []*pitypes.ProposalInfo
	if len(tokens) > 0 {
		prs, err = db.fetchProposalsData(tokens)
		if err != nil {
			return err
		}
	}

	if len(prs) > 0 {
		err = db.proposalsSave(prs)
		if err != nil {
			return err
		}
	}

	log.Infof("%d politeia records were synced",
		int(len(prs)+updatedCount))

	return nil
}

// ProposalsAll fetches the proposals data from the local db.
// The argument filterByVoteStatus is optional.
//
// Satisfies the PoliteiaBackend interface.
func (db *ProposalsDB) ProposalsAll(offset, rowsCount int,
	filterByVoteStatus ...int) ([]*pitypes.ProposalInfo, int, error) {
	// Sanity check
	if db == nil || db.dbP == nil {
		return nil, 0, errDef
	}

	var query storm.Query

	if len(filterByVoteStatus) > 0 {
		query = db.dbP.Select(q.Eq("VoteStatus",
			ticketvotev1.VoteStatusT(filterByVoteStatus[0])))
	} else {
		query = db.dbP.Select()
	}

	// Count the proposals based on the query created above.
	totalCount, err := query.Count(&pitypes.ProposalInfo{})
	if err != nil {
		return nil, 0, err
	}

	// Return the proposals listing starting with the newest.
	var proposals []*pitypes.ProposalInfo
	err = query.Skip(offset).Limit(rowsCount).Reverse().OrderBy("Timestamp").
		Find(&proposals)
	if err != nil && err != storm.ErrNotFound {
		log.Errorf("Failed to fetch data from Proposals DB: %v", err)
	} else {
		err = nil
	}

	return proposals, totalCount, nil
}

// ProposalByToken retrieves the proposal for the given token argument.
//
// Satisfies the PoliteiaBackend interface.
func (db *ProposalsDB) ProposalByToken(token string) (*pitypes.ProposalInfo, error) {
	if db == nil || db.dbP == nil {
		return nil, errDef
	}

	return db.proposal("Token", token)
}

// fetchVettedTokens fetches all vetted tokens ordered by the timestamp of
// their last status change.
func (db *ProposalsDB) fetchVettedTokensInventory() ([]string, error) {
	page := 0
	vettedTokens := []string{}
	for {
		inventoryReq := recordsv1.InventoryOrdered{
			State: recordsv1.RecordStateVetted,
			Page:  uint32(page + 1),
		}
		reply, err := db.client.RecordInventoryOrdered(inventoryReq)
		if err != nil {
			return nil, err
		}

		vettedTokens = append(vettedTokens, reply.Tokens...)

		// Break loop if we fetch last page
		if len(reply.Tokens) < int(recordsv1.InventoryPageSize) {
			break
		}
	}
	return vettedTokens, nil
}

// fetchRecordDetails fetches the record details of the given proposal tokens.
func (db *ProposalsDB) fetchRecordDetails(tokens []string) (map[string]recordsv1.Record, error) {
	records := make(map[string]recordsv1.Record, len(tokens))
	for _, token := range tokens {
		detailsReq := recordsv1.Details{
			Token: token,
		}
		dr, err := db.client.RecordDetails(detailsReq)
		if err != nil {
			return nil, err
		}
		records[token] = *dr
	}

	return records, nil
}

// fetchProposalsData returns the parsed vetted proposals from politeia
// API's. It also cooks up the data needed to save the proposals in stormdb.
// It first fetches the token inventory for the vetted proposals, then
// fetches the proposal details, then comments and then vote results.
// This data is needed for the information provided in the dcrdata UI.
func (db *ProposalsDB) fetchProposalsData(tokens []string) ([]*pitypes.ProposalInfo, error) {
	// Fetch record details for each token from the inventory
	recordDetails, err := db.fetchRecordDetails(tokens)
	if err != nil {
		return nil, err
	}

	// Fetch comments count for each token from the inventory
	cr, err := db.client.CommentCount(commentsv1.Count{
		Tokens: tokens,
	})
	if err != nil {
		return nil, err
	}
	commentsCounts := cr.Counts

	// Fetch vote summary for each token from the inventory
	sr, err := db.client.TicketVoteSummaries(ticketvotev1.Summaries{
		Tokens: tokens,
	})
	if err != nil {
		return nil, err
	}
	voteSummaries := sr.Summaries

	var proposals []*pitypes.ProposalInfo

	// Iterate through every record and feed data used by dcrdata
	for _, record := range recordDetails {
		proposal := pitypes.ProposalInfo{}

		// Record data
		proposal.State = record.State
		proposal.Status = record.Status
		proposal.Version = record.Version
		proposal.Timestamp = uint64(record.Timestamp)
		proposal.Username = record.Username
		proposal.Token = record.CensorshipRecord.Token

		// Proposal metadata
		pm, err := proposalMetadataDecode(record.Files)
		if err != nil {
			return nil, err
		}
		proposal.Name = pm.Name

		// User metadata
		um, err := userMetadataDecode(record.Metadata)
		if err != nil {
			return nil, err
		}
		proposal.UserID = um.UserID

		// Comments count
		proposal.CommentsCount = int32(commentsCounts[record.CensorshipRecord.Token])

		// Vote data
		summary := voteSummaries[proposal.Token]
		proposal.VoteStatus = summary.Status
		proposal.VoteResults = summary.Results
		proposal.EligibleTickets = summary.EligibleTickets
		proposal.StartBlockHeight = summary.StartBlockHeight
		proposal.EndBlockHeight = summary.EndBlockHeight
		proposal.QuorumPercentage = summary.QuorumPercentage
		proposal.PassPercentage = summary.PassPercentage

		// TODO: total votes
		var totalVotes uint64
		for _, v := range summary.Results {
			totalVotes += v.Votes
		}
		proposal.TotalVotes = totalVotes

		// Status change metadata
		ts, changeMsg, err := statusChangeMetadataDecode(record.Metadata)
		if err != nil {
			return nil, err
		}

		proposal.PublishedAt = ts[0]
		proposal.CensoredAt = ts[1]
		proposal.AbandonedAt = ts[2]
		proposal.StatusChangeMsg = changeMsg

		proposals = append(proposals, &proposal)
	}

	return proposals, nil
}

// proposalsSave adds the proposals data to the db.
func (db *ProposalsDB) proposalsSave(proposals []*pitypes.ProposalInfo) error {
	for _, proposal := range proposals {
		err := db.dbP.Save(proposal)
		if err != nil {
			return err
		}

		// Check if a duplicate censorship record was detected. If it exists,
		// it means that the proposal has undergone an update, and it's fixed
		// by updating the new changes to the db.
		if err == storm.ErrAlreadyExists {
			data, err := db.ProposalByToken(proposal.Token)
			updateData := proposal
			updateData.ID = data.ID
			err = db.dbP.Update(&updateData)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// proposal is used to retrieve proposals from stormdb given the search
// arguments passed in.
func (db *ProposalsDB) proposal(searchBy, searchTerm string) (*pitypes.ProposalInfo, error) {
	var proposal pitypes.ProposalInfo
	err := db.dbP.Select(q.Eq(searchBy, searchTerm)).Limit(1).First(&proposal)
	if err != nil {
		log.Errorf("Failed to fetch data from Proposals DB: %v", err)
		return nil, err
	}

	return &proposal, nil
}

// proposalsInProgressUpdate fetches proposals with the vote status equal to
// unauthorized, authorized and started. Afterwords, it proceeds to check if
// any of them need to be updated on stormdb.
func (db *ProposalsDB) proposalsInProgressUpdate() (int, error) {
	// Get proposals by vote status from storm db
	var inProgress []*pitypes.ProposalInfo
	err := db.dbP.Select(
		q.Or(
			q.Eq("VoteStatus", ticketvotev1.VoteStatusUnauthorized),
			q.Eq("VoteStatus", ticketvotev1.VoteStatusAuthorized),
			q.Eq("VoteStatus", ticketvotev1.VoteStatusStarted),
		),
	).Find(&inProgress)

	// Return an error only if the said error is not 'not found' error
	if err != nil && err != storm.ErrNotFound {
		return 0, err
	}

	// Update in progress proposals with newly fetched data from pi's API.
	countUpdated := 0
	for _, prop := range inProgress {
		proposals, err := db.fetchProposalsData([]string{prop.Token})
		if err != nil {
			return 0, fmt.Errorf("fetchProposalsData failed for token: %s", prop.Token)
		}
		proposal := proposals[0]

		// TODO: review if data compared on isEqual is enough to determine a
		// record update
		if prop.IsEqual(*proposal) {
			// No changes made to proposal
			continue
		}

		// Insert ID from storm DB to update proposal
		proposal.ID = prop.ID

		err = db.dbP.Update(proposal)
		if err != nil {
			return 0, fmt.Errorf("storm db Update failed for proposal: %s", proposal.Token)
		}

		countUpdated++
	}

	return countUpdated, nil
}
