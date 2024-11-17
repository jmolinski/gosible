package wrappers

import (
	"github.com/mitchellh/mapstructure"
	"github.com/scylladb/gosible/modules"
)

type Return struct {
	ret *modules.Return
}

func NewReturn() *Return {
	return &Return{&modules.Return{InternalReturn: &modules.InternalReturn{}}}
}

func (r *Return) MarkReturnFailed(err error) *modules.Return {
	r.ret.Failed = true
	if r.ret.InternalReturn == nil {
		r.ret.InternalReturn = &modules.InternalReturn{}
	}
	r.ret.InternalReturn.Exception = err.Error()
	r.Warn(err.Error())
	return r.ret
}

func (r *Return) Warn(msg string) {
	if r.ret.InternalReturn == nil {
		r.ret.InternalReturn = &modules.InternalReturn{}
	}
	r.ret.InternalReturn.Warnings = append(r.ret.InternalReturn.Warnings, msg)
}

func (r *Return) Debug(msg string) {
	if r.ret.InternalReturn == nil {
		r.ret.InternalReturn = &modules.InternalReturn{}
	}
	r.ret.InternalReturn.Debug = append(r.ret.InternalReturn.Debug, msg)
}

func (r *Return) Deprecate(deprecation modules.Deprecation) {
	if r.ret.InternalReturn == nil {
		r.ret.InternalReturn = &modules.InternalReturn{}
	}
	r.ret.InternalReturn.Deprecations = append(r.ret.InternalReturn.Deprecations, deprecation)
}

func (r *Return) GetReturn() *modules.Return {
	return r.ret
}

func (r *Return) UpdateReturn(update *modules.Return) *modules.Return {
	helper := map[string]any{}
	_ = mapstructure.Decode(update, &helper)
	_ = mapstructure.Decode(helper, &r.ret)

	return r.ret
}
