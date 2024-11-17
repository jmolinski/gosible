package template

import (
	"bytes"
	"crypto/sha1"
	"encoding/gob"
	"encoding/json"
	"fmt"
	gojinja2 "github.com/jmolinski/gosible-templates"
	"github.com/jmolinski/gosible-templates/exec"
	"github.com/jmolinski/gosible-templates/tokens"
	"github.com/maja42/goval"
	c "github.com/scylladb/gosible/config"
	"github.com/scylladb/gosible/plugins/lookup"
	"github.com/scylladb/gosible/utils/types"
	"regexp"
	"strings"
)

type sha1hash = [sha1.Size]byte

type Templar struct {
	AvailableVariables types.Vars
	Environment        *gojinja2.Environment
	cache              map[sha1hash]interface{}
	singleVarRegex     *regexp.Regexp
}

type Options struct {
	ConvertBare bool
	Cache       bool
	doTemplateOptions
}

type doTemplateOptions struct {
	PreserveTrailingNewLines bool
	EscapeBackSlashes        bool
	FailOnUndefined          bool
	Overrides                types.Vars
	ConvertData              bool
	DisableLookups           bool
}

func NewOptions() *Options {
	return &Options{
		ConvertBare: false,
		Cache:       true,
		doTemplateOptions: doTemplateOptions{
			PreserveTrailingNewLines: true,
			EscapeBackSlashes:        true,
			FailOnUndefined:          c.Manager().Settings.DEFAULT_UNDEFINED_VAR_BEHAVIOR,
			Overrides:                make(types.Vars),
			ConvertData:              true,
			DisableLookups:           false,
		},
	}
}

func (o *Options) SetConvertBare(v bool) *Options {
	o.ConvertBare = v
	return o
}

func (o *Options) SetCache(v bool) *Options {
	o.Cache = v
	return o
}

func (o *Options) SetPreserveTrailingNewLines(v bool) *Options {
	o.PreserveTrailingNewLines = v
	return o
}

func (o *Options) SetEscapeBackSlashes(v bool) *Options {
	o.EscapeBackSlashes = v
	return o
}

func (o *Options) SetFailOnUndefined(v bool) *Options {
	o.FailOnUndefined = v
	return o
}

func (o *Options) SetConvertData(v bool) *Options {
	o.ConvertData = v
	return o
}

func (o *Options) SetDisableLookups(v bool) *Options {
	o.DisableLookups = v
	return o
}

func (o *Options) SetOverrides(v types.Vars) *Options {
	o.Overrides = v
	return o
}

func New(vars types.Vars) *Templar {
	env := gojinja2.NewEnvironment(gojinja2.NewConfig(), gojinja2.DefaultLoader)
	r := regexp.MustCompile(fmt.Sprintf(`(?m)^%s\s*(\w*)\s*%s$`, env.VariableStartString, env.VariableEndString))
	return &Templar{
		AvailableVariables: vars,
		Environment:        env,
		cache:              make(map[sha1hash]interface{}),
		// FIXME this regex should be re-compiled each time variable_start_string and variable_end_string are changed
		singleVarRegex: r,
	}
}

