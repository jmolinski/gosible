package lookup

import (
	"github.com/scylladb/gosible/plugins/lookup/env"
	"github.com/scylladb/gosible/plugins/lookup/indexed_items"
	"github.com/scylladb/gosible/plugins/lookup/items"
	"github.com/scylladb/gosible/plugins/lookup/list"
	"github.com/scylladb/gosible/plugins/lookup/random_choice"
	"github.com/scylladb/gosible/plugins/lookup/sequence"
	"github.com/scylladb/gosible/plugins/lookup/url"
	"github.com/scylladb/gosible/plugins/lookup/varnames"
	"github.com/scylladb/gosible/plugins/lookup/vars"
)

func RegisterDefaultPlugins() {
	RegisterLookupPlugin(url.Name, url.Run)
	RegisterLookupPlugin(sequence.Name, sequence.Run)
	RegisterLookupPlugin(indexed_items.Name, indexed_items.Run)
	RegisterLookupPlugin(random_choice.Name, random_choice.Run)
	RegisterLookupPlugin(list.Name, list.Run)
	RegisterLookupPlugin(items.Name, items.Run)
	RegisterLookupPlugin(env.Name, env.Run)
	RegisterLookupPlugin(vars.Name, vars.Run)
	RegisterLookupPlugin(varnames.Name, varnames.Run)
}
