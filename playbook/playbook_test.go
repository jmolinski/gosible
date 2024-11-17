package playbook

import (
	"github.com/scylladb/gosible/modules"
	defaultModules "github.com/scylladb/gosible/modules/default"
	defaultPlugins "github.com/scylladb/gosible/plugins/default"
	"reflect"
	"testing"
)

func TestReadYaml(t *testing.T) {
	defaultPlugins.Register()
	mods := modules.NewRegistry()
	defaultModules.Register(mods)

	pbook, err := Parse("tests/assets/singleSimplePlayPb.yml", mods)
	if err != nil {
		t.Fatal("Parsing error was not expected\n", err)
	}

	if len(pbook.Plays) != 1 {
		t.Fatal("Expected 1 play, got", len(pbook.Plays))
	}

	play := pbook.Plays[0]
	if play.HostsPattern != "all:db" {
		t.Fatal("Expected hosts \"all:db\", got", play.HostsPattern)
	}
}

func TestResolveAction(t *testing.T) {
	defaultPlugins.Register()
	mods := modules.NewRegistry()
	defaultModules.Register(mods)

	pbook, err := Parse("tests/assets/singleSimplePlayPb.yml", mods)
	if err != nil {
		t.Fatal("Parsing error was not expected\n", err)
	}

	play := pbook.Plays[0]

	if len(play.Tasks) != 2 {
		t.Fatal("Expected 2 tasks\n")
	}

	// TODO: expand Task tests suite

	task := play.Tasks[0]
	if task.Action.Name != "get_url" {
		t.Fatal("Expected action to be get_url, got", task.Action.Name)
	}
}

func TestParseLoops(t *testing.T) {
	defaultPlugins.Register()
	mods := modules.NewRegistry()
	defaultModules.Register(mods)

	pbook, err := Parse("tests/assets/loops.yml", mods)
	if err != nil {
		t.Fatal("Parsing error was not expected\n", err)
	}

	if len(pbook.Plays) != 1 {
		t.Fatal("Expected 1 play, got", len(pbook.Plays))
	}
	play := pbook.Plays[0]
	if len(play.Tasks) != 2 {
		t.Fatal("Expected 2 tasks, got", len(play.Tasks))
	}

	if play.Tasks[0].Loop == nil || play.Tasks[1].Loop == nil {
		t.Fatal("Expected loop to be set")
	}

	if !reflect.DeepEqual(play.Tasks[0].Loop.Items, []interface{}{"one", "two"}) {
		t.Fatal("Expected loop items to be one and two, got", play.Tasks[0].Loop.Items)
	}
	if play.Tasks[1].Loop.Template != "{{ lookup('sequence', 'end=42 start=2 step=2') }}" {
		t.Fatal("Expected loop template to be '{{ lookup('sequence', 'end=42 start=2 step=2') }}', got", play.Tasks[1].Loop.Template)
	}
}
