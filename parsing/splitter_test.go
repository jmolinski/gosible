package parsing

import "testing"

type testData struct {
	s   string
	sa  []string
	pkv map[string]string
}

var dataForTests = []testData{
	{s: "a", sa: []string{"a"}, pkv: map[string]string{"_raw_params": "a"}},
	{s: "a=b", sa: []string{"a=b"}, pkv: map[string]string{"a": "b"}},
	{s: "a=\"foo bar\"", sa: []string{"a=\"foo bar\""}, pkv: map[string]string{"a": "foo bar"}},
	{s: "\"foo bar baz\"", sa: []string{"\"foo bar baz\""}, pkv: map[string]string{"_raw_params": "\"foo bar baz\""}},
	{s: "foo bar baz", sa: []string{"foo", "bar", "baz"}, pkv: map[string]string{"_raw_params": "foo bar baz"}},
	{s: "a=b c=\"foo bar\"", sa: []string{"a=b", "c=\"foo bar\""}, pkv: map[string]string{"a": "b", "c": "foo bar"}},
	{s: "a=\"echo \\\"hello world\\\"\" b=bar", sa: []string{"a=\"echo \\\"hello world\\\"\"", "b=bar"}, pkv: map[string]string{"a": "echo \"hello world\"", "b": "bar"}},
	{s: "a=\"blank\n\nline\"", sa: []string{"a=\"blank\n\nline\""}, pkv: map[string]string{"a": "blank\n\nline"}},
	{s: "a=\"blank\n\n\nlines\"", sa: []string{"a=\"blank\n\n\nlines\""}, pkv: map[string]string{"a": "blank\n\n\nlines"}},
	{s: "a=\"a long\nmessage\\\nabout a thing\n\"", sa: []string{"a=\"a long\nmessage\\\nabout a thing\n\""}, pkv: map[string]string{"a": "a long\nmessage\\\nabout a thing\n"}},
	{s: "a=\"multiline\nmessage1\\\n\" b=\"multiline\nmessage2\\\n\"", sa: []string{"a=\"multiline\nmessage1\\\n\"", "b=\"multiline\nmessage2\\\n\""}, pkv: map[string]string{"a": "multiline\nmessage1\\\n", "b": "multiline\nmessage2\\\n"}},
	{s: "a={{jinja}}", sa: []string{"a={{jinja}}"}, pkv: map[string]string{"a": "{{jinja}}"}},
	{s: "a={{ jinja }}", sa: []string{"a={{ jinja }}"}, pkv: map[string]string{"a": "{{ jinja }}"}},
	{s: "a=\"{{jinja}}\"", sa: []string{"a=\"{{jinja}}\""}, pkv: map[string]string{"a": "{{jinja}}"}},
	{s: "a={{ jinja }}{{jinja2}}", sa: []string{"a={{ jinja }}{{jinja2}}"}, pkv: map[string]string{"a": "{{ jinja }}{{jinja2}}"}},
	{s: "a=\"{{ jinja }}{{jinja2}}\"", sa: []string{"a=\"{{ jinja }}{{jinja2}}\""}, pkv: map[string]string{"a": "{{ jinja }}{{jinja2}}"}},
	{s: "a={{jinja}} b={{jinja2}}", sa: []string{"a={{jinja}}", "b={{jinja2}}"}, pkv: map[string]string{"a": "{{jinja}}", "b": "{{jinja2}}"}},
	{s: "a=\"{{jinja}}\n\" b=\"{{jinja2}}\n\"", sa: []string{"a=\"{{jinja}}\n\"", "b=\"{{jinja2}}\n\""}, pkv: map[string]string{"a": "{{jinja}}\n", "b": "{{jinja2}}\n"}},
	{s: "a=\"café eñyei\"", sa: []string{"a=\"café eñyei\""}, pkv: map[string]string{"a": "café eñyei"}},
	{s: "a=café b=eñyei", sa: []string{"a=café", "b=eñyei"}, pkv: map[string]string{"a": "café", "b": "eñyei"}},
	{s: "a={{ foo | some_filter(' ', \" \") }} b=bar", sa: []string{"a={{ foo | some_filter(' ', \" \") }}", "b=bar"}, pkv: map[string]string{"a": "{{ foo | some_filter(' ', \" \") }}", "b": "bar"}},
	{s: "One\n  Two\n    Three\n", sa: []string{"One\n ", "Two\n   ", "Three\n"}, pkv: map[string]string{"_raw_params": "One\n  Two\n    Three\n"}},
}

func TestSplitArgs(t *testing.T) {
	for _, data := range dataForTests {
		split, err := splitArgs(data.s)
		if err != nil {
			t.Errorf("splitArgs(%q) failed: %s", data.s, err)
		}
		if len(split) != len(data.sa) {
			t.Errorf("splitArgs(%q) failed: expected %q, got %q", data.s, data.sa, split)
		}
		for i, s := range split {
			if s != data.sa[i] {
				t.Errorf("splitArgs(%q) failed: expected %q, got %q", data.s, data.sa, split)
			}
		}
	}
}

func TestParseKeyValuePairsString(t *testing.T) {
	for _, data := range dataForTests {
		parsed := ParseKeyValuePairsString(data.s, false)
		if parsed == nil {
			t.Errorf("ParseKeyValuePairsString(%q) failed: got nil", data.s)
		}
		if len(parsed) != len(data.pkv) {
			t.Errorf("ParseKeyValuePairsString(%q) failed: expected %q, got %q", data.s, data.pkv, parsed)
		}
		for i, s := range data.pkv {
			actual, ok := parsed[i]
			if !ok {
				t.Errorf("ParseKeyValuePairsString(%q) failed: expected %q, got %q", data.s, data.pkv, parsed)
			}
			if actual != s {
				t.Errorf("ParseKeyValuePairsString(%q) failed: expected %q, got %q", data.s, data.pkv, parsed)
			}
		}
	}
}
