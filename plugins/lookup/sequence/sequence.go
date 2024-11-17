package sequence

import (
	"errors"
	"fmt"
	"github.com/jmolinski/gosible-templates/exec"
	"github.com/scylladb/gosible/parsing"
	"github.com/scylladb/gosible/utils/maps"
	"regexp"
	"strconv"
	"strings"
)

type seq struct {
	start  int64
	count  int64
	end    int64
	stride int64
	format string
}

const num = "(0?x?[0-9a-f]+)"

var shortcut = regexp.MustCompile(
	"(?i)" +
		"^(" + // Group 0
		num + // Group 1: Start
		"-)?" +
		num + // Group 2: End
		"(/" + // Group 3
		num + // Group 4: Stride
		")?" +
		"(:(.+))?$", // Group 5, Group 6: Format String

)

func (s *seq) reset() {
	s.start = 1
	s.count = -1
	s.end = -1
	s.stride = 1
	s.format = "%d"
}

func (s *seq) parseKvArgs(args map[string]string) error {
	var err error
	argsHelper := map[string]*int64{
		"start":  &s.start,
		"count":  &s.count,
		"end":    &s.end,
		"stride": &s.stride,
	}
	for k, v := range argsHelper {
		if r, ok := args[k]; ok {
			delete(args, k)
			if *v, err = strconv.ParseInt(r, 0, 64); err != nil {
				return fmt.Errorf("can't parse %s=%s as integer", k, r)
			}
		}
	}
	if f, ok := args["format"]; ok {
		delete(args, "format")
		s.format = f
	}
	if len(args) != 0 {
		return fmt.Errorf("unrecognized arguments to with_sequence: %s", strings.Join(maps.Keys(args), ","))
	}
	return nil
}

func (s *seq) parseSimpleArgs(term string) (bool, error) {
	m := shortcut.FindStringSubmatch(term)
	if m == nil {
		return false, nil
	}
	start, end, stride, format := m[1], m[2], m[4], m[6]
	var err error
	if start != "" {
		if s.start, err = strconv.ParseInt(start, 0, 64); err != nil {
			return false, fmt.Errorf("can't parse start=%s as integer", start)
		}
	}
	if end != "" {
		if s.end, err = strconv.ParseInt(end, 0, 64); err != nil {
			return false, fmt.Errorf("can't parse end=%s as integer", end)
		}
	}
	if stride != "" {
		if s.stride, err = strconv.ParseInt(stride, 0, 64); err != nil {
			return false, fmt.Errorf("can't parse stride=%s as integer", stride)
		}
	}
	if format != "" {
		s.format = format
	}

	return true, nil
}

func (s *seq) sanityCheck() error {
	if s.count == -1 && s.end == -1 {
		return errors.New("must specify count or end in with_sequence")
	}
	if s.count != -1 && s.end != -1 {
		return errors.New("can't specify both count and end in with_sequence")
	}

	if s.count != -1 {
		// convert count to end
		if s.count == 0 {
			s.start = 0
			s.end = 0
			s.stride = 0
		} else {
			s.end = s.start + s.count*s.stride - 1
		}
	}

	if s.stride > 0 && s.end < s.start {
		return errors.New("to count backwards make stride negative")
	}
	if s.stride < 0 && s.end > s.start {
		return errors.New("to count forward don't make stride negative")
	}
	if strings.Count(s.format, "%") != 1 || !strings.Contains(s.format, "%d") {
		return fmt.Errorf("bad formatting string: %s", s.format)
	}
	return nil
}

func (s *seq) generateSequence() []string {
	end := s.end + 1
	if s.stride < 1 {
		end = s.end - 1
	}
	numbers := make([]string, 0, (end-s.start)/s.stride)
	for i := s.start; s.stride > 0 && i < end || s.stride < 0 && i > end; i += s.stride {
		numbers = append(numbers, fmt.Sprintf(s.format, i))
	}
	return numbers
}

const Name = "sequence"

func Run(va *exec.VarArgs) *exec.Value {
	var s seq

	result := make([]string, 0)
	for _, term := range va.Args {
		s.reset()
		ok, err := s.parseSimpleArgs(term.String())
		if err != nil {
			return exec.AsValue(err)
		}
		if !ok {
			if err = s.parseKvArgs(parsing.ParseKeyValuePairsString(term.String(), false)); err != nil {
				return exec.AsValue(err)
			}
		}

		if err = s.sanityCheck(); err != nil {
			return exec.AsValue(err)
		}
		if s.stride != 0 {
			result = append(result, s.generateSequence()...)
		}
	}

	return exec.AsValue(result)
}
