// Package h264 splits Annex-B streams and decodes H.264 for Android live preview.
//
// On macOS the decoder is VideoToolbox (system framework, ebitengine/purego).
// Other platforms use Cisco OpenH264.
package h264

import (
	"context"
	"image"
)

// Decoder decodes Annex-B NAL units into NRGBA frames.
type Decoder interface {
	// Decode feeds one Annex-B NAL (or access unit). A nil image means no
	// picture is ready yet.
	Decode(annexB []byte) (*image.NRGBA, error)
	Close()
}

// ProgressFunc reports a codec-library download. total is 0 when unknown.
type ProgressFunc func(downloaded, total int64)

// Percent is downloaded/total as 0–100. It is 0 when total is unknown.
func Percent(downloaded, total int64) int {
	if total <= 0 || downloaded <= 0 {
		return 0
	}
	if downloaded >= total {
		return 100
	}
	return int(downloaded * 100 / total)
}

// NewDecoder initializes a platform H.264 decoder.
func NewDecoder(ctx context.Context) (Decoder, error) {
	return NewDecoderProgress(ctx, nil)
}
