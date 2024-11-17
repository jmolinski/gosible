package ping

import (
	"errors"
	"github.com/scylladb/gosible/module_utils/gosibleModule"
	"github.com/scylladb/gosible/modules"
	"github.com/scylladb/gosible/utils/types"
)

type Return struct {
	Ping string
}

type Params struct {
	Data string `mapstructure:"data"`
}

func (p *Params) Validate() error {
	return nil
}

type Module struct {
	*gosibleModule.GosibleModule[*Params]
}

var _ modules.Module = &Module{}

const DefaultData string = "pong"
const errorStr string = "boom"

func New() *Module {
	return &Module{GosibleModule: gosibleModule.New(&Params{Data: DefaultData})}
}

func (m *Module) Name() string {
	return "ping"
}

func (m *Module) Run(ctx *modules.RunContext, vars types.Vars) *modules.Return {
	if err := m.ParseParams(ctx, vars); err != nil {
		return m.MarkReturnFailed(err)
	}

	if m.Params.Data == "crash" {
		return m.MarkReturnFailed(errors.New(errorStr))
	}

	if err := m.Close(); err != nil {
		return m.MarkReturnFailed(err)
	}
	return m.UpdateReturn(&modules.Return{ModuleSpecificReturn: &Return{Ping: m.Params.Data}})
}
