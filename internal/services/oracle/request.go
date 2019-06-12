package oracle

import (
	"context"
	"encoding/json"
	"gitlab.com/distributed_lab/logan/v3/errors"
	"gitlab.com/tokend/go/xdr"
	"gitlab.com/tokend/go/xdrbuild"
	regources "gitlab.com/tokend/regources/generated"
	"strconv"
)

const (
	TaskWithdrawReadyToSendPayment = 2048
	TaskWithdrawSending            = 4096

	//Request state
	ReviewableRequestStatePending = 1
	//page size
	requestPageSizeLimit = 10

)
func (s *Service) approveRequest(
	ctx context.Context,
	request regources.ReviewableRequest,
	toAdd, toRemove uint32,
	extDetails map[string]interface{}) error {
	id, err := strconv.ParseUint(request.ID, 10, 64)
	if err != nil {
		return errors.Wrap(err, "failed to parse request id")
	}
	bb, err := json.Marshal(extDetails)
	if err != nil {
		return errors.Wrap(err, "failed to marshal external bb map")
	}
	envelope, err := s.builder.Transaction(s.withdrawCfg.Owner).Op(xdrbuild.ReviewRequest{
		ID:     id,
		Hash:   &request.Attributes.Hash,
		Action: xdr.ReviewRequestOpActionApprove,
		Details: xdrbuild.WithdrawalDetails{
			ExternalDetails: string(bb),
		},
		ReviewDetails: xdrbuild.ReviewDetails{
			TasksToAdd:    toAdd,
			TasksToRemove: toRemove,
			ExternalDetails: string(bb),
		},
	}).Sign(s.withdrawCfg.Signer).Marshal()
	if err != nil {
		return errors.Wrap(err, "failed to prepare transaction envelope")
	}
	_, err = s.txSubmitter.Submit(ctx, envelope, false)
	if err != nil {
		return errors.Wrap(err, "failed to approve withdraw request")
	}

	return nil
}
