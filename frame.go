package reisen

import (
	"fmt"
	"time"
)

// Frame is an abstract data frame.
type Frame interface {
	Data() []byte
	PresentationOffset() (time.Duration, error)
	PresentationOffsetOrDie() time.Duration
}

// baseFrame contains the information
// common for all frames of any type.
type baseFrame struct {
	stream       Stream
	pts          int64
	indexCoded   int
	indexDisplay int
}

// PresentationOffset returns the duration offset
// since the start of the media at which the frame
// should be played.
func (frame *baseFrame) PresentationOffset() (time.Duration, error) {
	tbNum, tbDen := frame.stream.TimeBase()
	tb := float64(tbNum) / float64(tbDen)
	tm := float64(frame.pts) * tb

	return time.ParseDuration(fmt.Sprintf("%fs", tm))
}

// PresentationOffsetOrDie is a more convenient function for
// obtaining the presentation offset. It allows to obtain
// a presentation offset in one call, at the cost of possible
// catastrophic failure of the application.
func (frame *baseFrame) PresentationOffsetOrDie() time.Duration {
	ts, err := frame.PresentationOffset()
	if err != nil {
		panic(err)
	}
	return ts
}

// IndexCoded returns the index of
// the frame in the bitstream order.
func (frame *baseFrame) IndexCoded() int {
	return frame.indexCoded
}

// IndexDisplay returns the index of
// the frame in the display order.
func (frame *baseFrame) IndexDisplay() int {
	return frame.indexDisplay
}
