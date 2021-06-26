// Copyright (c) 2019-2021, The Decred developers
// See LICENSE for details.

package politeia

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
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
	if err != nil && !errors.Is(err, storm.ErrNotFound) {
		return nil, err
	}

	if version != dbVersion.String() {
		// Attempt to delete the ProposalRecord bucket.
		if err = db.Drop(&pitypes.ProposalRecord{}); err != nil {
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

	pc, err := piclient.New(politeiaURL+"api", piclient.Opts{})
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
// in stormdb. Then it fetches possible new proposals since last sync,
// and saves them to stormdb.
//
// Satisfies the PoliteiaBackend interface.
func (db *ProposalsDB) ProposalsSync() error {
	// Sanity check
	if db == nil || db.dbP == nil {
		return errDef
	}

	// Save the timestamp of the last update check
	defer atomic.StoreInt64(&db.lastSync, time.Now().UTC().Unix())

	// Update our db with any new proposals on politeia server
	newCount, err := db.proposalsNewUpdate()
	if err != nil {
		return err
	}

	// Update all current proposals who might still be suffering changes
	// with edits, and that has undergone some data change
	ipCount, err := db.proposalsInProgressUpdate()
	if err != nil {
		return err
	}

	// Update vote results data on finished proposals that are not yet
	// fully synced with politeia
	vrCount, err := db.proposalsVoteResultsUpdate()
	if err != nil {
		return err
	}

	log.Infof("%d politeia records were synced.",
		newCount+ipCount+vrCount)

	return nil
}

// ProposalsAll fetches the proposals data from the local db.
// The argument filterByVoteStatus is optional.
//
// Satisfies the PoliteiaBackend interface.
func (db *ProposalsDB) ProposalsAll(offset, rowsCount int,
	filterByVoteStatus ...int) ([]*pitypes.ProposalRecord, int, error) {
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
	totalCount, err := query.Count(&pitypes.ProposalRecord{})
	if err != nil {
		return nil, 0, err
	}

	// Return the proposals listing starting with the newest.
	var proposals []*pitypes.ProposalRecord
	err = query.Skip(offset).Limit(rowsCount).Reverse().OrderBy("Timestamp").
		Find(&proposals)
	if err != nil && !errors.Is(err, storm.ErrNotFound) {
		log.Errorf("Failed to fetch data from Proposals DB: %v", err)
	}

	return proposals, totalCount, nil
}

// ProposalByToken retrieves the proposal for the given token argument.
//
// Satisfies the PoliteiaBackend interface.
func (db *ProposalsDB) ProposalByToken(token string) (*pitypes.ProposalRecord, error) {
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
			return nil, fmt.Errorf("Pi client RecordInventoryOrdered err: %v",
				err)
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
			return nil, fmt.Errorf("Pi client RecordDetails err: %v", err)
		}
		records[token] = *dr
	}

	return records, nil
}

// fetchProposalsData returns the parsed vetted proposals from politeia
// API's. It cooks up the data needed to save the proposals in stormdb. It
// first fetches the proposal details, then comments and then vote summary.
// This data is needed for the information provided in the dcrdata UI. The
// data returned does not include ticket vote data.
func (db *ProposalsDB) fetchProposalsData(tokens []string) ([]*pitypes.ProposalRecord, error) {
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
		return nil, fmt.Errorf("Pi client CommentCount err: %v", err)
	}
	commentsCounts := cr.Counts

	// Fetch vote summary for each token from the inventory
	sr, err := db.client.TicketVoteSummaries(ticketvotev1.Summaries{
		Tokens: tokens,
	})
	if err != nil {
		return nil, fmt.Errorf("Pi client TicketVoteSummaries err: %v", err)
	}
	voteSummaries := sr.Summaries

	var proposals []*pitypes.ProposalRecord

	// Iterate through every record and feed data used by dcrdata
	for _, record := range recordDetails {
		proposal := pitypes.ProposalRecord{}

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
			return nil, fmt.Errorf("proposalMetadataDecode err: %v", err)
		}
		proposal.Name = pm.Name

		// User metadata
		um, err := userMetadataDecode(record.Metadata)
		if err != nil {
			return nil, fmt.Errorf("userMetadataDecode err: %v", err)
		}
		proposal.UserID = um.UserID

		// Comments count
		proposal.CommentsCount =
			int32(commentsCounts[record.CensorshipRecord.Token])

		// Vote data
		summary := voteSummaries[proposal.Token]
		proposal.VoteStatus = summary.Status
		proposal.VoteResults = summary.Results
		proposal.EligibleTickets = summary.EligibleTickets
		proposal.StartBlockHeight = summary.StartBlockHeight
		proposal.EndBlockHeight = summary.EndBlockHeight
		proposal.QuorumPercentage = summary.QuorumPercentage
		proposal.PassPercentage = summary.PassPercentage

		var totalVotes uint64
		for _, v := range summary.Results {
			totalVotes += v.Votes
		}
		proposal.TotalVotes = totalVotes

		// Status change metadata
		ts, changeMsg, err := statusChangeMetadataDecode(record.Metadata)
		if err != nil {
			return nil, fmt.Errorf("statusChangeMetadataDecode err: %v", err)
		}
		proposal.PublishedAt = ts[0]
		proposal.CensoredAt = ts[1]
		proposal.AbandonedAt = ts[2]
		proposal.StatusChangeMsg = changeMsg

		// Append proposal after inserting the relevant data
		proposals = append(proposals, &proposal)
	}

	return proposals, nil
}

