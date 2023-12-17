package telebot

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"time"
)

const (
	ffmpeg  = "ffmpeg"
	ffprobe = "ffprobe"
	convert = "convert"
)

// ThumbnailAt creates a thumbnail at a position of 2 types:
// 1. float64 -- from [0, 1], relative position in Video
// 2. string  -- position in ffmpeg format, i.e. 00:05:12.99
func ThumbnailAt(position interface{}, opts ...interface{}) ThumbnailBuilder {
	options := &ThumbnailOptions{}
	if len(opts) != 0 {
		options = opts[0].(*ThumbnailOptions)
	}
	options = options.Defaults()

	return func(video *Video) (filename string, err error) {
		if video == nil || video.FileLocal == "" {
			return "", nil
		}

		metadata, err := getFileMetadata(options.Ffprobe, video.FileLocal)
		if err != nil {
			return "", err
		}

		videoDuration, err := strconv.ParseFloat(metadata.Format.Duration, 10)
		if err != nil {
			return "", err
		}

		return makePreviewAt(options.TmpDir, options.Convert, options.Ffmpeg, video.FileLocal, calcThumbnailPosition(videoDuration, position))
	}
}

type ThumbnailOptions struct {
	Convert string
	Ffmpeg  string
	Ffprobe string
	TmpDir  string
}

func (opts *ThumbnailOptions) Defaults() *ThumbnailOptions {
	if opts == nil {
		opts = &ThumbnailOptions{}
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
	return opts
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

func formatPreview(tmpDir string, convert string, filename string) (string, error) {
	tempFile, err := os.CreateTemp(tmpDir, "*_graphomania_tg_small_preview.jpg")
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

func makePreviewAt(tmpDir string, convert string, ffmpeg string, filename string, at string) (string, error) {
	tmpBig, err := os.CreateTemp(tmpDir, "*_graphomania_tg_big_preview.jpg")
	if err != nil {
		return "", err
	}
	defer func() {
		_ = tmpBig.Close()
		_ = os.Remove(tmpBig.Name())
	}()

	output, err := exec.Command(ffmpeg, "-y", "-i", filename, "-ss", at, "-vframes", "1", tmpBig.Name()).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%v\n%s", err, string(output))
	}

	return formatPreview(tmpDir, convert, tmpBig.Name())
}

func calcThumbnailPosition(duration float64, position interface{}) string {
	ret := ""
	switch pos := position.(type) {
	case string:
		ret = pos
	case float64:
		ret = formatDuration(time.Duration(duration * pos))
	}
	return ret
}
