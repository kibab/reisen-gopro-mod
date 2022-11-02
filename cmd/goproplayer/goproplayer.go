package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"time"

	"github.com/faiface/beep"
	"github.com/faiface/beep/speaker"
	"github.com/hajimehoshi/ebiten"
	_ "github.com/silbinarywolf/preferdiscretegpu"
	"github.com/zergon321/reisen"
)

const (
	frameBufferSize                   = 1024 * 1024
	sampleRate                        = 44100
	channelCount                      = 2
	bitDepth                          = 8
	sampleBufferSize                  = 32 * channelCount * bitDepth * 1024 * 1000
	SpeakerSampleRate beep.SampleRate = 44100
)

type videoWithSync struct {
	imgData *image.RGBA
	pts     time.Duration
}

// readVideoAndAudio reads video and audio frames
// from the opened media and sends the decoded
// data to che channels to be played.
func readVideoAndAudio(media *reisen.Media) (<-chan videoWithSync, <-chan [2]float64, <-chan *reisen.DataFrame, chan error, error) {
	frameBuffer := make(chan videoWithSync,
		frameBufferSize)
	sampleBuffer := make(chan [2]float64, sampleBufferSize)
	dataFrame := make(chan *reisen.DataFrame, frameBufferSize)
	errs := make(chan error)

	err := media.OpenDecode()

	if err != nil {
		return nil, nil, nil, nil, err
	}

	videoStream := media.VideoStreams()[0]
	err = videoStream.Open()

	if err != nil {
		return nil, nil, nil, nil, err
	}

	audioStream := media.AudioStreams()[0]
	err = audioStream.Open()

	if err != nil {
		return nil, nil, nil, nil, err
	}

	go func() {
		fmt.Println("*** START DECODER ***")
		numPkt := 0
		for {
			packet, gotPacket, err := media.ReadPacket()
			numPkt++
			fmt.Printf("*** READ PACKET %d ***\n", numPkt)

			if err != nil {
				go func(err error) {
					errs <- err
				}(err)
			}

			if !gotPacket {
				break
			}

			switch packet.Type() {
			case reisen.StreamVideo:
				s := media.Streams()[packet.StreamIndex()].(*reisen.VideoStream)
				videoFrame, gotFrame, err := s.ReadVideoFrame()

				if err != nil {
					go func(err error) {
						errs <- err
					}(err)
				}

				if !gotFrame {
					break
				}

				if videoFrame == nil {
					continue
				}

				frameBuffer <- videoWithSync{imgData: videoFrame.Image(), pts: videoFrame.PresentationOffsetOrDie()}

			case reisen.StreamAudio:
				s := media.Streams()[packet.StreamIndex()].(*reisen.AudioStream)
				audioFrame, gotFrame, err := s.ReadAudioFrame()

				if err != nil {
					go func(err error) {
						errs <- err
					}(err)
				}

				if !gotFrame {
					break
				}

				if audioFrame == nil {
					continue
				}

				// Turn the raw byte data into
				// audio samples of type [2]float64.
				reader := bytes.NewReader(audioFrame.Data())

				// See the README.md file for
				// detailed scheme of the sample structure.
				for reader.Len() > 0 {
					sample := [2]float64{0, 0}
					var result float64
					err = binary.Read(reader, binary.LittleEndian, &result)

					if err != nil {
						go func(err error) {
							errs <- err
						}(err)
					}

					sample[0] = result

					err = binary.Read(reader, binary.LittleEndian, &result)

					if err != nil {
						go func(err error) {
							errs <- err
						}(err)
					}

					sample[1] = result
					sampleBuffer <- sample
				}
			case reisen.StreamData:
				s := media.Streams()[packet.StreamIndex()].(*reisen.DataStream)
				gotDataFrame, gotFrame, err := s.ReadFrame()

				if err != nil {
					go func(err error) {
						errs <- err
					}(err)
				}

				if !gotFrame {
					break
				}

				if gotDataFrame == nil {
					continue
				}
				if df, ok := gotDataFrame.(*reisen.DataFrame); !ok {
					fmt.Println("cannot assign data frame")
				} else {
					dataFrame <- df
				}
			}
		}
		fmt.Println("=========== FINISH DECODING DATA ===================")
		videoStream.Close()
		audioStream.Close()
		media.CloseDecode()
		close(frameBuffer)
		close(sampleBuffer)
		close(dataFrame)
		close(errs)
		fmt.Println("=========== QUIT DECODING DATA ===================")
	}()
	return frameBuffer, sampleBuffer, dataFrame, errs, nil
}

// streamSamples creates a new custom streamer for
// playing audio samples provided by the source channel.
//
// See https://github.com/faiface/beep/wiki/Making-own-streamers
// for reference.
func streamSamples(sampleSource <-chan [2]float64) beep.Streamer {
	return beep.StreamerFunc(func(samples [][2]float64) (n int, ok bool) {
		numRead := 0

		for i := 0; i < len(samples); i++ {
			sample, ok := <-sampleSource

			if !ok {
				numRead = i + 1
				break
			}

			samples[i] = sample
			numRead++
		}

		if numRead < len(samples) {
			return numRead, false
		}

		return numRead, true
	})
}

