package videoutil

import (
	"encoding/json"
	"fmt"
	telebot "github.com/graphomania/tg"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"time"
)

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

func formatDuration(seconds float64) string {
	trailingZeros := func(d time.Duration, zeros int) string {
		num := int64(d)
		s := fmt.Sprintf("%d", num)
		for len(s) < zeros {
			s = "0" + s
		}
		return s
	}

	d := time.Duration(seconds) * time.Second
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

func makeThumbnailAtAlt(tmpDir string, ffmpeg string, filename string, at string) (string, error) {
	tmpBig, err := os.CreateTemp(tmpDir, "*_graphomania_tg_big_preview.jpg")
	if err != nil {
		return "", err
	}
	defer func() {
		_ = tmpBig.Close()
	}()

	output, err := exec.Command(ffmpeg, "-y", "-i", filename,
		"-ss", at,
		"-vframes", "1",
		"-q:v", "1", "-qmin", "1", "-qmax", "1", "-vf", makeScaleRule(320, 320),
		tmpBig.Name()).
		CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%v\n%s", err, string(output))
	}

	return tmpBig.Name(), nil
}

func calcThumbnailPosition(duration float64, position interface{}) string {
	ret := ""
	switch pos := position.(type) {
	case string:
		ret = pos
	case float64:
		ret = formatDuration(duration * pos)
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

func parseVideoModOptions(opts ...*Opt) *Opt {
	options := &Opt{}
	if len(opts) != 0 {
		options = opts[0]
	}
	return options.Defaults()
}

func getSetMetadata(video *telebot.Video, opt *Opt) (meta *fileMetadata, duration float64, err error) {
	if video == nil || video.FileLocal == "" {
		return nil, 0, nil
	}
	meta, err = getFileMetadata(opt.Ffprobe, video.FileLocal)
	if err != nil {
		return
	}
	duration, err = strconv.ParseFloat(meta.Format.Duration, 10)
	if err != nil {
		return
	}
	if len(meta.Streams) > 0 {
		video.Width = meta.Streams[0].Width
		video.Height = meta.Streams[0].Height
		video.Duration = int(duration)
		video.MIME = "video/mp4"
	}
	if stat, err := os.Stat(video.FileLocal); err == nil {
		video.FileSize = stat.Size()
	}

	return
}

func makeScaleRule(width int, height int) string {
	return fmt.Sprintf("scale=if(gte(iw\\,ih)\\,min(%d\\,iw)\\,-2):if(lt(iw\\,ih)\\,min(%d\\,ih)\\,-2)", width, height)
}

func getFunctionName(i interface{}) string {
	return runtime.FuncForPC(reflect.ValueOf(i).Pointer()).Name()
}
