package rox

import (
	"fmt"
	"github.com/nomos/go-log/log"
	"github.com/nomos/go-lokas"
	"net/http"
	"runtime"
	"runtime/debug"
)

const panicText = "PANIC: %s\n%s"

const NoPrintStackBodyString = "500 Internal Server Error"

type PanicFormatter interface {
	FormatPanicError(rw http.ResponseWriter, r *http.Request, infos *PanicInformation)
}

type TextPanicFormatter struct{}

func (t *TextPanicFormatter) FormatPanicError(rw http.ResponseWriter, r *http.Request, infos *PanicInformation) {
	if rw.Header().Get("Content-Type") == "" {
		rw.Header().Set("Content-Type", "text/plain; charset=utf-8")
	}
	fmt.Fprintf(rw, panicText, infos.RecoveredPanic, infos.Stack)
}

type PanicInformation struct {
	RecoveredPanic interface{}
	Stack          []byte
	Request        *http.Request
}

func (p *PanicInformation) StackAsString() string {
	return string(p.Stack)
}

const nilRequestMessage = "Request is nil"

func (p *PanicInformation) RequestDescription() string {

	if p.Request == nil {
		return nilRequestMessage
	}

	var queryOutput string
	if p.Request.URL.RawQuery != "" {
		queryOutput = "?" + p.Request.URL.RawQuery
	}
	return fmt.Sprintf("%s %s%s", p.Request.Method, p.Request.URL.Path, queryOutput)
}

type Recovery struct {
	PrintStack       bool
	LogStack         bool
	PanicHandlerFunc func(*PanicInformation)
	StackAll         bool
	StackSize        int
	Formatter        PanicFormatter
}

// NewRecovery returns a new instance of Recovery
func NewRecovery() *Recovery {
	return &Recovery{
		PrintStack: true,
		LogStack:   true,
		StackAll:   false,
		StackSize:  1024 * 8,
		Formatter:  &TextPanicFormatter{},
	}
}

func (rec *Recovery) ServeHTTP(writer ResponseWriter, request *http.Request, next http.Handler) {
	defer func() {
		if err := recover(); err != nil {
			writer.WriteHeader(http.StatusInternalServerError)

			infos := &PanicInformation{
				RecoveredPanic: err,
				Request:        request,
				Stack:          make([]byte, rec.StackSize),
			}
			infos.Stack = infos.Stack[:runtime.Stack(infos.Stack, rec.StackAll)]
			if rec.PrintStack && rec.Formatter != nil {
				rec.Formatter.FormatPanicError(writer, request, infos)
			} else {
				if writer.Header().Get("Content-Type") == "" {
					writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
				}
				InternalServerError(writer,NoPrintStackBodyString)
			}

			if rec.LogStack {
				log.Errorf(panicText, err, infos.Stack)
			}
			if rec.PanicHandlerFunc != nil {
				func() {
					defer func() {
						if err := recover(); err != nil {
							log.Errorf("provided ErrorHandlerFunc panic'd: %s, trace:\n%s", err, debug.Stack())
							log.Errorf("%s\n", debug.Stack())
						}
					}()
					rec.PanicHandlerFunc(infos)
				}()
			}
		}
	}()
	next.ServeHTTP(writer, request)
}

var ErrHandler = CreateMiddleWare(func(w ResponseWriter, r *http.Request, a lokas.IProcess, next http.Handler) {
	rec:=NewRecovery()
	rec.ServeHTTP(w.(ResponseWriter),r,next)
})

