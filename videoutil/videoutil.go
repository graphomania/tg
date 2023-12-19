package videoutil

const (
	ffmpeg  = "ffmpeg"
	ffprobe = "ffprobe"
	convert = "convert"
	preset  = "fast"
)

type Option struct {
	Width   int
	Height  int
	Preset  string
	Ffmpeg  string
	Ffprobe string
	Convert string
	TmpDir  string
}

func (opts *Option) Defaults() *Option {
	if opts == nil {
		opts = &Option{}
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
