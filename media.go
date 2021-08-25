package reisen

// #cgo LDFLAGS: -lavformat -lavcodec -lavutil -lswscale
// #include <libavcodec/avcodec.h>
// #include <libavformat/avformat.h>
// #include <libavutil/avconfig.h>
// #include <libswscale/swscale.h>
import "C"
import (
	"fmt"
	"time"
	"unsafe"
)

type Media struct {
	ctx     *C.AVFormatContext
	streams []Stream
}

func (media *Media) StreamCount() int {
	return int(media.ctx.nb_streams)
}

func (media *Media) Streams() []Stream {
	streams := make([]Stream, len(media.streams))
	copy(streams, media.streams)

	return streams
}

func (media *Media) Duration() (time.Duration, error) {
	dur := media.ctx.duration
	tm := float64(dur) / float64(TimeBase)

	return time.ParseDuration(fmt.Sprintf("%fs", tm))
}

func (media *Media) FormatName() string {
	if media.ctx.iformat.name == nil {
		return ""
	}

	return C.GoString(media.ctx.iformat.name)
}

func (media *Media) FormatLongName() string {
	if media.ctx.iformat.long_name == nil {
		return ""
	}

	return C.GoString(media.ctx.iformat.long_name)
}

func (media *Media) FormatMIMEType() string {
	if media.ctx.iformat.mime_type == nil {
		return ""
	}

	return C.GoString(media.ctx.iformat.mime_type)
}

func (media *Media) findStreams() error {
	streams := []Stream{}
	innerStreams := unsafe.Slice(
		media.ctx.streams, media.ctx.nb_streams)
	status := C.avformat_find_stream_info(media.ctx, nil)

	if status < 0 {
		return fmt.Errorf(
			"couldn't find stream information")
	}

	for _, innerStream := range innerStreams {
		codecParams := innerStream.codecpar
		codec := C.avcodec_find_decoder(codecParams.codec_id)

		if codec == nil {
			return fmt.Errorf(
				"couldn't find codec by ID = %d",
				codecParams.codec_id)
		}

		switch codecParams.codec_type {
		case C.AVMEDIA_TYPE_VIDEO:
			videoStream := new(VideoStream)
			videoStream.inner = innerStream
			videoStream.codecParams = codecParams
			videoStream.codec = codec

			streams = append(streams, videoStream)

		case C.AVMEDIA_TYPE_AUDIO:
			audioStream := new(AudioStream)
			audioStream.inner = innerStream
			audioStream.codecParams = codecParams
			audioStream.codec = codec

			streams = append(streams, audioStream)

		default:
			return fmt.Errorf("unknown stream type")
		}
	}

	media.streams = streams

	return nil
}

func (media *Media) Close() {
	C.avformat_free_context(media.ctx)
}

func NewMedia(filename string) (*Media, error) {
	media := &Media{
		ctx: C.avformat_alloc_context(),
	}

	if media.ctx == nil {
		return nil, fmt.Errorf(
			"couldn't create a new media context")
	}

	fname := C.CString(filename)
	status := C.avformat_open_input(&media.ctx, fname, nil, nil)

	if status < 0 {
		return nil, fmt.Errorf(
			"couldn't open file %s", filename)
	}

	C.free(unsafe.Pointer(fname))
	err := media.findStreams()

	if err != nil {
		return nil, err
	}

	return media, nil
}