// Game holds all the data
// necessary for playing video.
type Game struct {
	videoSprite            *ebiten.Image
	ticker                 <-chan time.Time
	errs                   <-chan error
	frameBufferWithSync    <-chan videoWithSync
	data                   <-chan *reisen.DataFrame
	fps                    int
	videoTotalFramesPlayed int
	videoPlaybackFPS       int
	perSecond              <-chan time.Time
	last                   time.Time
	deltaTime              float64
	Width                  int
	Height                 int
	lastVideoPts           time.Duration
	lastDataPts            time.Duration
	lastData               *reisen.DataFrame
}

// Strarts reading samples and frames
// of the media file.
func (game *Game) Start(fname string) error {
	// Initialize the audio speaker.
	err := speaker.Init(sampleRate,
		SpeakerSampleRate.N(time.Second/10))

	if err != nil {
		return err
	}

	// Open the media file.
	media, err := reisen.NewMedia(fname)

	if err != nil {
		return err
	}

	videoStream := media.VideoStreams()[0]

	// Get the FPS for playing
	// video frames.
	videoFPS, _ := media.Streams()[0].FrameRate()

	if err != nil {
		return err
	}

	// SPF for frame ticker.
	spf := 1.0 / float64(videoFPS)
	frameDuration, err := time.
		ParseDuration(fmt.Sprintf("%fs", spf))

	if err != nil {
		return err
	}

	// Start decoding streams.
	var sampleSource <-chan [2]float64

	game.frameBufferWithSync, sampleSource, game.data,
		game.errs, err = readVideoAndAudio(media)

	if err != nil {
		return err
	}

	time.Sleep(20 * time.Second)
	fmt.Println("START WITH THE REST OF THE PLAYBACK")
	// Now that decoding has started, we can get width and height of the video stream.
	game.Width = videoStream.Width()
	game.Height = videoStream.Height()

	ebiten.SetWindowSize(game.Width, game.Height)
	// Sprite for drawing video frames.
	game.videoSprite, err = ebiten.NewImage(
		game.Width, game.Height, ebiten.FilterDefault)

	if err != nil {
		return err
	}

	// Start playing audio samples.
	speaker.Play(streamSamples(sampleSource))

	game.ticker = time.Tick(frameDuration)

	// Setup metrics.
	game.last = time.Now()
	game.fps = 0
	game.perSecond = time.Tick(time.Second)
	game.videoTotalFramesPlayed = 0
	game.videoPlaybackFPS = 0

	return nil
}

func (game *Game) Update(screen *ebiten.Image) error {
	// Compute dt.
	game.deltaTime = time.Since(game.last).Seconds()
	game.last = time.Now()

	// Check for incoming errors.
	select {
	case err, ok := <-game.errs:
		if ok {
			return err
		}

	default:
	}

	go func(game *Game) {
		for {
			// Sync to video
			if game.lastDataPts > game.lastVideoPts {
				//fmt.Printf("There is more data in queue, but its time is not up yet\n")
				time.Sleep(time.Second / 2)
			}
			df, ok := <-game.data
			if ok {
				game.lastDataPts = df.PresentationOffsetOrDie()
				game.lastData = df
				fmt.Printf("\n\nhttp://maps.google.com/?ie=UTF8&hq=&ll=%f,%f&z=13\n\n", df.Telemetry().Lat, df.Telemetry().Long)
			} else {
				return
			}
		}
	}(game)

	// Read video frames and draw them.
	select {
	case <-game.ticker:
		frameWithSync, ok := <-game.frameBufferWithSync

		if ok {
			game.videoSprite.ReplacePixels(frameWithSync.imgData.Pix)
			game.lastVideoPts = frameWithSync.pts

			game.videoTotalFramesPlayed++
			game.videoPlaybackFPS++
		}

	default:
	}

	// Draw the video sprite.
	op := &ebiten.DrawImageOptions{}
	err := screen.DrawImage(game.videoSprite, op)

	if err != nil {
		return err
	}

	game.fps++

	// Update metrics in the window title.
	select {
	case <-game.perSecond:
		ebiten.SetWindowTitle(fmt.Sprintf("%s | FPS: %d | dt: %f | Frames: %d | Video FPS: %d",
			"Video", game.fps, game.deltaTime, game.videoTotalFramesPlayed, game.videoPlaybackFPS))

		game.fps = 0
		game.videoPlaybackFPS = 0

	default:
	}

	return nil
}

func (game *Game) Layout(a, b int) (int, int) {
	return game.Width, game.Height
}

func main() {
	game := &Game{}
	err := game.Start("demo.mp4")
	handleError(err)

	ebiten.SetWindowTitle("Video")
	err = ebiten.RunGame(game)
	handleError(err)
}

func handleError(err error) {
	if err != nil {
		panic(err)
	}
}
