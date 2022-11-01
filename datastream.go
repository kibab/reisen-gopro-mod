package reisen

// #cgo pkg-config: libavcodec
// #include <libavcodec/avcodec.h>
import "C"
import (
	"fmt"
)

// There are no implementations for data stream codecs in FFMPeg library.
// Here, we define an implementation for GoPro Metadata format.
// All other data streams are assigned to a "generic" data codec that
// doesn't do anything.
const (
	gpmdCodecTag int = 0x646d7067
)

type NativeCodec struct {
	ffmpegCodec *C.AVCodec
	handler     func(*Packet)
}

func (nc *NativeCodec) FFMPEGCodec() *C.AVCodec {
	return nc.ffmpegCodec
}

type DataStream struct {
	baseStream
	nativeCodec NativeCodec
}

// DataFrame is a data frame
// obtained from an data stream.
type DataFrame struct {
	baseFrame
	data []byte
}

// Data returns a raw slice of
// audio frame samples.
func (frame *DataFrame) Data() []byte {
	return frame.data
}

// newDataFrame returns a newly created data frame.
func newDataFrame(stream Stream, pts int64, data []byte) *DataFrame {
	frame := new(DataFrame)

	frame.stream = stream
	frame.pts = pts
	frame.data = data

	return frame
}

// The following definitions are needed to make the rest of code happy
// when printing codec names.
var unknownDataCodec *C.AVCodec = &C.AVCodec{
	name: C.CString("unknown-data-codec"),
}

var gpmdCodec *C.AVCodec = &C.AVCodec{
	name: C.CString("gopro-met"),
}

var tagToNativeCodec map[int]NativeCodec = map[int]NativeCodec{
	0: {
		ffmpegCodec: unknownDataCodec,
		handler:     genericFrameHandler,
	},
	gpmdCodecTag: {
		ffmpegCodec: gpmdCodec,
		handler:     gmpdFrameHandler,
	},
}

// DataCodecByTag returns a NativeCodec by a given tag.
// If there is no codec, a generic implementation is used.
// This functin never fails.
func DataCodecByTag(tag int) NativeCodec {
	var nativeCodec NativeCodec
	var ok bool
	if nativeCodec, ok = tagToNativeCodec[tag]; !ok {
		return tagToNativeCodec[0]
	}
	return nativeCodec
}

// Open opens the data stream to decode
// frames and samples from it.
// We don't call into baseStream.open here because we don't need to init
// any FFMPeg structures.
func (gs *DataStream) Open() error {
	return nil
}

// ReadFrame reads the next frame from the stream.
func (gs *DataStream) ReadFrame() (Frame, bool, error) {
	fmt.Printf("DataStream (codec %s) ReadFrame() called\n", gs.CodecName())
	pkt := newPacket(gs.media, gs.media.packet)
	gs.nativeCodec.handler(pkt)
	frame := newDataFrame(gs, pkt.pts, pkt.Data())
	return frame, true, nil
}

func gmpdFrameHandler(pkt *Packet) {
	fmt.Printf("About to decode GPMD data w/ size %d, pts %d\n", len(pkt.Data()), pkt.pts)
	if err := DecodeGoproData(pkt.Data()); err != nil {
		fmt.Printf("cannot handle packet: %v", err)
	}
}

func genericFrameHandler(pkt *Packet) {
	/* Do nothing */
}

// Close closes the stream and
// stops decoding frames.
func (gs *DataStream) Close() error {
	return nil
}
