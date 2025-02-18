package photoutil

import (
	"fmt"
	telebot "github.com/graphomania/tg"
	"os"
	"os/exec"
)

const (
	convert = "convert"
)

type Opt struct {
	Width   int
	Height  int
	Convert string
	TmpPath string
	Quality int
}

func Converted(opts ...*Opt) telebot.PhotoModifier {
	opt := parseOpts(opts...)
	resizeArg := fmt.Sprintf("%dx%d>", opt.Width, opt.Height)
	return func(photo *telebot.Photo) (temporaries []string, err error) {
		tmp, err := os.CreateTemp(opt.TmpPath, "*.jpg")
		if err != nil {
			return nil, err
		}
		_ = tmp.Close()

		ret := []string{tmp.Name()}
		output, err := exec.Command(opt.Convert, photo.File.FileLocal, "-resize", resizeArg, tmp.Name()).CombinedOutput()
		if err != nil {
			return ret, wrapExecError(err, output)
		}

		photo.FileLocal = tmp.Name()

		return ret, nil
	}
}

func RemoveAfter(opts ...*Opt) telebot.PhotoModifier {
	return func(photo *telebot.Photo) (temporaries []string, err error) {
		return []string{photo.FileLocal}, nil
	}
}

func parseOpts(opts ...*Opt) Opt {
	opt := Opt{
		Convert: convert,
		TmpPath: "",
		Quality: 95,
	}
	if len(opts) == 0 {
		return opt
	}
	opts_ := opts[0]

	if opts_.Width == 0 {
		opt.Width = 5000
	} else {
		opt.Width = opts_.Width
	}
	if opts_.Height == 0 {
		opt.Height = 5000
	} else {
		opt.Height = opts_.Height
	}
	if opts_.Convert != "" {
		opt.Convert = opts_.Convert
	}
	if opts_.TmpPath != "" {
		opt.TmpPath = opts_.TmpPath
	}
	if opts_.Quality != 0 {
		opt.Quality = opts_.Quality
	}
	return opt
}

func wrapExecError(err error, output []byte) error {
	if err == nil || len(output) == 0 {
		return err
	}
	return fmt.Errorf("err: %s\nout: %s", err.Error(), string(output))
}
