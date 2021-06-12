// Copyright (c) 2019-2021, The Decred developers
// See LICENSE for details.

package types

import (
	"github.com/decred/dcrdata/v6/db/dbtypes"
	recordsv1 "github.com/decred/politeia/politeiawww/api/records/v1"
	ticketvotev1 "github.com/decred/politeia/politeiawww/api/ticketvote/v1"
)

// Politeia votes occur in 2016 block windows.
const windowSize = 2016

// Proposaln is the struct that holds all politeia data that dcrdata needs.
// It is also the object data that we save to the database. It fetches data
// from three politeia api's: records, comments and ticketvote.
type ProposalInfo struct {
	ID int `json:"id" storm:"id,increment"`

	// Record API data
	State     recordsv1.RecordStateT  `json:"state"`
	Status    recordsv1.RecordStatusT `json:"status"`
	Token     string                  `json:"token"`
	Version   uint32                  `json:"version"`
	Timestamp int64                   `json:"timestamp"`
	Username  string                  `json:"username"`

	// Pi metadata
	Name string `json:"name"`

	// User metadata
	UserID string `json:"userid"`

	// Comments API data
	CommentsCount int32 `json:"commentscount`

	// Ticketvote API data
	VoteStatus       ticketvotev1.VoteStatusT  `json:"votestatus"`
	VoteResults      []ticketvotev1.VoteResult `json:"voteresults"`
	StatusChangeMsg  string                    `json:"statuschangemsg"`
	EligibleTickets  uint32                    `json:"eligibletickets"`
	StartBlockHeight uint32                    `json:"startblockheight"`
	EndBlockHeight   uint32                    `json:"endblockheight"`
	QuorumPercentage uint32                    `json:"quorumpercentage"`
	PassPercentage   uint32                    `json:"passpercentage"`

	TotalVotes uint64 `json:"totalvotes"`

	// Timestamps
	PublishedAt uint64 `json:"publishedat" storm:"index"`
	CensoredAt  uint64 `json:"censoredat"`
	AbandonedAt uint64 `json:"abandonedat"`
}

// ProposalsChartData defines the data used to plot proposal votes charts.
type ProposalsChartData struct {
	Yes  []uint64          `json:"yes,omitempty"`
	No   []uint64          `json:"no,omitempty"`
	Time []dbtypes.TimeDef `json:"time,omitempty"`
}

// VoteStatusType defines the various vote statuses available as referenced in
// https://github.com/decred/politeia/blob/master/politeiawww/api/www/v1/v1.go
type VoteStatusType ticketvotev1.VoteStatusT

// ShorterDesc maps the short description to there respective vote status type.
var ShorterDesc = map[ticketvotev1.VoteStatusT]string{
	ticketvotev1.VoteStatusInvalid:      "Invalid",
	ticketvotev1.VoteStatusUnauthorized: "Unauthorized",
	ticketvotev1.VoteStatusAuthorized:   "Authorized",
	ticketvotev1.VoteStatusStarted:      "Started",
	ticketvotev1.VoteStatusFinished:     "Finished",
	ticketvotev1.VoteStatusApproved:     "Approved",
	ticketvotev1.VoteStatusRejected:     "Rejected",
}

// ShortDesc returns the shorter vote status description.
func (s VoteStatusType) ShortDesc() string {
	return ShorterDesc[ticketvotev1.VoteStatusT(s)]
}

// VotesStatuses returns the ShorterDesc map contents exclusive of Invalid and
// Doesn't exist statuses.
func VotesStatuses() map[VoteStatusType]string {
	m := make(map[VoteStatusType]string)
	for k, val := range ShorterDesc {
		if k == ticketvotev1.VoteStatusInvalid {
			continue
		}
		m[VoteStatusType(k)] = val
	}
	return m
}

func (pi ProposalInfo) VoteStatusDesc() string {
	return ticketvotev1.VoteStatuses[pi.VoteStatus]
}

// IsEqual compares CensorshipRecord, Name, State, NumComments, StatusChangeMsg,
// Timestamp, CensoredDate, AbandonedDate, PublishedDate, Token, VoteStatus,
// TotalVotes and count of VoteResults between the two ProposalsInfo structs passed.
// nts: update comment
func (pi *ProposalInfo) IsEqual(b ProposalInfo) bool {
	if pi.Token != b.Token || pi.Name != b.Name || pi.State != b.State ||
		pi.CommentsCount != b.CommentsCount ||
		pi.StatusChangeMsg != b.StatusChangeMsg ||
		pi.Status != b.Status || pi.Timestamp != b.Timestamp ||
		pi.VoteStatus != b.VoteStatus || pi.TotalVotes != b.TotalVotes ||
		pi.PublishedAt != b.PublishedAt ||
		pi.CensoredAt != b.CensoredAt || pi.AbandonedAt != b.AbandonedAt {
		return false
	}
	return true
}

// ProposalMetadata contains some status-dependent data representations for
// display purposes.
type ProposalMetadata struct {
	// Time until start for "Authorized" proposals, Time until done for "Started"
	// proposals.
	SecondsTil         int64
	IsPassing          bool
	Approval           float32
	Rejection          float32
	Yes                int64
	No                 int64
	VoteCount          int64
	QuorumCount        int64
	QuorumAchieved     bool
	PassPercent        float32
	VoteStatusDesc     string
	ProposalStateDesc  string
	ProposalStatusDesc string
}

// Metadata performs some common manipulations of the ProposalInfo data to
// prepare figures for display. Many of these manipulations require a tip height
// and a target block time for the network, so those must be provided as
// arguments.
func (pi *ProposalInfo) Metadata(tip, targetBlockTime int64) *ProposalMetadata {
	meta := new(ProposalMetadata)
	// desc := ticketvotev1.VoteStatuses[pi.VoteStatus]
	switch pi.VoteStatus {
	case ticketvotev1.VoteStatusStarted, ticketvotev1.VoteStatusFinished:
		for _, count := range pi.VoteResults {
			switch count.ID {
			case "yes":
				meta.Yes = int64(count.Votes)
			case "no":
				meta.No = int64(count.Votes)
			}
		}
		meta.VoteCount = meta.Yes + meta.No
		quorumPct := float32(pi.QuorumPercentage) / 100
		meta.QuorumCount = int64(quorumPct * float32(pi.EligibleTickets))
		meta.PassPercent = float32(pi.PassPercentage) / 100
		pctVoted := float32(meta.VoteCount) / float32(pi.EligibleTickets)
		meta.QuorumAchieved = pctVoted > quorumPct
		if meta.VoteCount > 0 {
			meta.Approval = float32(meta.Yes) / float32(meta.VoteCount)
			meta.Rejection = 1 - meta.Approval
		}
		meta.IsPassing = meta.Approval > meta.PassPercent
		if pi.VoteStatus == ticketvotev1.VoteStatusStarted {
			blocksLeft := int64(pi.EndBlockHeight) - tip
			meta.SecondsTil = blocksLeft * targetBlockTime
		}
	}
	meta.VoteStatusDesc = ticketvotev1.VoteStatuses[pi.VoteStatus]
	meta.ProposalStateDesc = recordsv1.RecordStates[pi.State]
	meta.ProposalStatusDesc = recordsv1.RecordStatuses[pi.Status]
	return meta
}
