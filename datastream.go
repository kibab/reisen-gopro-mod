package reisen

// #cgo pkg-config: libavcodec
// #include <libavcodec/avcodec.h>
import "C"
import (
	"bytes"
	"fmt"
	"io"

	"github.com/kibab/gopro-utils/telemetry"
)

// There are no implementations for data stream codecs in FFMPeg library.
// Here, we define an implementation for GoPro Metadata format.
// All other data streams are assigned to a "generic" data codec that
// doesn't do anything.
const (
	gpmdCodecTag int = 0x646d7067 // 'gmpd' in little-engian
)

type NativeCodec struct {
	ffmpegCodec *C.AVCodec
	handler     func(*DataStream, *Packet) (Frame, bool)
}

func (nc *NativeCodec) FFMPEGCodec() *C.AVCodec {
	return nc.ffmpegCodec
}

type DataStream struct {
	baseStream
	nativeCodec NativeCodec
}

// Parsed data from GPMD packet
type TelemetryData struct {
	Lat, Long float64
}

// DataFrame is a data frame
// obtained from an data stream.
type DataFrame struct {
	baseFrame
	data  []byte
	tData TelemetryData
}

// Data returns a raw slice of
// data frame samples.
func (frame *DataFrame) Data() []byte {
	return frame.data
}

func (frame *DataFrame) Telemetry() TelemetryData {
	return frame.tData
}

// newDataFrame returns a newly created data frame.
func newDataFrame(stream Stream, pts int64, data []byte, telemetryData TelemetryData) *DataFrame {
	frame := new(DataFrame)

	frame.stream = stream
	frame.pts = pts
	frame.data = data
	frame.tData = telemetryData
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
	pkt := newPacket(gs.media, gs.media.packet)
	frame, handled := gs.nativeCodec.handler(gs, pkt)
	if !handled {
		return nil, false, nil
	}
	return frame, true, nil
}

func gmpdFrameHandler(gs *DataStream, pkt *Packet) (Frame, bool) {
	tdata := TelemetryData{
		Lat:  21,
		Long: 11,
	}
	return newDataFrame(gs, pkt.pts, pkt.Data(), tdata), true

	r := bytes.NewReader(pkt.Data())
	for {
		telem, err := telemetry.Read(r)
		if telem != nil && telem.Gps != nil {
			tdata := TelemetryData{
				Lat:  telem.Gps[0].Latitude,
				Long: telem.Gps[0].Longitude,
			}
			return newDataFrame(gs, pkt.pts, pkt.Data(), tdata), true
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Printf("Telemetry read err: %v", err)
			return nil, false
		}
	}
	return nil, false
}

func genericFrameHandler(gs *DataStream, pkt *Packet) (Frame, bool) {
	/* Do nothing */
	return nil, false
}

// Close closes the stream and
// stops decoding frames.
func (gs *DataStream) Close() error {
	return nil
}
