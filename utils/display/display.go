package display

import (
	"errors"
	"fmt"
	"github.com/hhkbp2/go-logging"
	"github.com/isbm/textwrap"
	"github.com/mattn/go-runewidth"
	"github.com/scylladb/gosible/config"
	"github.com/scylladb/gosible/utils/callbacks"
	"os"
	"strings"
	"syscall"
	"time"
)

// Options struct provides default values for function parameters and a way to customize message display.
type Options struct {
	Color      string
	Stderr     bool
	ScreenOnly bool
	LogOnly    bool
	NoNewline  bool
}
type WarnOptions struct {
	Formatted bool
}
type BannerOptions struct {
	Color string
}
type ErrorOptions struct {
	DontWrapText bool
}

var colorToLogLevelMap map[string]logging.LogLevelType = nil

func init() {
	// Register a callback to set up a logger once the config has been loaded (so that the log path is known).
	callbacks.ScheduleCallback(callbacks.ConfigLoaded, true, callbacks.Callback{Fn: setupDefaultLogger, Name: "setup default logger", Registerer: "display"})
	callbacks.ScheduleCallback(callbacks.ConfigLoaded, true, callbacks.Callback{Fn: createColorToLogLevelMap, Name: "create color -> log level map", Registerer: "display"})
}

func createColorToLogLevelMap() {
	cfg := config.Manager().Settings
	colorToLogLevelMap = map[string]logging.LogLevelType{
		cfg.COLOR_OK:          logging.LevelInfo,
		cfg.COLOR_ERROR:       logging.LevelError,
		cfg.COLOR_WARN:        logging.LevelWarning,
		cfg.COLOR_SKIP:        logging.LevelWarning,
		cfg.COLOR_UNREACHABLE: logging.LevelError,
		cfg.COLOR_DEBUG:       logging.LevelDebug,
		cfg.COLOR_CHANGED:     logging.LevelInfo,
		cfg.COLOR_VERBOSE:     logging.LevelInfo,
	}
}

func getLogLevel(color string) logging.LogLevelType {
	if colorToLogLevelMap == nil {
		createColorToLogLevelMap()
	}

	if level, ok := colorToLogLevelMap[color]; ok {
		return level
	}
	return logging.LevelInfo
}

var logger logging.Logger = nil

func getFormatter() logging.Formatter {
	// TODO Format string should be "%(asctime)s p=%(process)d u=%(user)s n=%(name)s | %(message)s"
	// but the logging library does not support %(process)d and %(user)d.
	return logging.NewStandardFormatter("%(asctime)s n=%(name)s | %(message)s", "%Y-%m-%d %H:%M:%S.%3n")
}

func initFileLogger(logFile string, formatter logging.Formatter) logging.Handler {
	// TODO these values should _probably_ be configurable -- are they configurable in ansible?
	const (
		// Defaults from https://github.com/snower/slock/blob/master/server/config.go
		logRotatingSize    = 67108864 // 64 mb
		logBackupCount     = 5
		logBufferSize      = 0
		logBufferFlushTime = 1
	)
	handler := logging.MustNewRotatingFileHandler(
		logFile, os.O_APPEND, logBufferSize, time.Duration(logBufferFlushTime)*time.Second, 64,
		logRotatingSize, logBackupCount)

	handler.SetFormatter(formatter)
	return handler
}

func setupDefaultLogger() {
	pathSetting := config.Manager().Settings.DEFAULT_LOG_PATH
	if pathSetting == "" {
		Instance().VV(nil, "No default log path specified. Logging to file will be disabled.")
		return
	}

	// TODO gracefully handle situation when path is not writable, is not a file etc

	// TODO how to properly refer to the root handler logger - "" or "root"?
	logger = logging.GetLogger("")
	handler := initFileLogger(pathSetting, getFormatter())
	_ = handler.SetLevel(logging.LevelInfo)
	_ = logger.SetLevel(logging.LevelInfo)
	logger.AddHandler(handler)

	logger = logging.GetLogger("gosible")
	for _, _ = range logging.GetLogger("root").GetHandlers() {
		// TODO FilterBlackList
		// TODO FilterUserInjector
	}
}

const DefaultVerbosityLevel = 2
const MaxVerbosityLevel = 5

