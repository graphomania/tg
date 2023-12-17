package telebot

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	ffmpeg  = "ffmpeg"
	ffprobe = "ffprobe"
	convert = "convert"
	preset  = "fast"
)

// ThumbnailAt creates a thumbnail at a position of 2 types:
// 1. float64 -- from [0, 1], relative position in Video
// 2. string  -- position in ffmpeg format, i.e. 00:05:12.99
func ThumbnailAt(position interface{}, opts ...*VideoModOpt) VideoModifier {
	options := parseVideoModOptions(opts...)

	return func(video *Video) (filename []string, err error) {
		if video == nil || video.FileLocal == "" {
			return nil, nil
		}

		metadata, err := getFileMetadata(options.Ffprobe, video.FileLocal)
		if err != nil {
			return nil, err
		}

		videoDuration, err := strconv.ParseFloat(metadata.Format.Duration, 10)
		if err != nil {
			return nil, err
		}

		thumbnail, err := makeThumbnailAt(options.TmpDir, options.Convert, options.Ffmpeg, video.FileLocal, calcThumbnailPosition(videoDuration, position))
		video.Thumbnail = &Photo{File: FromDisk(thumbnail)}
		return []string{thumbnail}, err
	}
}

// MuteVideo by creating a local muted copy.
// https://superuser.com/questions/268985/remove-audio-from-video-file-with-ffmpeg
// ffmpeg -i $input_file -vcodec copy -an $output_file
func MuteVideo(opts ...*VideoModOpt) VideoModifier {
	options := parseVideoModOptions(opts...)

	return func(video *Video) (temporaries []string, err error) {
		if video == nil || video.FileLocal == "" {
			return nil, nil
		}

		tmpFile, err := os.CreateTemp(options.TmpDir, fmt.Sprintf("*_graphomania_tg%s", filetype(video.FileLocal)))
		if err != nil {
			return nil, err
		}

		output, err := exec.Command(options.Ffmpeg, "-y", "-i", video.FileLocal, "-vcodec", "copy", "-an", tmpFile.Name()).
			CombinedOutput()
		if err != nil {
			return nil, wrapExecError(err, output)
		}

		video.FileLocal = tmpFile.Name()

		return []string{tmpFile.Name()}, nil
	}
}

// VideoMod is a general purpose VideoModifier. Accepts (*VideoModOpt) as argument.
func VideoMod(opts ...*VideoModOpt) VideoModifier {
	options := parseVideoModOptions(opts...)

	// https://stackoverflow.com/questions/54063902/resize-videos-with-ffmpeg-keep-aspect-ratio
	scaleRule := fmt.Sprintf("scale=if(gte(iw\\,ih)\\,min(%d\\,iw)\\,-2):if(lt(iw\\,ih)\\,min(%d\\,ih)\\,-2)", options.Width, options.Height)

	return func(video *Video) (temporaries []string, err error) {
		if video == nil || video.FileLocal == "" {
			return nil, nil
		}

		tmpFile, err := os.CreateTemp(options.TmpDir, fmt.Sprintf("*_graphomania_tg%s", filetype(video.FileLocal)))
		if err != nil {
			return nil, err
		}

		output, err := exec.Command(options.Ffmpeg,
			"-y", "-i", video.FileLocal,
			"-vf", scaleRule,
			"-vcodec", "libx264",
			"-acodec", "aac",
			"-preset", options.Preset,
			tmpFile.Name()).
			CombinedOutput()
		if err != nil {
			return []string{tmpFile.Name()}, wrapExecError(err, output)
		}

		video.FileLocal = tmpFile.Name()
		metadata, err := getFileMetadata(options.Ffprobe, video.FileLocal)
		if err != nil {
			return []string{tmpFile.Name()}, wrapExecError(err, output)
		}

		video.Width = metadata.Streams[0].Width
		video.Height = metadata.Streams[0].Height

		return []string{tmpFile.Name()}, nil
	}
}

type VideoModOpt struct {
	Width   int
	Height  int
	Preset  string
	Ffmpeg  string
	Ffprobe string
	Convert string
	TmpDir  string
}

func (opts *VideoModOpt) Defaults() *VideoModOpt {
	if opts == nil {
		opts = &VideoModOpt{}
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
	if opts.Width <= 0 {
		opts.Width = 5000
	}
	if opts.Height <= 0 {
		opts.Height = 5000
	}
	if opts.Preset == "" {
		opts.Preset = preset
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

func makeThumbnailAt(tmpDir string, convert string, ffmpeg string, filename string, at string) (string, error) {
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

func filetype(filename string) string {
	index := strings.LastIndexByte(filename, '.')
	if index < 0 {
		return filepath.Base(filename)
	}
	return filename[index:]
}

func wrapExecError(err error, output []byte) error {
	if err == nil || len(output) == 0 {
		return err
	}
	return fmt.Errorf("err: %s\nout: %s", err.Error(), string(output))
}

func parseVideoModOptions(opts ...*VideoModOpt) *VideoModOpt {
	options := &VideoModOpt{}
	if len(opts) != 0 {
		options = opts[0]
	}
	return options.Defaults()
}
