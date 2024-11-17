package action

import (
	"github.com/mitchellh/mapstructure"
	"github.com/scylladb/gosible/utils/types"
	"github.com/scylladb/gosible/utils/wrappers"
)

type Validatable interface {
	Validate() error
}

type Base[P Validatable] struct {
	*wrappers.Return
	Params P
}

type DefaultParams struct{}

func (*DefaultParams) Validate() error {
	return nil
}

func New[P Validatable](defaultParams P) *Base[P] {
	return &Base[P]{
		Return: wrappers.NewReturn(),
		Params: defaultParams,
	}
}

func (a *Base[P]) ParseParams(vars types.Vars) error {
	if err := mapstructure.Decode(vars, a.Params); err != nil {
		return err
	}
	return a.Params.Validate()
}
