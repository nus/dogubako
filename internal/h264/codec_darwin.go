package h264

import "context"

// Enabled reports whether H.264 live preview may try VideoToolbox.
func Enabled() bool { return true }

// Attribution is empty on macOS: Cisco OpenH264 is not used.
func Attribution() string { return "" }

// NewDecoderProgress is NewDecoder; VideoToolbox needs no download.
func NewDecoderProgress(ctx context.Context, _ ProgressFunc) (Decoder, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return newVTDecoder()
}
