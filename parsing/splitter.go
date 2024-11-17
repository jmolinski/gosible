package parsing

import (
	"fmt"
	"strings"
)

var ignoredFromRawParams = [...]string{
	"creates", "removes", "chdir", "executable", "warn", "stdin", "stdin_add_newline", "strip_empty_ends",
}

// ParseKeyValuePairsString converts a string of key/value items to a dict. If any free-form params
// are found and the check_raw option is set to True, they will be added to a new parameter
// called '_raw_params'. If checkRaw is not enabled, they will simply be ignored.
func ParseKeyValuePairsString(args string, checkRaw bool) map[string]string {
	vargs, err := splitArgs(args)
	if err != nil {
		return nil
	}

	options := map[string]string{}
	var rawParams []string

	for _, rawArg := range vargs {
		x := decodeEscapes(rawArg)
		if strings.Contains(x, "=") {
			pos := findUnescapedEqualsSignPos(x, &rawParams)
			if pos < 0 {
				continue
			}

			k := x[:pos]
			v := x[pos+1:]

			if checkRaw && !isIgnoredFromRawParams(k) {
				rawParams = append(rawParams, rawArg)
			} else {
				options[strings.TrimSpace(k)] = Unquote(strings.TrimSpace(v))
			}
		} else {
			rawParams = append(rawParams, rawArg)
		}
	}

	// recombine the free-form params, if any were found, and assign
	// them to a special option for use later by the shell/command module
	if len(rawParams) > 0 {
		options["_raw_params"] = joinArgs(rawParams)
	}

	return options
}

func isIgnoredFromRawParams(k string) bool {
	for _, x := range ignoredFromRawParams {
		if k == x {
			return true
		}
	}
	return false
}

func decodeEscapes(x string) string {
	// TODO ansible uses codecs unicode-escape, we'll have to port it in the future
	x = strings.ReplaceAll(x, "\\\\", "\\")
	x = strings.ReplaceAll(x, "\\'", "'")
	x = strings.ReplaceAll(x, "\\\"", "\"")
	return x
}

// findEqualsSignPos returns the position of the equals sign in the string, and -1 if the = character could not be found.
func findUnescapedEqualsSignPos(x string, rawParams *[]string) int {
	pos := 0
	for {
		offset := pos + 1
		pos = strings.Index(x[offset:], "=")
		if pos == -1 {
			// ran out of string, but we must have some escaped equals,
			// so replace those and append this to the list of raw params
			*rawParams = append(*rawParams, strings.ReplaceAll(x, "\\=", "="))
			return -1
		} else {
			pos += offset
			if x[pos-1] != '\\' {
				return pos
			}
		}
	}
}

type stringBuilderLastCharAware struct {
	builder  strings.Builder
	LastChar byte
	NonEmpty bool
}

func (b *stringBuilderLastCharAware) write(s string) {
	if len(s) > 0 {
		b.LastChar = s[len(s)-1]
		b.builder.WriteString(s)
		b.NonEmpty = true
	}
}

func (b *stringBuilderLastCharAware) String() string {
	return b.builder.String()
}

// Join the original cmd based on manipulations by split_args().
// This retains the original newlines and whitespaces.
func joinArgs(s []string) string {
	var result stringBuilderLastCharAware
	for _, p := range s {
		if !result.NonEmpty || result.LastChar == '\n' {
			result.write(p)
		} else {
			result.write(" ")
			result.write(p)
		}
	}
	return result.String()
}

