package display

import (
	"fmt"
	"github.com/mattn/go-isatty"
	"github.com/scylladb/gosible/config"
	"github.com/scylladb/gosible/constants"
	"github.com/scylladb/gosible/utils/callbacks"
	"os"
	"regexp"
	"strconv"
	"strings"
)

var ansibleColor bool
var parseColorRegex = regexp.MustCompile(
	"Color(?P<Color>[0-9]+)|(?P<rgb>rgb(?P<red>[0-5])(?P<green>[0-5])(?P<blue>[0-5]))|gray(?P<gray>[0-9]+)",
)

func init() {
	callbacks.ScheduleCallback(callbacks.ConfigLoaded, true, callbacks.Callback{Fn: setAnsibleColor, Name: "enable/disable terminal colors based on config", Registerer: "display/color.go"})
}

func setAnsibleColor() {
	ansibleColor = true

	if config.Manager().Settings.ANSIBLE_NOCOLOR {
		ansibleColor = false
	} else if !isatty.IsTerminal(os.Stdout.Fd()) {
		ansibleColor = false
	} else {
		// TODO try to determine terminal colors capabilities
		//    try:
		//        import curses
		//        curses.setupterm()
		//        if curses.tigetnum('colors') < 0:
		//            ANSIBLE_COLOR = False
		//    except ImportError:
		//        # curses library was not found
		//        pass
		//    except curses.error:
		//        # curses returns an error (e.g. could not find terminal)
		//        ANSIBLE_COLOR = False
	}

	if config.Manager().Settings.ANSIBLE_FORCE_COLOR {
		ansibleColor = true
	}
}

func getRegexGroupsByName(regEx *regexp.Regexp, url string) (paramsMap map[string]string) {
	// https://stackoverflow.com/a/39635221

	match := regEx.FindStringSubmatch(url)

	paramsMap = make(map[string]string)
	for i, name := range regEx.SubexpNames() {
		if i > 0 && i <= len(match) && match[i] != "" {
			paramsMap[name] = match[i]
		}
	}
	return paramsMap
}

// SGR (Select Graphic Rendition) parameter string for the specified color name.
func parseColor(color string) string {
	paramsMap := getRegexGroupsByName(parseColorRegex, color)

	if len(paramsMap) == 0 {
		return constants.ColorCodes[color]
	} else if code, ok := paramsMap["Color"]; ok {
		if i, err := strconv.Atoi(code); err == nil {
			return fmt.Sprintf("38;5;%d", i)
		}
	} else if _, ok := paramsMap["rgb"]; ok {
		red, _ := strconv.Atoi(paramsMap["red"])
		green, _ := strconv.Atoi(paramsMap["green"])
		blue, _ := strconv.Atoi(paramsMap["blue"])
		code := (16 + 36*red) + 6*green + blue
		return fmt.Sprintf("38;5;%d", code)
	} else if gray, err := strconv.Atoi(paramsMap["gray"]); err == nil {
		return fmt.Sprintf("38;5;%d", 232+int(gray))
	}

	panic(fmt.Sprintf("unsupported color format - failed to parse %s", color))
}

// Returns a string wrapped in ANSI Color codes.
func stringc(text, color string, wrapNonvisibleChars bool) string {
	if !ansibleColor {
		return text
	}

	colorCode := parseColor(color)
	fmtString := "\033[%sm%s\033[0m"
	if wrapNonvisibleChars {
		// Comment from ansible:
		// This option is provided for use in cases when the
		// formatting of a command line prompt is needed, such as
		// `ansible-console`. As said in `readline` sources:
		// readline/display.c:321
		// /* Current implementation:
		//         \001 (^A) start non-visible characters
		//         \002 (^B) end non-visible characters
		//    all characters except \001 and \002 (following a \001) are copied to
		//    the returned string; all characters except those between \001 and
		//    \002 are assumed to be `visible'. */
		fmtString = "\001\033[%sm\002%s\001\033[0m\002"
	}

	parts := make([]string, 0)
	for _, t := range strings.Split(text, "\n") {
		parts = append(parts, fmt.Sprintf(fmtString, colorCode, t))
	}
	return strings.Join(parts, "\n")
}
