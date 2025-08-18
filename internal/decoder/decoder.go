package decoder

import "errors"

var (
	ErrInputContextNil = errors.New("input context is nil")

	ErrNoVideo = errors.New("no video stream found")
	ErrNoAudio = errors.New("no audio stream found")
)
