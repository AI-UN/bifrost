package siliconflow

import (
	"time"

	"github.com/maximhq/bifrost/core/schemas"
)

// toBifrostBatchStatus converts a SiliconFlow batch status to a Bifrost batch
// status. SiliconFlow reports `in_queue` for newly accepted batches, which
// maps onto Bifrost's validating state.
func toBifrostBatchStatus(status string) schemas.BatchStatus {
	switch status {
	case "in_queue", "validating":
		return schemas.BatchStatusValidating
	case "failed":
		return schemas.BatchStatusFailed
	case "in_progress":
		return schemas.BatchStatusInProgress
	case "finalizing":
		return schemas.BatchStatusFinalizing
	case "completed":
		return schemas.BatchStatusCompleted
	case "expired":
		return schemas.BatchStatusExpired
	case "cancelling":
		return schemas.BatchStatusCancelling
	case "cancelled":
		return schemas.BatchStatusCancelled
	default:
		return schemas.BatchStatus(status)
	}
}

// toBifrostBatchErrors converts SiliconFlow's `errors: string[] | null` shape
// to Bifrost batch errors. A nil slice (wire `null`) yields nil.
func toBifrostBatchErrors(errs []string) *schemas.BatchErrors {
	if len(errs) == 0 {
		return nil
	}
	data := make([]schemas.BatchError, 0, len(errs))
	for _, message := range errs {
		if message == "" {
			continue
		}
		data = append(data, schemas.BatchError{Message: message})
	}
	if len(data) == 0 {
		return nil
	}
	return &schemas.BatchErrors{Data: data}
}

// requestCounts converts SiliconFlow request counts to Bifrost format.
func (r *SiliconFlowBatchResponse) requestCounts() schemas.BatchRequestCounts {
	if r.RequestCounts == nil {
		return schemas.BatchRequestCounts{}
	}
	return schemas.BatchRequestCounts{
		Total:     r.RequestCounts.Total,
		Completed: r.RequestCounts.Completed,
		Failed:    r.RequestCounts.Failed,
	}
}

// ToBifrostBatchCreateResponse converts a SiliconFlow batch response to a
// Bifrost batch create response.
func (r *SiliconFlowBatchResponse) ToBifrostBatchCreateResponse(latency time.Duration, sendBackRawRequest bool, sendBackRawResponse bool, rawRequest interface{}, rawResponse interface{}) *schemas.BifrostBatchCreateResponse {
	resp := &schemas.BifrostBatchCreateResponse{
		ID:               r.ID,
		Object:           r.Object,
		Endpoint:         r.Endpoint,
		InputFileID:      r.InputFileID,
		CompletionWindow: r.CompletionWindow,
		Status:           toBifrostBatchStatus(r.Status),
		RequestCounts:    r.requestCounts(),
		Metadata:         r.Metadata,
		CreatedAt:        r.CreatedAt,
		ExpiresAt:        r.ExpiresAt,
		OutputFileID:     r.OutputFileID,
		ErrorFileID:      r.ErrorFileID,
		ExtraFields: schemas.BifrostResponseExtraFields{
			Latency: latency.Milliseconds(),
		},
	}
	if sendBackRawRequest {
		resp.ExtraFields.RawRequest = rawRequest
	}
	if sendBackRawResponse {
		resp.ExtraFields.RawResponse = rawResponse
	}
	return resp
}

// ToBifrostBatchRetrieveResponse converts a SiliconFlow batch response to a
// Bifrost batch retrieve response.
func (r *SiliconFlowBatchResponse) ToBifrostBatchRetrieveResponse(latency time.Duration, sendBackRawRequest bool, sendBackRawResponse bool, rawRequest interface{}, rawResponse interface{}) *schemas.BifrostBatchRetrieveResponse {
	resp := &schemas.BifrostBatchRetrieveResponse{
		ID:               r.ID,
		Object:           r.Object,
		Endpoint:         r.Endpoint,
		InputFileID:      r.InputFileID,
		CompletionWindow: r.CompletionWindow,
		Status:           toBifrostBatchStatus(r.Status),
		RequestCounts:    r.requestCounts(),
		Metadata:         r.Metadata,
		CreatedAt:        r.CreatedAt,
		ExpiresAt:        r.ExpiresAt,
		InProgressAt:     r.InProgressAt,
		FinalizingAt:     r.FinalizingAt,
		CompletedAt:      r.CompletedAt,
		FailedAt:         r.FailedAt,
		ExpiredAt:        r.ExpiredAt,
		CancellingAt:     r.CancellingAt,
		CancelledAt:      r.CancelledAt,
		OutputFileID:     r.OutputFileID,
		ErrorFileID:      r.ErrorFileID,
		Errors:           toBifrostBatchErrors(r.Errors),
		ExtraFields: schemas.BifrostResponseExtraFields{
			Latency: latency.Milliseconds(),
		},
	}
	if sendBackRawRequest {
		resp.ExtraFields.RawRequest = rawRequest
	}
	if sendBackRawResponse {
		resp.ExtraFields.RawResponse = rawResponse
	}
	return resp
}

// ToBifrostBatchCancelResponse converts a SiliconFlow batch response to a
// Bifrost batch cancel response.
func (r *SiliconFlowBatchResponse) ToBifrostBatchCancelResponse(latency time.Duration, sendBackRawRequest bool, sendBackRawResponse bool, rawRequest interface{}, rawResponse interface{}) *schemas.BifrostBatchCancelResponse {
	resp := &schemas.BifrostBatchCancelResponse{
		ID:            r.ID,
		Object:        r.Object,
		Status:        toBifrostBatchStatus(r.Status),
		RequestCounts: r.requestCounts(),
		CancellingAt:  r.CancellingAt,
		CancelledAt:   r.CancelledAt,
		ExtraFields: schemas.BifrostResponseExtraFields{
			Latency: latency.Milliseconds(),
		},
	}
	if sendBackRawRequest {
		resp.ExtraFields.RawRequest = rawRequest
	}
	if sendBackRawResponse {
		resp.ExtraFields.RawResponse = rawResponse
	}
	return resp
}
