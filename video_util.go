package telebot

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// SendLocalVideo has extra steps compared to Bot.Send(..., Video),
// 1. Setting the preview (by a picture or a timestamp of the video ([0, 1]))
// 2. Ensuring the video is being sent correctly
// Unique option -- SendVideoOpts.
func (b *Bot) SendLocalVideo(to Recipient, filename string, opts ...interface{}) (*Message, error) {
	telebotOpts, sendVideoOpts := sendLocalVideoParseOpts(opts)
	video, tmpFile, err := VideoWithPreviewVerbose(sendVideoOpts.Convert, sendVideoOpts.Ffmpeg, sendVideoOpts.Ffprobe, filename, sendVideoOpts.Preview)
	defer os.Remove(tmpFile)
	if err != nil {
		return nil, wrapError(err)
	}
	video.Caption = sendVideoOpts.Caption
	video.FileName = sendVideoOpts.Filename
	return b.SendWithConnectionRetries(to, video, sendVideoOpts.Retries, telebotOpts...)
}

const (
	ffmpeg       = "ffmpeg"
	ffprobe      = "ffprobe"
	convert      = "convert"
	temporaryDir = "testdata"
)

type SendVideoOpts struct {
	// string -- path to file, Photo -- Telegram Cloud picture, float64 -- [0, 1] part of video
	Preview interface{}
	// (Retries < 0)  --> (no retries)
	// (Retries == 0) --> (Default (3) retries)
	Retries int

	Caption  string
	Filename string

	Convert string
	Ffmpeg  string
	Ffprobe string
}

func (opts *SendVideoOpts) Defaults() *SendVideoOpts {
	if opts == nil {
		opts = &SendVideoOpts{}
	}
	if opts.Convert == "" {
		opts.Convert = convert
	}
	if opts.Ffmpeg == "" {
		opts.Ffmpeg = ffmpeg
	}
	if opts.Ffprobe == "" {
		opts.Ffprobe = ffprobe
	}
	if opts.Preview == nil {
		opts.Preview = 0.25
	}
	if opts.Retries == 0 {
		opts.Retries = 3
	} else if opts.Retries < 0 {
		opts.Retries = 0
	}
	return opts
}

func VideoWithPreview(filename string, preview interface{}, tgFilename ...string) (vid *Video, tmpfile string, err error) {
	return VideoWithPreviewVerbose(convert, ffmpeg, ffprobe, filename, preview, tgFilename...)
}

func VideoWithPreviewVerbose(convert, ffmpeg, ffprobe, filename string, preview interface{}, tgFilename ...string) (*Video, string, error) {
	metadata, err := getFileMetadata(ffprobe, filename)
	if err != nil {
		return nil, "", err
	}
	videoDuration, err := strconv.ParseFloat(metadata.Format.Duration, 10)
	if err != nil {
		return nil, "", err
	}

	var previewPicture *Photo
	var extraFile string
	switch prev := preview.(type) {
	case *Photo:
		previewPicture = prev

	case string:
		previewPic, err := formatPreview(convert, filename)
		if err != nil {
			return nil, previewPic, err
		}
		extraFile = previewPic
		previewPicture = &Photo{File: FromDisk(previewPic)}

	case float64:
		previewPath, err := makePreviewAt(convert, ffmpeg, filename, videoDuration*prev)
		if err != nil {
			return nil, previewPath, err
		}
		extraFile = previewPath
		previewPicture = &Photo{File: FromDisk(previewPath)}

	default:
		return nil, "", errors.New("unknown argument type: <preview>")
	}

	return &Video{
		File:      FromDisk(filename),
		Width:     metadata.Streams[0].Width,
		Height:    metadata.Streams[0].Height,
		Duration:  int(videoDuration),
		Thumbnail: previewPicture,
		Streaming: true,
		MIME:      "video/mp4",
		FileName:  strings.Join(tgFilename, ""),
	}, extraFile, nil
}

type fileMetadata struct {
	Streams []struct {
		Width  int `json:"width"`
		Height int `json:"height"`
	} `json:"streams"`
	Format struct {
		Filename string `json:"filename"`
		Duration string `json:"duration"`
	} `json:"format"`
}

func getFileMetadata(ffprobe, filename string) (*fileMetadata, error) {
	output, err := exec.Command(ffprobe, "-v", "error", "-select_streams", "v:0", "-show_entries", "stream=width,height",
		"-of", "json", "-show_format", filename).Output()
	if err != nil {
		return nil, fmt.Errorf("%v\n%s", err, string(output))
	}

	var metadata fileMetadata
	err = json.Unmarshal(output, &metadata)
	if err != nil {
		return nil, fmt.Errorf("%v\n%s", err, string(output))
	}

	return &metadata, nil
}

func formatDuration(d time.Duration) string {
	trailingZeros := func(d time.Duration, zeros int) string {
		num := int64(d)
		s := fmt.Sprintf("%d", num)
		for len(s) < zeros {
			s = "0" + s
		}
		return s
	}

	return fmt.Sprintf("%s:%s:%s.%s",
		trailingZeros(d/time.Hour%24, 2), trailingZeros(d/time.Minute%60, 2),
		trailingZeros(d/time.Second%60, 2), trailingZeros(d/time.Millisecond%1000, 3))
}

func formatPreview(convert string, filename string) (string, error) {
	tempFile, err := os.CreateTemp(temporaryDir, "*_graphomania_tg_small_preview.jpg")
	if err != nil {
		return "", err
	}
	defer tempFile.Close()

	output, err := exec.Command(convert, filename, "-resize", "320x320", "-quality", "87", tempFile.Name()).CombinedOutput()
	if err != nil {
		return tempFile.Name(), fmt.Errorf("%v\n%s", err, string(output))
	}

	return tempFile.Name(), nil
}

func makePreviewAt(convert string, ffmpeg string, filename string, at float64) (string, error) {
	tmpBig, err := os.CreateTemp(temporaryDir, "*_graphomania_tg_big_preview.jpg")
	if err != nil {
		return "", err
	}
	defer func() {
		_ = tmpBig.Close()
		_ = os.Remove(tmpBig.Name())
	}()

	output, err := exec.Command(ffmpeg, "-y", "-i", filename,
		"-ss", formatDuration(time.Duration(at)), "-vframes", "1", tmpBig.Name()).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%v\n%s", err, string(output))
	}

	return formatPreview(convert, tmpBig.Name())
}

func sendLocalVideoParseOpts(opts []interface{}) ([]interface{}, *SendVideoOpts) {
	sendVideoOpts := &SendVideoOpts{}
	telebotOpts := make([]interface{}, 0)
	for _, opt := range opts {
		switch val := opt.(type) {
		case *SendVideoOpts:
			sendVideoOpts = val

		default:
			telebotOpts = append(telebotOpts, opt)
		}
	}

	return telebotOpts, sendVideoOpts.Defaults()
}