func (db *ProposalsDB) fetchTicketVoteResults(token string) (*pitypes.ProposalChartData, error) {
	// Fetch ticket votes details to acquire vote bits options info.
	details, err := db.client.TicketVoteDetails(ticketvotev1.Details{
		Token: token,
	})
	if err != nil {
		return nil, fmt.Errorf("Pi client TicketVoteDetails err: %v", err)
	}

	// Maps the vote bits option to their respective string ID.
	voteOptsMap := make(map[uint64]string)
	for _, opt := range details.Vote.Params.Options {
		voteOptsMap[opt.Bit] = opt.ID
	}

	tvr, err := db.client.TicketVoteResults(ticketvotev1.Results{
		Token: token,
	})
	if err != nil {
		return nil, fmt.Errorf("Pi client TicketVoteResults err: %v", err)
	}

	// Parse reply to proposal chart data
	var chart pitypes.ProposalChartData
	var timestamps []int64
	for _, v := range tvr.Votes {
		bit, err := strconv.ParseUint(v.VoteBit, 16, 64)
		if err != nil {
			return nil, err
		}
		switch voteOptsMap[bit] {
		case "yes":
			chart.Yes = append(chart.Yes, 1)
			chart.No = append(chart.No, 0)
		case "no":
			chart.No = append(chart.No, 1)
			chart.Yes = append(chart.Yes, 0)
		}
		timestamps = append(timestamps, v.Timestamp)
	}
	sort.Slice(timestamps, func(i, j int) bool {
		return timestamps[i] < timestamps[j]
	})
	chart.Time = timestamps

	return &chart, nil
}

// proposalsSave saves the proposals data to the db.
func (db *ProposalsDB) proposalsSave(proposals []*pitypes.ProposalRecord) error {
	for _, proposal := range proposals {
		proposal.Synced = false
		err := db.dbP.Save(proposal)
		if errors.Is(err, storm.ErrAlreadyExists) {
			// Proposal exists, update instead of inserting new
			data, err := db.ProposalByToken(proposal.Token)
			if err != nil {
				return fmt.Errorf("ProposalsDB ProposalByToken err: %v", err)
			}
			updateData := *proposal
			updateData.ID = data.ID
			err = db.dbP.Update(&updateData)
			if err != nil {
				return fmt.Errorf("stormdb update err: %v", err)
			}
		}
		if err != nil {
			return fmt.Errorf("stormdb save err: %v", err)
		}
	}

	return nil
}

// proposal is used to retrieve proposals from stormdb given the search
// arguments passed in.
func (db *ProposalsDB) proposal(searchBy, searchTerm string) (*pitypes.ProposalRecord, error) {
	var proposal pitypes.ProposalRecord
	err := db.dbP.Select(q.Eq(searchBy, searchTerm)).Limit(1).First(&proposal)
	if err != nil {
		log.Errorf("Failed to fetch data from Proposals DB: %v", err)
		return nil, err
	}

	return &proposal, nil
}

