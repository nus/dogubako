//go:build !darwin

package h264

import (
	"context"

	"github.com/nus/dogubako/internal/openh264"
)

// Enabled reports whether H.264 live preview may try OpenH264.
func Enabled() bool { return openh264.Enabled() }

// Attribution is the credit required by Cisco's binary license.
func Attribution() string { return openh264.Attribution }

// NewDecoderProgress is NewDecoder with optional OpenH264 download progress.
func NewDecoderProgress(ctx context.Context, progress ProgressFunc) (Decoder, error) {
	return openh264.NewDecoderProgress(ctx, openh264.ProgressFunc(progress))
}
