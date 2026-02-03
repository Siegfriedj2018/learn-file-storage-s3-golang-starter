package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
)

// // GetGCD calculates the greatest common divisor using the Euclidean algorithm
// func getGCD(a, b int) int {
// 	for b != 0 {
// 		a, b = b, a%b
// 	}
// 	return a
// }

// IdentifyAspectRatio takes dimensions and returns a string representation
func IdentifyAspectRatio(width, height int) (string, error) {
	if width <= 0 || height <= 0 {
		return "", fmt.Errorf("Invalid ratio")
	}

	if width == 16*height/9 {
    return "16:9", nil
	} else if height == 16*width/9 {
		return "9:16", nil
	}

	return "other", nil
}

func getvideoAspectRatio(filepath string) (string, error) {
	type Stream struct {
		Width  int `json:"width"`
		Height int `json:"height"`
	}
	type FFProbeOutput struct {
		Streams []Stream `json:"streams"`
	}


	videoCmd := exec.Command("ffprobe", "-v", "error", "-print_format", "json", "-show_streams", filepath)
	var bytesBuff bytes.Buffer
	videoCmd.Stdout = &bytesBuff
	videoCmd.Run()

	var videoAsp *FFProbeOutput 
	err := json.Unmarshal(bytesBuff.Bytes(), &videoAsp)
	if err != nil {
		return "", fmt.Errorf("Error unmarshalling, %v", err)
	}

	calculatedAspect, err := IdentifyAspectRatio(videoAsp.Streams[0].Width, videoAsp.Streams[0].Height)
	if err != nil {
		return "", fmt.Errorf("Error calculating aspect ratio, %w", err)
	}

	return calculatedAspect, nil
}