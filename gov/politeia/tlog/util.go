package tlog

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/decred/politeia/politeiad/plugins/usermd"
	piv1 "github.com/decred/politeia/politeiawww/api/pi/v1"
	recordsv1 "github.com/decred/politeia/politeiawww/api/records/v1"
)

func userMetadataDecode(ms []recordsv1.MetadataStream) (*usermd.UserMetadata, error) {
	var userMD *usermd.UserMetadata
	for _, m := range ms {
		if m.PluginID != usermd.PluginID ||
			m.StreamID != usermd.StreamIDUserMetadata {
			// This is not user metadata
			continue
		}
		var um usermd.UserMetadata
		err := json.Unmarshal([]byte(m.Payload), &um)
		if err != nil {
			return nil, err
		}
		userMD = &um
		break
	}
	return userMD, nil
}

func proposalMetadataDecode(fs []recordsv1.File) (*piv1.ProposalMetadata, error) {
	var pmp *piv1.ProposalMetadata
	for _, f := range fs {
		if f.Name != piv1.FileNameProposalMetadata {
			continue
		}
		b, err := base64.StdEncoding.DecodeString(f.Payload)
		if err != nil {
			return nil, err
		}
		var pm piv1.ProposalMetadata
		err = json.Unmarshal(b, &pm)
		if err != nil {
			return nil, err
		}
		pmp = &pm
		break
	}
	if pmp == nil {
		return nil, fmt.Errorf("proposal metadata not found")
	}
	return pmp, nil
}