// Template templates (possibly recursively) any given data as input. If convert_bare is
// set to True, the given data will be wrapped as a gojinja2 variable ('{{foo}}')
// before being sent through the template engine.
func (templar *Templar) Template(templateString string, options *Options) (interface{}, error) {
	if options.ConvertBare {
		templateString = templar.convertBareVariable(templateString)
	}

	if !templar.IsPossiblyTemplate(templateString) {
		return templateString, nil
	}

	// Check to see if the string we are trying to render is just referencing a single
	// var.  In this case we don't want to accidentally change the type of the variable
	// to a string by using the gojinja2 template renderer. We just want to pass it.
	matches := templar.singleVarRegex.FindStringSubmatch(templateString)
	onlyOne := len(matches) != 0
	if onlyOne {
		varName := matches[1]
		if resolvedVar, ok := templar.AvailableVariables[varName]; ok {
			if isNonTemplatedType(resolvedVar) {
				return resolvedVar, nil
			} else if resolvedVar == nil {
				return c.Manager().Settings.DEFAULT_NULL_REPRESENTATION, nil
			}
		}
	}

	var sha1Hash sha1hash
	if options.Cache {
		sha1Hash = calcHash(templateString, options.doTemplateOptions)
		if res, ok := templar.cache[sha1Hash]; ok {
			return res, nil
		}
	}

	templatedString, err := templar.doTemplate(templateString, options.doTemplateOptions)
	var templatedValue interface{} = templatedString
	if err == nil {
		templatedValue = tryConvertTemplatedStringToNativeValue(templatedString)
	}
	// we only cache in the case where we have a single variable
	// name, to make sure we're not putting things which mconvertBareVariableay otherwise
	// be dynamic in the cache (filters, lookups, etc.)
	if options.Cache && onlyOne {
		templar.cache[sha1Hash] = templatedValue
	}
	return templatedValue, err
}

func tryConvertTemplatedStringToNativeValue(templatedString string) interface{} {
	// Strings are in single quotes, but json requires double quotes.
	// Let's pretend that simply naively replacing the single quotes with double quotes is fine.
	// FIXME this is a hack, and should be replaced with a proper parser
	s := strings.ReplaceAll(templatedString, "'", "\"")

	var jsonDecodedVal interface{}
	if err := json.Unmarshal([]byte(s), &jsonDecodedVal); err == nil {
		// Ok, it's nice, we could parse it as json.
		// But json is not great - it converts all numbers to float64.
		// Let's try to evaluate the same expression (that we know is a valid json, so it should be safe)
		// and see if it works. If it does, we can return the evaluated value - which should be better, because
		// no (or less) implicit conversions were made.
		// TODO maybe evaluating (in an empty env, like it is done below, for safety) is a better way than
		// TODO even parsing it as json in the first place?
		// goeval.Evaluate(s, nil, nil) does seem to be closer to python's ast.literal_eval than other parsers
		if evaluatedVal, err := goval.NewEvaluator().Evaluate(s, nil, nil); err == nil {
			return evaluatedVal
		}
		return jsonDecodedVal
	}

	return templatedString
}

// IsPossiblyTemplate determines if a string looks like a template, by seeing if it
// contains a gojinja2 start delimiter. Does not guarantee that the string
// is actually a template.
// This is different than ``isTemplate`` which is more strict.
// This method may return ``True`` on a string that is not templatable.
// Useful when guarding passing a string for templating, but when
// you want to allow the templating engine to make the final
// assessment which may result in ``TemplateSyntaxError``.
func (templar *Templar) IsPossiblyTemplate(data string) bool {
	markers := [...]string{
		templar.Environment.Config.BlockStartString,
		templar.Environment.Config.VariableStartString,
		templar.Environment.Config.CommentStartString,
	}

	for _, marker := range markers {
		if strings.Contains(data, marker) {
			return true
		}
	}
	return false
}

func (templar *Templar) Templatable(data string) bool {
	return templar.IsTemplate(data)
}

// IsTemplate attempts to quickly detect whether a value is a gojinja2
// template. To do so, we look for the first 2 matching gojinja2 tokens for
// start and end delimiters.
func (templar *Templar) IsTemplate(data string) bool {
	found := tokens.Whitespace // Arbitrary token type.
	start := true
	comment := false

	if !templar.IsPossiblyTemplate(data) {
		return false
	}

	s := tokens.LexWithConfig(data, templar.Environment.Config)

	for !s.End() {
		current := s.Current().Type

		if gojinja2BeginTokens[current] {
			if start && current == tokens.CommentBegin {
				comment = true
			}
			start = false
			found = current
		} else if gojinja2EndTokens[current] {
			if closingTokens[found] == current {
				return true
			} else if !comment {
				return false
			}
		}
		s.Next()
	}

	return false
}

