package videoutil

import (
	"fmt"
	telebot "github.com/graphomania/tg"
	"log"
	"os"
	"os/exec"
	"time"
)

// ExtraFormats tries to perform `ffmpeg -i $FILENAME -vcodec copy -acodec copy return.mp4`.
// To ensure conversion to .H264 use with Converted().
func ExtraFormats(opts ...*Opt) telebot.VideoModifier {
	options := parseVideoModOptions(opts...)

	return func(video *telebot.Video) (temporaries []string, err error) {
		if video == nil || video.FileLocal == "" {
			return nil, nil
		}

		tmpFile, err := os.CreateTemp(options.TmpDir, "*.mp4")
		if err != nil {
			return nil, err
		}

		output, err := exec.Command(options.Ffmpeg, "-y",
			"-i", video.FileLocal,
			"-vcodec", "copy",
			"-acodec", "copy",
			tmpFile.Name()).
			CombinedOutput()
		if err != nil {
			return []string{tmpFile.Name()}, wrapExecError(err, output)
		}

		video.FileLocal = tmpFile.Name()

		return []string{tmpFile.Name()}, nil
	}
}

// WithMetadata ensures, Telegram would process a file correctly.
// REQUIRES `ffprobe` on the system, which could be passed via Opt.Convert
func WithMetadata(opts ...*Opt) telebot.VideoModifier {
	options := parseVideoModOptions(opts...)

	return func(video *telebot.Video) (temporaries []string, err error) {
		_, _, err = getSetMetadata(video, options)
		return nil, err
	}
}

// ThumbnailFrom converts a picture to a 320x320 frame, suitable from Telegram video thumbnail.
// REQUIRES `convert` on the system, could be passed via Opt.Convert.
func ThumbnailFrom(filename string, opts ...*Opt) telebot.VideoModifier {
	options := parseVideoModOptions(opts...)

	return func(video *telebot.Video) (temporaries []string, err error) {
		extraFile, err := formatPreview(options.TmpDir, options.Convert, filename)
		if err != nil {
			return []string{extraFile}, err
		}
		video.Thumbnail = &telebot.Photo{File: telebot.FromDisk(extraFile)}
		return []string{extraFile}, nil
	}
}

// ThumbnailAt creates a thumbnail from the video frame. Position could be chosen as:
//  1. float64 -- from [0, 1], relative position in Video
//  2. string  -- position in ffmpeg format, i.e. "00:05:12.99"
//
// REQUIRES `ffmpeg`, `ffprobe` on the system, could be passed via Opt.Convert.
func ThumbnailAt(position interface{}, opts ...*Opt) telebot.VideoModifier {
	switch position.(type) {
	case float64:
	case string:
	default:
		panic("ThumbnailAt: position type is not supported")
	}

	options := parseVideoModOptions(opts...)

	return func(video *telebot.Video) (filename []string, err error) {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("ThumbnailAt panicked with %v", err)
			}
		}()
		if video == nil || video.FileLocal == "" {
			return nil, nil
		}

		_, videoDuration, err := getSetMetadata(video, options)

		thumbnail, err := makeThumbnailAtAlt(options.TmpDir, options.Ffmpeg, video.FileLocal, calcThumbnailPosition(videoDuration, position))
		if err != nil {
			return []string{thumbnail}, err
		}
		video.Thumbnail = &telebot.Photo{File: telebot.FromDisk(thumbnail)}

		return []string{thumbnail}, err
	}
}

// Muted a video by creating a local muted copy.
// REQUIRES `ffmpeg` on the system, could be passed via Opt.Convert.
func Muted(opts ...*Opt) telebot.VideoModifier {
	options := parseVideoModOptions(opts...)

	return func(video *telebot.Video) (temporaries []string, err error) {
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

// Converted is a general purpose VideoModifier, converts a video to h264, could decrease its dimensions.
// REQUIRES `ffmpeg` on the system, which could be passed via Opt.Convert.
func Converted(opts ...*Opt) telebot.VideoModifier {
	options := parseVideoModOptions(opts...)

	// https://stackoverflow.com/questions/54063902/resize-videos-with-ffmpeg-keep-aspect-ratio
	scaleRule := makeScaleRule(options.Width, options.Height)
	return func(video *telebot.Video) (temporaries []string, err error) {
		if video == nil || video.FileLocal == "" {
			return nil, nil
		}

		tmpFile, err := os.CreateTemp(options.TmpDir, fmt.Sprintf("*_graphomania_tg%s", filetype(video.FileLocal)))
		if err != nil {
			return nil, err
		}

		output, err := exec.Command(options.Ffmpeg, "-y",
			"-i", video.FileLocal,
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

		return []string{tmpFile.Name()}, nil
	}
}

// Timed logs time each telebot.VideoModifier to complete the task. Could be useful for testing.
func Timed(mods ...telebot.VideoModifier) telebot.VideoModifier {
	return func(video *telebot.Video) (temporaries []string, err error) {
		for _, mod := range mods {
			start := time.Now()
			temp, err := mod(video)
			log.Printf("%s for %s took %s", getFunctionName(mod), video.FileName, time.Since(start).String())
			temporaries = append(temporaries, temp...)
			if err != nil {
				return temporaries, err
			}
		}
		return
	}
}

// Join multiple telebot.VideoModifier into a one. I.e., for measuring a bunch of them.
func Join(mods ...telebot.VideoModifier) telebot.VideoModifier {
	return func(video *telebot.Video) (temporaries []string, err error) {
		for _, mod := range mods {
			temp, err := mod(video)
			temporaries = append(temporaries, temp...)
			if err != nil {
				return temporaries, err
			}
		}
		return
	}
}

func OnError(mod telebot.VideoModifier, fn func(temporaries []string, err error)) telebot.VideoModifier {
	return func(video *telebot.Video) (temporaries []string, err error) {
		ret_, err_ := mod(video)
		if err_ != nil {
			fn(ret_, err_)
		}
		return ret_, err_
	}
}

// IgnoreErr mutes any error the given telebot.VideoModifier produces.
func IgnoreErr(mod telebot.VideoModifier) telebot.VideoModifier {
	return func(video *telebot.Video) (temporaries []string, err error) {
		ret_, _ := mod(video)
		return ret_, nil
	}
}

func RemoveAfter(opts ...Opt) telebot.VideoModifier {
	return func(video *telebot.Video) (temporaries []string, err error) {
		return []string{video.FileLocal}, nil
	}
}
