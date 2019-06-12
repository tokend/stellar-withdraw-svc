package submit

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/tokend/stellar-withdraw-svc/internal/horizon/client"
	"gitlab.com/tokend/go/xdr"
	"net/http"

	regources "gitlab.com/tokend/regources/generated"

	"gitlab.com/distributed_lab/logan/v3/errors"
)

var (
	ErrSubmitTimeout              = errors.New("submit timed out")
	ErrSubmitInternal             = errors.New("internal submit error")
	ErrSubmitUnexpectedStatusCode = errors.New("Unexpected unsuccessful status code.")
)

type TxFailure struct {
	error
	ResultXDR             string
	TransactionResultCode string
	OperationResultCodes  []string
}

type txFailureResponse struct {
	Errors []struct {
		Title  string `json:"title"`
		Detail string `json:"detail"`
		Status string `json:"status"`
		Meta   *struct {
			Envelope     string                `json:"envelope"`
			ResultXDR    string                `json:"result_xdr"`
			ParsedResult xdr.TransactionResult `json:"parsed_result"`
			ResultCodes  struct {
				TransactionCode string   `json:"transaction"`
				OperationCodes  []string `json:"operations,omitempty"`
				Messages        []string `json:"messages"`
			} `json:"result_codes"`
		} `json:"meta,omitempty"`
	} `json:"errors"`
}

type Interface interface {
	Submit(ctx context.Context, envelope string, waitIngest bool) (*regources.TransactionResponse, error)
}

type submitter struct {
	*client.Client
}

func New(cl *client.Client) *submitter {
	return &submitter{
		Client: cl,
	}
}

func (s *submitter) Submit(ctx context.Context, envelope string, waitIngest bool) (*regources.TransactionResponse, error) {
	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(&regources.SubmitTransactionBody{
		Tx:            envelope,
		WaitForIngest: &waitIngest,
	})
	if err != nil {
		panic(errors.Wrap(err, "failed to marshal request"))
	}
	url, err := s.Resolve().URL("/v3/transactions")
	if err != nil {
		return nil, errors.Wrap(err, "failed to resolve url")
	}
	r, err := http.NewRequest("POST", url, &buf)
	if err != nil {
		return nil, errors.Wrap(err, "failed to prepare request")
	}
	status, response, err := s.Do(r)

	if isStatusCodeSuccessful(status) && err == nil {
		var success regources.TransactionResponse
		if err := json.Unmarshal(response, &success); err != nil {
			return nil, errors.Wrap(err, "failed to unmarshal transaction response")
		}
		return &success, nil
	}

	// go through known response codes and try to build meaningful result
	switch status {
	case http.StatusGatewayTimeout: // timeout
		return nil, ErrSubmitTimeout
	case http.StatusBadRequest: // rejected or malformed
		// check which error it was exactly, might be useful for consumer
		var failureResp txFailureResponse
		if err := json.Unmarshal(response, &failureResp); err != nil {
			panic(errors.Wrap(err, "failed to unmarshal horizon response"))
		}
		return nil, TxFailure{
			error:                 errors.New(failureResp.Errors[0].Detail),
			ResultXDR:             failureResp.Errors[0].Meta.ResultXDR,
			OperationResultCodes:  failureResp.Errors[0].Meta.ResultCodes.OperationCodes,
			TransactionResultCode: failureResp.Errors[0].Meta.ResultCodes.TransactionCode,
		}
	case http.StatusInternalServerError: // internal error
		return nil, ErrSubmitInternal
	default:
		return nil, ErrSubmitUnexpectedStatusCode
	}
}

func isStatusCodeSuccessful(code int) bool {
	return code >= 200 && code < 300
}
