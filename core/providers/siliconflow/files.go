package siliconflow

import (
	"time"

	"github.com/maximhq/bifrost/core/schemas"
)

// toBifrostFileStatus maps SiliconFlow file status strings onto Bifrost file statuses.
func toBifrostFileStatus(status string) schemas.FileStatus {
	switch status {
	case "processed":
		return schemas.FileStatusProcessed
	case "uploaded":
		return schemas.FileStatusUploaded
	case "error":
		return schemas.FileStatusError
	default:
		return schemas.FileStatus(status)
	}
}

// ToBifrostFileUploadResponse converts a SiliconFlow file object to a Bifrost
// file upload response.
func (f *SiliconFlowFileObject) ToBifrostFileUploadResponse(latency time.Duration, sendBackRawRequest bool, sendBackRawResponse bool, rawRequest interface{}, rawResponse interface{}) *schemas.BifrostFileUploadResponse {
	resp := &schemas.BifrostFileUploadResponse{
		ID:        f.ID,
		Object:    f.Object,
		Bytes:     f.Bytes,
		CreatedAt: f.createdAtUnix(),
		Filename:  f.Filename,
		Purpose:   schemas.FilePurpose(f.Purpose),
		Status:    toBifrostFileStatus(f.Status),
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

// ToFileObject converts a SiliconFlow file object to a Bifrost file object.
func (f *SiliconFlowFileObject) ToFileObject() schemas.FileObject {
	return schemas.FileObject{
		ID:        f.ID,
		Object:    f.Object,
		Bytes:     f.Bytes,
		CreatedAt: f.createdAtUnix(),
		Filename:  f.Filename,
		Purpose:   schemas.FilePurpose(f.Purpose),
		Status:    toBifrostFileStatus(f.Status),
	}
}
