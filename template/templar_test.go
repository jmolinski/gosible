package template

import (
	"github.com/jmolinski/gosible-templates/exec"
	"github.com/scylladb/gosible/plugins/lookup"
	"github.com/scylladb/gosible/utils/types"
	"reflect"
	"testing"
)

type parseTestData struct {
	template       string
	values         types.Vars
	parsedTemplate interface{}
	options        *Options
	returnsErr     bool
}

var defaultVars = types.Vars{
	"foo":                    "bar",
	"bam":                    "{{foo}}",
	"num":                    1,
	"var_true":               true,
	"var_false":              false,
	"var_map":                map[string]string{"a": "b"},
	"bad_map":                "{a='b'",
	"var_list":               []int{1},
	"recursive":              "{{recursive}}",
	"some_var":               "blip",
	"some static_var":        "static_blip",
	"some_keyword":           "{{ foo }}",
	"some_unsafe_var":        nil,
	"some_static_unsafe_var": nil,
	"some_unsafe_keyword":    nil,
	"str_with_error":         "{{ 'str' | from_json }}",
}

func getDefaultTemplar() *Templar {
	return New(defaultVars)
}

// TestTemplarTemplate

func TestIsPossiblyTemplateTrue(t *testing.T) {
	tests := [...]string{
		"{{ foo }}",
		"{% foo %}",
		"{# foo #}",
		"{# {{ foo }} #}",
		"{# {{ nothing }} {# #}",
		"{# {{ nothing }} {# #} #}",
		"{% raw %}{{ foo }}{% endraw %}",
		"{{",
		"{%",
		"{#",
		"{% raw",
	}

	for _, test := range tests {
		if !getDefaultTemplar().IsPossiblyTemplate(test) {
			t.Fatal("should be possibly template", test)
		}
	}
}

func TestIsPossiblyTemplateFalse(t *testing.T) {
	tests := [...]string{
		"{",
		"%",
		"#",
		"foo",
		"}}",
		"%}",
		"raw %}",
		"#}",
	}

	for _, test := range tests {
		if getDefaultTemplar().IsPossiblyTemplate(test) {
			t.Fatal("should not be possibly template", test)
		}
	}
}

// TestIsPossibleTemplate ensures that a broken template still gets templated
func TestIsPossibleTemplate(t *testing.T) {
	// Purposefully invalid jinja
	res, err := getDefaultTemplar().Template("{{ foo|default(False)) }}", NewOptions())
	if err == nil {
		t.Fatal("not nil err should be returned", res)
	}
}

func TestIsTemplateTrue(t *testing.T) {
	tests := [...]string{
		"{{ foo }}",
		"{% foo %}",
		"{# foo #}",
		"{# {{ foo }} #}",
		"{# {{ nothing }} {# #}",
		"{# {{ nothing }} {# #} #}",
		"{% raw %}{{ foo }}{% endraw %}",
	}

	for _, test := range tests {
		if !getDefaultTemplar().IsTemplate(test) {
			t.Fatal("should be template", test)
		}
	}
}

func TestIsTemplateFalse(t *testing.T) {
	tests := [...]string{
		"foo",
		"{{ foo",
		"{% foo",
		"{# foo",
		"{{ foo %}",
		"{{ foo #}",
		"{% foo }}",
		"{% foo #}",
		"{# foo %}",
		"{# foo }}",
		"{{ foo {{",
		//"{% raw %}{% foo %}", // FIXME this test fails because it raises `TemplateSyntaxError` in original implementation but not here.
	}

	for _, test := range tests {
		if getDefaultTemplar().Templatable(test) {
			t.Fatal("should not be template", test)
		}
	}
}

func TestIsTemplateRawString(t *testing.T) {
	if getDefaultTemplar().IsTemplate("foo") {
		t.Fatal("`foo` is not a template")
	}
}

func TestTemplateConvertBareString(t *testing.T) {
	opt := NewOptions()
	opt.ConvertBare = true

	test(t, parseTestData{
		template:       "foo",
		parsedTemplate: "bar",
		options:        opt,
	})
}

func TestTemplateConvertBareFilter(t *testing.T) {
	opt := NewOptions()
	opt.ConvertBare = true

	test(t, parseTestData{
		template:       "foo|capitalize",
		parsedTemplate: "Bar",
		options:        opt,
	})
}

func TestWeird(t *testing.T) {
	test(t, parseTestData{
		template:   "1 2 #}huh{# %}ddfg{% }}dfdfg{{  {%what%} {{#foo#}} {%{bar}%} {#%blip%#} {{asdfsd%} 3 4 {{foo}} 5 6 7",
		returnsErr: true,
	})
}