// isNonTemplatedType checks if type v should be converted to string in case of template being single variable
func isNonTemplatedType(v interface{}) bool {
	if isNumericType(v) {
		return true
	}
	switch v.(type) {
	case bool:
		return true
	default:
		return false
	}
}

func isNumericType(v interface{}) bool {
	switch v.(type) {
	case uint8, uint16, uint32, uint64, uint,
		int8, int16, int32, int64, int,
		float32, float64,
		complex64, complex128:
		return true
	default:
		return false
	}
}

func calcHash(variable string, options doTemplateOptions) sha1hash {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	_ = enc.Encode(options)

	hash := sha1.New()
	hash.Write([]byte(variable))
	hash.Write(buf.Bytes())
	sum := hash.Sum(nil)

	return *((*sha1hash)(sum))
}

// convertBareVariable wraps a bare string, which may have an attribute portion (ie. foo.bar)
// in gojinja2 variable braces so that it is evaluated properly.
func (templar *Templar) convertBareVariable(variable string) string {
	containsFilter := strings.Contains(variable, "|")
	firstPart := strings.Split(variable, "|")[0]
	firstPart = strings.Split(firstPart, ".")[0]
	firstPart = strings.Split(firstPart, "[")[0]
	_, variableAvailable := templar.AvailableVariables[firstPart]
	if (containsFilter || variableAvailable) && !strings.Contains(variable, templar.Environment.VariableStartString) {
		return fmt.Sprintf("%s%s%s", templar.Environment.VariableStartString, variable, templar.Environment.VariableEndString)
	}
	return variable
}

// TODO should it be exported?
func (templar *Templar) doTemplate(data string, options doTemplateOptions) (string, error) {
	// TODO handle options
	env := templar.getEnv(data, options)
	template, err := env.FromString(data)
	if err != nil {
		return "", err
	}
	return template.Execute(templar.AvailableVariables)
}

var globals = exec.NewContext(map[string]interface{}{
	"lookup": lookup.Lookup,
	"query":  lookup.Query,
})

func (templar *Templar) getEnv(data string, options doTemplateOptions) *gojinja2.Environment {
	// TODO implement overrides
	// https://github.com/ansible/ansible/blob/c8a14c6be846e9a187b9062f861d650c51ef5b45/lib/ansible/template/__init__.py#L1049
	env := gojinja2.NewEnvironment(templar.Environment.Config.Inherit(), templar.Environment.Loader)
	env.Globals.Merge(globals)
	env.Config.KeepTrailingNewline = options.PreserveTrailingNewLines
	return env
}

var closingTokens = map[tokens.Type]tokens.Type{
	tokens.VariableBegin: tokens.VariableEnd,
	tokens.BlockBegin:    tokens.BlockEnd,
	tokens.CommentBegin:  tokens.CommentEnd,
	tokens.RawBegin:      tokens.RawEnd,
}

var gojinja2BeginTokens = map[tokens.Type]bool{
	tokens.VariableBegin: true,
	tokens.BlockBegin:    true,
	tokens.CommentBegin:  true,
	tokens.RawBegin:      true,
}

var gojinja2EndTokens = map[tokens.Type]bool{
	tokens.VariableEnd: true,
	tokens.BlockEnd:    true,
	tokens.CommentEnd:  true,
	tokens.RawEnd:      true,
}

func Template(templateString string, varsEnv types.Vars, options *Options) (interface{}, error) {
	engine := New(varsEnv)
	if options == nil {
		options = NewOptions()
	}

	if !engine.IsTemplate(templateString) {
		return templateString, nil
	} else {
		return engine.Template(templateString, options)
	}
}

func TemplateToString(templateString string, varsEnv types.Vars, options *Options) (interface{}, error) {
	templated, err := Template(templateString, varsEnv, options)
	if err != nil {
		return nil, err
	}
	if s, ok := templated.(string); ok {
		return s, nil
	} else {
		return fmt.Sprintf("%v", templated), nil
	}
}
