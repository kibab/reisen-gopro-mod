package reisen

import (
	"bytes"
	"fmt"
	"io"

	"github.com/kibab/gopro-utils/telemetry"
)

func DecodeGoproData(data []byte) error {
	r := bytes.NewReader(data)
	for {
		telem, err := telemetry.Read(r)
		if telem != nil && telem.Gps != nil {
			//fmt.Printf("Telemetry pkt: %+v\n", telem.Gps)
			fmt.Printf("\n\nhttp://maps.google.com/?ie=UTF8&hq=&ll=%f,%f&z=13\n\n", telem.Gps[0].Latitude, telem.Gps[0].Longitude)
		}
		if err == io.EOF {
			return nil
		}
		if err != nil {
			fmt.Printf("Telemetry read err: %v", err)
			return err
		}
	}
	return nil
}
