package lox

import (
	"github.com/nomos/go-lokas"
	"github.com/nomos/go-lokas/rox"
	"net/http"
	"sync"
)

var StatCtor = statCtor{}

type statCtor struct{}

func (this statCtor) Type() string {
	return "Stat"
}

func (this statCtor) Create() lokas.IModule {
	ret := &Stat{
		Http:   HttpCtor.Create().(*Http),
		status: map[string]string{},
	}
	ret.SetType(this.Type())
	return ret
}

var _ lokas.IModule = (*Stat)(nil)

type Stat struct {
	*Http
	status map[string]string
	sync.RWMutex
}

func (this *Stat) Type() string {
	return HttpCtor.Type()
}

func (this *Stat) Load(conf lokas.IConfig) error {
	err := this.Http.Load(conf)
	if err != nil {
		return err
	}
	this.Use(rox.CorsAllowAll().MiddleWare)
	this.Use(rox.FormParser)
	this.Use(rox.RequestLogger)
	this.Use(rox.ErrHandler)

	this.HandleFunc("/api/setstat", setStat).Methods("POST")
	this.HandleFunc("/api/getstat", getStat).Methods("POST")
	return nil
}

var setStat = rox.CreateHandler(func(w rox.ResponseWriter, r *http.Request, a lokas.IProcess) {

})

var getStat = rox.CreateHandler(func(w rox.ResponseWriter, r *http.Request, a lokas.IProcess) {

})

func (this *Stat) SetStatus(key string, value string) {
	this.RWMutex.Lock()
	defer this.RWMutex.Unlock()
	this.status[key] = value
}

func (this *Stat) GetStatus(key string) string {
	this.RWMutex.RLock()
	defer this.RWMutex.RUnlock()
	return this.status[key]
}

func (this *Stat) Unload() error {
	return this.Http.Unload()
}

func (this *Stat) OnStart() error {
	return this.Http.OnStart()
}

func (this *Stat) OnStop() error {
	return this.Http.OnStop()
}