// proposalsNewUpdate verifies if there is any new proposals on the politeia
// server that are not yet synced with our stormdb.
func (db *ProposalsDB) proposalsNewUpdate() (int, error) {
	var proposals []*pitypes.ProposalRecord
	err := db.dbP.All(&proposals)
	if err != nil {
		return 0, fmt.Errorf("stormdb All err: %v", err)
	}

	var tokens []string
	if len(proposals) == 0 {
		// Empty db so first time fetching proposals, fetch all vetted tokens
		vettedTokens, err := db.fetchVettedTokensInventory()
		if err != nil {
			return 0, err
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
			return 0, fmt.Errorf("Pi client RecordInventoryOrdered err: %v",
				err)
		}

		// Create proposals map
		proposalsMap := make(map[string]*pitypes.ProposalRecord, len(proposals))
		for _, prop := range proposals {
			proposalsMap[prop.Token] = prop
		}

		// Filter new proposals to be fetched
		var tokensProposalsNew []string
		for _, token := range reply.Tokens {
			if _, ok := proposalsMap[token]; ok {
				continue
			}
			// New proposal found
			tokensProposalsNew = append(tokensProposalsNew, token)
		}
		tokens = tokensProposalsNew
	}

	// Fetch data for found tokens
	var prs []*pitypes.ProposalRecord
	if len(tokens) > 0 {
		prs, err = db.fetchProposalsData(tokens)
		if err != nil {
			return 0, err
		}
	}

	// Save proposals data on stormdb
	if len(prs) > 0 {
		err = db.proposalsSave(prs)
		if err != nil {
			return 0, err
		}
	}

	return len(prs), nil
}

// proposalsInProgressUpdate fetches proposals with the vote status equal to
// unauthorized, authorized and started. Afterwords, it proceeds to check if
// any of them need to be updated on stormdb.
func (db *ProposalsDB) proposalsInProgressUpdate() (int, error) {
	// Get proposals by vote status from storm db
	var propsInProgress []*pitypes.ProposalRecord
	err := db.dbP.Select(
		q.Or(
			q.Eq("VoteStatus", ticketvotev1.VoteStatusUnauthorized),
			q.Eq("VoteStatus", ticketvotev1.VoteStatusAuthorized),
			q.Eq("VoteStatus", ticketvotev1.VoteStatusStarted),
		),
	).Find(&propsInProgress)
	if err != nil && !errors.Is(err, storm.ErrNotFound) {
		return 0, err
	}

	// Update in progress proposals with newly fetched data from pi's API.
	countUpdated := 0
	for _, prop := range propsInProgress {
		proposals, err := db.fetchProposalsData([]string{prop.Token})
		if err != nil {
			return 0, fmt.Errorf("fetchProposalsData failed with err: %s", err)
		}
		proposal := proposals[0]

		// If voting on the proposal has already started, sync ticket vote
		// results data as well.
		if prop.VoteStatus == ticketvotev1.VoteStatusStarted {
			voteResults, err := db.fetchTicketVoteResults(prop.Token)
			if err != nil {
				return 0, fmt.Errorf("fetchTicketVoteResults failed with err: %s", err)
			}
			proposal.ChartData = voteResults
		}

		if prop.IsEqual(*proposal) {
			// No changes made to proposal, skip db call.
			continue
		}

		// Insert ID from storm DB to update proposal.
		proposal.ID = prop.ID

		err = db.dbP.Update(proposal)
		if err != nil {
			return 0, fmt.Errorf("storm db Update failed with err: %s", err)
		}

		countUpdated++
	}

	return countUpdated, nil
}

// proposalsVoteResultsUpdate verifies if there is still a need to update vote
// results data for proposals with the vote status equal to finished, approved
// and rejected.
func (db *ProposalsDB) proposalsVoteResultsUpdate() (int, error) {
	// Get proposals that need to be synced
	var propsVotingComplete []*pitypes.ProposalRecord
	err := db.dbP.Select(
		q.Or(
			q.And(
				q.Eq("VoteStatus", ticketvotev1.VoteStatusFinished),
				q.Eq("Synced", false),
			),
			q.And(
				q.Eq("VoteStatus", ticketvotev1.VoteStatusApproved),
				q.Eq("Synced", false),
			), q.And(
				q.Eq("VoteStatus", ticketvotev1.VoteStatusRejected),
				q.Eq("Synced", false),
			),
		),
	).Find(&propsVotingComplete)
	if err != nil && !errors.Is(err, storm.ErrNotFound) {
		return 0, err
	}

	// Update finished proposals that are not yet synced with the
	// latest vote results.
	countUpdated := 0
	for _, prop := range propsVotingComplete {
		voteResults, err := db.fetchTicketVoteResults(prop.Token)
		if err != nil {
			return 0, fmt.Errorf("fetchTicketVoteResults failed with err: %s", err)
		}
		prop.ChartData = voteResults
		prop.Synced = true

		err = db.dbP.Update(prop)
		if err != nil {
			return 0, fmt.Errorf("storm db Update failed with err: %s", err)
		}

		countUpdated++
	}

	return countUpdated, nil
}
