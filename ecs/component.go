package ecs

import (
	"github.com/nomos/go-lokas"
	"github.com/nomos/go-lokas/protocol"
	"reflect"
)

type Component struct {
	dirty   bool
	runtime lokas.IRuntime
	entity  lokas.IEntity
}

func (this *Component) SetDirty(d bool) {
	this.dirty = d
	if this.entity!=nil {
		this.entity.SetDirty(true)
	}
}

func (this *Component) SetEntity(e lokas.IEntity) {
	this.entity = e
}

func (this *Component) SetRuntime(r lokas.IRuntime) {
	this.runtime = r
}

func (this *Component) GetRuntime()lokas.IRuntime {
	return this.runtime
}

func (this *Component) GetComponentName()string{
	return protocol.GetTypeRegistry().GetNameByType(reflect.TypeOf(this))
}

func (this *Component) GetSibling(t protocol.BINARY_TAG) lokas.IComponent {
	return this.entity.Get(t)
}
