package reisen

// #cgo pkg-config: libavcodec
// #include <libavcodec/avcodec.h>
import "C"
import (
	"fmt"
)

const ()

type GenericStream struct {
	baseStream
}

var unknownDataCodec *C.AVCodec = &C.AVCodec{
	name: C.CString("Unknown data codec"),
}

var gpmdCodec *C.AVCodec = &C.AVCodec{
	name: C.CString("gopro-met"),
}

// Open opens the generic stream to decode
// frames and samples from it.
func (gs *GenericStream) Open() error {
	err := gs.open()

	if err != nil {
		fmt.Printf("GenericStream Open() error: %v\n", err)
		return err
	}

	return nil
}

// ReadFrame reads the next frame from the stream.
func (gs *GenericStream) ReadFrame() (Frame, bool, error) {
	fmt.Printf("GenericStream ReadFrame() called\n")
	return nil, false, nil
}

// Close closes the stream and
// stops decoding frames.
func (gs *GenericStream) Close() error {
	err := gs.close()

	if err != nil {
		return err
	}

	return nil
}