var displaySingleton *display = nil

type display struct {
	// TODO deduplicate warnings & errors, like ansible does
	// TODO add methods for input handling
	columns   int
	verbosity int
}

func Instance() *display {
	if displaySingleton != nil {
		return displaySingleton
	}

	displaySingleton = &display{}
	displaySingleton.setColumnWidth()
	err := displaySingleton.SetVerbosity(MaxVerbosityLevel) // TODO is this default value right?
	if err != nil {
		panic(err)
	}

	return displaySingleton
}

func (d *display) SetVerbosity(verbosity int) error {
	if verbosity < 0 || verbosity > 5 {
		return fmt.Errorf("invalid verbosity level %d", verbosity)
	}

	d.verbosity = verbosity
	return nil
}

func (d *display) GetVerbosity() int {
	return d.verbosity
}

func (d *display) setColumnWidth() {
	d.columns = 79
	// TODO
	//    def _set_column_width(self):
	//        if os.isatty(1):
	//            tty_size = unpack('HHHH', fcntl.ioctl(1, TIOCGWINSZ, pack('HHHH', 0, 0, 0, 0)))[1]
	//        else:
	//            tty_size = 0
	//        self.columns = max(79, tty_size - 1)

}

func (d *display) Display(options Options, msg string, a ...interface{}) error {
	msg = fmt.Sprintf(msg, a...)

	if !options.LogOnly {
		hasNewline := msg[len(msg)-1] == '\n'
		var msg2 string
		if hasNewline {
			msg2 = msg[:len(msg)-1]
		} else {
			msg2 = msg
		}

		if options.Color != "" {
			msg2 = stringc(msg2, options.Color, false)
		}
		if hasNewline || !options.NoNewline {
			msg2 += "\n"
		}

		// TODO ansible converts string->bytes->string to get rid of characters that are invalid in the user's locale
		// do we need to do that too?

		var err error
		if options.Stderr {
			_, err = fmt.Fprint(os.Stderr, msg2)
		} else {
			_, err = fmt.Fprint(os.Stdout, msg2)
		}
		if err != nil && !errors.Is(err, syscall.EPIPE) {
			// Ignore EPIPE in case file object has been prematurely closed, eg.
			// when piping to "head -n1"
			// TODO is it the right way to check for EPIPE?
			return err
		}
	}

	if logger != nil && !options.ScreenOnly {
		// TODO ansible converts to bytes to get rid of characters invalid in user's locale

		// Set logger level based on Color - that's what ansible does XD
		logger.Log(getLogLevel(options.Color), msg)
	}

	return nil
}

func Display(options Options, msg string, a ...interface{}) {
	_ = Instance().Display(options, msg, a...)
}

func (d *display) V(host *string, msg string, a ...interface{}) error {
	return d.Verbose(host, 0, msg, a...)
}

func V(host *string, msg string, a ...interface{}) {
	_ = Instance().V(host, msg, a...)
}

func (d *display) VV(host *string, msg string, a ...interface{}) error {
	return d.Verbose(host, 0, msg, a...)
}

func VV(host *string, msg string, a ...interface{}) {
	_ = Instance().VV(host, msg, a...)
}

func (d *display) VVV(host *string, msg string, a ...interface{}) error {
	return d.Verbose(host, 0, msg, a...)
}

func VVV(host *string, msg string, a ...interface{}) {
	_ = Instance().VVV(host, msg, a...)
}

func (d *display) VVVV(host *string, msg string, a ...interface{}) error {
	return d.Verbose(host, 0, msg, a...)
}

func VVVV(host *string, msg string, a ...interface{}) {
	_ = Instance().VVVV(host, msg, a...)
}

func (d *display) VVVVV(host *string, msg string, a ...interface{}) error {
	return d.Verbose(host, 0, msg, a...)
}

func VVVVV(host *string, msg string, a ...interface{}) {
	_ = Instance().VVVVV(host, msg, a...)
}

func (d *display) VVVVVV(host *string, msg string, a ...interface{}) error {
	return d.Verbose(host, 0, msg, a...)
}

func VVVVVV(host *string, msg string, a ...interface{}) {
	_ = Instance().VVVVVV(host, msg, a...)
}