// splitArgs splits args on whitespace, but intelligently reassembles
// those that may have been split over a jinja2 block or quotes.
//
// When used in a remote module, we won't ever have to be concerned about
// jinja2 blocks, however this function is/will be used in the
// core portions as well before the args are templated.
//
// example input: a=b c="foo bar"
// example output: ['a=b', 'c="foo bar"']
//
// Basically this is a variation shlex that has some more intelligence for
// how Ansible needs to use it.
func splitArgs(args string) ([]string, error) {
	// the list of params parsed out of the arg string
	// this is going to be the result value when we are done
	var params []string

	// Initial split on newlines
	items := strings.Split(args, "\n")

	// These variables are used
	// to keep track of the state of the parsing, since blocks and quotes
	// may be nested within each other.

	var quoteChar string
	insideQuotes := false
	printDepth := 0   // used to count nested jinja2 {{ }} blocks
	blockDepth := 0   // used to count nested jinja2 {% %} blocks
	commentDepth := 0 // used to count nested jinja2 {# #} blocks

	// now we loop over each split chunk, coalescing tokens if the white space
	// split occurred within quotes or a jinja2 block of some kind
	for itemIdx, item := range items {
		// we split on spaces and newlines separately, so that we
		// can tell which character we split on for reassembly
		// inside quotation characters
		tokens := strings.Split(item, " ")

		lineContinuation := false
		for idx, token := range tokens {
			// Empty entries means we have subsequent spaces
			// We want to hold onto them so we can reconstruct them later
			if token == "" && idx != 0 {
				params[len(params)-1] += " "
				continue
			}

			// if we hit a line continuation character, but
			// we're not inside quotes, ignore it and continue
			// on to the next token while setting a flag
			if token == "\\" && !insideQuotes {
				lineContinuation = true
				continue
			}

			// store the previous quoting state for checking later
			wasInsideQuotes := insideQuotes
			quoteChar = getQuoteState(token, quoteChar)
			insideQuotes = quoteChar != ""

			// multiple conditions may append a token to the list of params,
			// so we keep track with this flag to make sure it only happens once
			// append means add to the end of the list, don't append means concatenate
			// it to the end of the last token
			appended := false

			// if we're inside quotes now, but weren't before, append the token
			// to the end of the list, since we'll tack on more to it later
			// otherwise, if we're inside any jinja2 block, inside quotes, or we were
			// inside quotes (but aren't now) concat this token to the last param
			if insideQuotes && !wasInsideQuotes && !(printDepth > 0 || blockDepth > 0 || commentDepth > 0) {
				params = append(params, token)
				appended = true
			} else if printDepth > 0 || blockDepth > 0 || commentDepth > 0 || insideQuotes || wasInsideQuotes {
				if idx == 0 && wasInsideQuotes {
					params[len(params)-1] = fmt.Sprintf("%s%s", params[len(params)-1], token)
				} else if len(tokens) > 1 {
					spacer := ""
					if idx > 0 {
						spacer = " "
					}
					params[len(params)-1] = fmt.Sprintf("%s%s%s", params[len(params)-1], spacer, token)
				} else {
					params[len(params)-1] = fmt.Sprintf("%s\n%s", params[len(params)-1], token)
				}
				appended = true
			}

			// if the number of paired block tags is not the same, the depth has changed, so we calculate that here
			// and may append the current token to the params (if we haven't previously done so)
			prevPrintDepth := printDepth
			printDepth = countJinja2Blocks(token, printDepth, "{{", "}}")
			if printDepth != prevPrintDepth && !appended {
				params = append(params, token)
				appended = true
			}

			prevBlockDepth := blockDepth
			blockDepth = countJinja2Blocks(token, blockDepth, "{%", "%}")
			if blockDepth != prevBlockDepth && !appended {
				params = append(params, token)
				appended = true
			}

			prevCommentDepth := commentDepth
			commentDepth = countJinja2Blocks(token, commentDepth, "{#", "#}")
			if commentDepth != prevCommentDepth && !appended {
				params = append(params, token)
				appended = true
			}

			// finally, if we're at zero depth for all blocks and not inside quotes, and have not
			// yet appended anything to the list of params, we do so now
			if !(printDepth > 0 || blockDepth > 0 || commentDepth > 0) && !insideQuotes && !appended && token != "" {
				params = append(params, token)
			}
		}

		// if this was the last token in the list, and we have more than
		// one item (meaning we split on newlines), add a newline back here
		// to preserve the original structure
		if len(items) > 1 && itemIdx != len(items)-1 && !lineContinuation {
			params[len(params)-1] += "\n"
		}
	}

	// If we're done and things are not at zero depth or we're still inside quotes,
	// raise an error to indicate that the args were unbalanced
	if printDepth != 0 || blockDepth != 0 || commentDepth != 0 || insideQuotes {
		return nil, fmt.Errorf("failed at splitting arguments, either an unbalanced jinja2 block or quotes: %q", args)
	}

	return params, nil
}

// getQuoteState determines if the quoted string is unterminated in which case it needs to be put back together.
// It takes current quoteChar ("", "'" or "\"" are the only valid values) and returns the new one (if any).
// An empty string means the string is not quoted (eg. the terminating quote has been found),
// otherwise the returned value is the new quoteChar.
func getQuoteState(token string, quoteChar string) string {
	prevCharIsEscape := false
	for idx, char := range token {
		if idx > 0 {
			prevCharIsEscape = token[idx-1] == '\\'
		}
		if (char == '"' || char == '\'') && !prevCharIsEscape {
			if quoteChar != "" {
				if string(char) == quoteChar {
					quoteChar = ""
				}
			} else {
				quoteChar = string(char)
			}
		}
	}

	return quoteChar
}

// countJinja2Blocks counts the number of opening/closing blocks for a
// given opening/closing type and adjusts the current depth for that
// block based on the difference
func countJinja2Blocks(token string, curDepth int, openToken string, closeToken string) int {
	numOpen := strings.Count(token, openToken)
	numClose := strings.Count(token, closeToken)
	if numOpen != numClose {
		curDepth += numOpen - numClose
		if curDepth < 0 {
			curDepth = 0
		}
	}
	return curDepth
}
