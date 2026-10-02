package v8go

// #include "inspector.h"
import "C"
import (
	"runtime/cgo"
	"strconv"
	"unicode/utf16"
)

type MessageErrorLevel uint8

func (lvl MessageErrorLevel) String() string {
	switch lvl {
	case ErrorLevelLog:
		return "log"
	case ErrorLevelDebug:
		return "debug"
	case ErrorLevelError:
		return "error"
	case ErrorLevelInfo:
		return "info"
	case ErrorLevelWarning:
		return "warning"
	default:
		return strconv.Itoa(int(lvl))
	}
}

const (
	ErrorLevelLog MessageErrorLevel = 1 << iota
	ErrorLevelDebug
	ErrorLevelInfo
	ErrorLevelError
	ErrorLevelWarning
	ErrorLevelAll = ErrorLevelLog | ErrorLevelDebug | ErrorLevelInfo | ErrorLevelError | ErrorLevelWarning
)

type Inspector struct {
	ptr *C.v8Inspector
}

type InspectorClient struct {
	ptr          *C.v8InspectorClient
	clientHandle cgo.Handle
}

type ConsoleAPIMessage struct {
	contextGroupId int
	ErrorLevel     MessageErrorLevel
	Message        string
	Url            string
	LineNumber     uint
	ColumnNumber   uint
}

type ConsoleAPIMessageHandler interface {
	ConsoleAPIMessage(message ConsoleAPIMessage)
}

func NewInspector(iso *Isolate, client *InspectorClient) *Inspector {
	ptr := C.CreateInspector(iso.ptr, client.ptr)
	return &Inspector{
		ptr: ptr,
	}
}

func (i *Inspector) Dispose() {
	C.DeleteInspector(i.ptr)
}

func (i *Inspector) ContextCreated(ctx *Context) {
	C.InspectorContextCreated(i.ptr, ctx.ptr)
}

func (i *Inspector) ContextDestroyed(ctx *Context) {
	C.InspectorContextDestroyed(i.ptr, ctx.ptr)
}

func NewInspectorClient(handler ConsoleAPIMessageHandler) *InspectorClient {
	clientHandle := cgo.NewHandle(handler)
	ptr := C.NewInspectorClient(C.uintptr_t(clientHandle))
	return &InspectorClient{
		clientHandle: clientHandle,
		ptr:          ptr,
	}
}

func (c *InspectorClient) Dispose() {
	c.clientHandle.Delete()
	C.DeleteInspectorClient(c.ptr)
}

func stringViewToString(d C.StringViewData) string {
	if d.is8bit {
		data := C.GoBytes(d.data, d.length)
		return string(data)
	}

	data := C.GoBytes(d.data, d.length*2)
	shorts := make([]uint16, len(data)/2)
	for i := 0; i < len(data); i += 2 {
		shorts[i/2] = (uint16(data[i+1]) << 8) | uint16(data[i])
	}

	return string(utf16.Decode(shorts))
}

//export goHandleConsoleAPIMessageCallback
func goHandleConsoleAPIMessageCallback(
	cgoHandle C.uintptr_t,
	contextGroupId C.int,
	errorLevel C.int,
	message C.StringViewData,
	url C.StringViewData,
	lineNumber C.uint,
	columnNumber C.uint,
) {
	handle := cgo.Handle(cgoHandle)
	if client, ok := handle.Value().(ConsoleAPIMessageHandler); ok {
		client.ConsoleAPIMessage(ConsoleAPIMessage{
			contextGroupId: int(contextGroupId),
			ErrorLevel:     MessageErrorLevel(errorLevel),
			Message:        stringViewToString(message),
			Url:            stringViewToString(url),
			LineNumber:     uint(lineNumber),
			ColumnNumber:   uint(columnNumber),
		})
	}
}