func (d *display) Debug(host *string, msg string, a ...interface{}) error {
	if !config.Manager().Settings.DEFAULT_DEBUG {
		return nil
	}

	// We want now to be the current time in seconds as a float (with fraction part).
	pid, now := os.Getpid(), float64(time.Now().UnixNano())/1e9
	var formatted string
	if host == nil {
		formatted = fmt.Sprintf("%6d %0.5f: %s", pid, now, msg)
	} else {
		formatted = fmt.Sprintf("%6d %0.5f [%s]: %s", pid, now, *host, msg)
	}
	return d.Display(Options{Color: config.Manager().Settings.COLOR_DEBUG}, formatted, a...)
}

func Debug(host *string, msg string, a ...interface{}) {
	_ = Instance().Debug(host, msg, a...)
}

func (d *display) Verbose(host *string, caplevel int, msg string, a ...interface{}) error {
	if d.verbosity > caplevel {
		toStderr := config.Manager().Settings.VERBOSE_TO_STDERR
		if host != nil {
			msg = fmt.Sprintf("<%s> %s", *host, msg)
		}
		return d.Display(Options{Color: config.Manager().Settings.COLOR_VERBOSE, Stderr: toStderr}, msg, a...)
	}

	return nil
}

func Verbose(host *string, caplevel int, msg string, a ...interface{}) {
	_ = Instance().Verbose(host, caplevel, msg, a...)
}

func (d *display) Warning(options WarnOptions, msg string, a ...interface{}) error {
	if !options.Formatted {
		msg = fmt.Sprintf("[WARNING]: %s", msg)
		wrapped := wrapText(msg, d.columns)
		msg = strings.Join(wrapped, "\n") + "\n"
	} else {
		msg = fmt.Sprintf("\n[WARNING]: \n%s", msg)
	}

	return d.Display(Options{Color: config.Manager().Settings.COLOR_WARN, Stderr: true}, msg, a...)
}

func Warning(options WarnOptions, msg string, a ...interface{}) {
	_ = Instance().Warning(options, msg, a...)
}

func (d *display) SystemWarning(msg string, a ...interface{}) error {
	if config.Manager().Settings.SYSTEM_WARNINGS {
		return d.Warning(WarnOptions{}, msg, a...)
	}
	return nil
}

func SystemWarning(msg string, a ...interface{}) {
	_ = Instance().SystemWarning(msg, a...)
}

func (d *display) Banner(options BannerOptions, msg string, a ...interface{}) error {
	// Prints a header-looking line with stars with length depending on terminal width (3 minimum)

	msg = strings.TrimSpace(msg)

	starLen := d.columns - getTextWidth(msg)
	if starLen <= 3 {
		starLen = 3
	}

	return d.Display(Options{Color: options.Color}, fmt.Sprintf("\n%s %s", msg, strings.Repeat("*", starLen)), a...)
}

func Banner(options BannerOptions, msg string, a ...interface{}) {
	_ = Instance().Banner(options, msg, a...)
}

func (d *display) Error(options ErrorOptions, msg string, a ...interface{}) error {
	if options.DontWrapText {
		msg = fmt.Sprintf("ERROR! %s", msg)
	} else {
		msg = fmt.Sprintf("\n[ERROR]: %s", msg)
		wrapped := wrapText(msg, d.columns)
		msg = strings.Join(wrapped, "\n") + "\n"
	}

	return d.Display(Options{Stderr: true, Color: config.Manager().Settings.COLOR_ERROR}, msg, a...)
}

func Error(options ErrorOptions, msg string, a ...interface{}) {
	_ = Instance().Error(options, msg, a...)
}

func Fatal(options ErrorOptions, msg string, a ...interface{}) {
	_ = Instance().Error(options, msg, a...)
	os.Exit(1)
}

func getTextWidth(text string) int {
	return runewidth.StringWidth(text)
}

func wrapText(text string, columns int) []string {
	wrapper := textwrap.NewTextWrap().SetWidth(columns).SetExpandTabs(true).SetReplaceWhitespace(true)
	wrapper = wrapper.SetDropWhitespace(true).SetTabSpacesWidth(8)
	return wrapper.Wrap(text)
}