// TestTemplarMisc

func TestTemplarSimple(t *testing.T) {
	preserve := NewOptions()
	preserve.PreserveTrailingNewLines = true

	notPreserve := NewOptions()
	notPreserve.PreserveTrailingNewLines = false

	bare := NewOptions()
	bare.ConvertBare = true

	undef := NewOptions()
	undef.FailOnUndefined = false
	tests := [...]parseTestData{
		{template: "{{foo}}", parsedTemplate: "bar"},
		{template: "1{{foo}}\n", parsedTemplate: "1bar\n"},
		{template: "2{{foo}}\n", parsedTemplate: "2bar\n", options: preserve},
		{template: "3{{foo}}\n", parsedTemplate: "3bar", options: notPreserve},
		//{template: "{{bam}}", parsedTemplate: "bar"}, //FIXME recursive templating is not currently supported
		{template: "{{num}}", parsedTemplate: 1},
		{template: "{{var_true}}", parsedTemplate: true},
		{template: "{{var_false}}", parsedTemplate: false},
		//{template: "{{var_map}}", parsedTemplate: map[string]string{"a": "b"}}, //FIXME research why this test passes in ansible
		{template: "{{bad_map}}", parsedTemplate: "{a='b'"},
		//{template: "{{var_list}}", parsedTemplate: []int{1}}, //FIXME research why this test passes in ansible
		//{template: 1, parsedTemplate: 1}, //FIXME non string templates should be supported
		//{template: "{{bad_var}}", returnsErr: true}, //FIXME gonja currently changes unexisting variables to empty string and there no config for it
		//{template: "{{recursive}}", returnsErr: true}, //FIXME recursive templating is not currently supported
		//{template: "{{foo-bar}}", returnsErr: true}, //FIXME gonja currently changes unexisting variables to empty string and there no config for it
		//{template: "{{bad_var}}", options: undef, parsedTemplate: "{{bad_var}}"}, //FIXME gonja currently changes unexisting variables to empty string and there no config for it
	}

	for _, data := range tests {
		test(t, data)
	}

	// Test available variables
	templar := New(defaultVars)
	templar.AvailableVariables["foo"] = "bam"
	out, err := templar.Template("{{foo}}", NewOptions())

	if err != nil {
		t.Fatal("Error was not expected", err)
	}
	if !reflect.DeepEqual(out, "bam") {
		t.Fatal("Template didn't parse correctly", "`bam`", out)

	}
}

func TestLookup(t *testing.T) {
	defer lookup.ResetLookupPlugins()

	test(t, parseTestData{
		template:   `{{lookup("foo")}}`,
		returnsErr: true,
	})
	test(t, parseTestData{
		template:   `{{lookup()}}`,
		returnsErr: true,
	})
	lookup.RegisterLookupPlugin("foo", func(*exec.VarArgs) *exec.Value { return exec.AsSafeValue([]interface{}{"bar"}) })

	test(t, parseTestData{
		template:       `{{lookup("foo", wantlist=True)}}`,
		parsedTemplate: []interface{}{"bar"},
	})
	test(t, parseTestData{
		template:       `{{lookup("foo", 42, wantlist=True)}}`,
		parsedTemplate: []interface{}{"bar"},
	})
}

// Own

func test(t *testing.T, data parseTestData) {
	vals := data.values
	if vals == nil {
		vals = defaultVars
	}
	options := data.options
	if options == nil {
		options = NewOptions()
	}
	out, err := New(vals).Template(data.template, options)
	if data.returnsErr {
		if err == nil {
			t.Fatal("Error was expected", err, "got", out)
		}
	} else {
		if err != nil {
			t.Fatal("Error was not expected", err)
		}
		if !reflect.DeepEqual(out, data.parsedTemplate) {
			t.Fatal("Template didn't parse correctly", data.parsedTemplate, reflect.TypeOf(data.parsedTemplate), out, reflect.TypeOf(out))
		}
	}
}

func TestCorrectParse(t *testing.T) {
	correctParseTestData := [...]parseTestData{
		{template: "", values: types.Vars{}, parsedTemplate: ""},
		{template: "{{foo}}", values: types.Vars{"foo": 42}, parsedTemplate: 42},
		{template: "{{foo}} * {{bar}} = {{ baz }}", values: types.Vars{"foo": 6, "bar": 9, "baz": 42}, parsedTemplate: "6 * 9 = 42"},
		{template: "{{foo}} * {{bar}} = {{ foo * bar }}", values: types.Vars{"foo": 6, "bar": 9}, parsedTemplate: "6 * 9 = 54"},
	}

	for _, data := range correctParseTestData {
		test(t, data)
	}
}
