package edge

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/netip"
	"sync"
	"sync/atomic"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

type EdgeComputer interface {
	io.Closer
	Register(ctx context.Context, method, path string, binary []byte) error
	Unregister(method, path string) error
	StartRequest(ctx context.Context, p *http.Request) (uint64, error)
	ProcessRequest(ctx context.Context, reqID uint64, p *http.Request) error
	ProcessResponse(ctx context.Context, reqID uint64, p *http.Response) error
	FinishRequest(ctx context.Context, reqID uint64) error
}

var _ EdgeComputer = (*edgeComputing)(nil)

type HandleContext struct {
	mod  api.Module
	req  *RequestInfo
	resp *ResponseInfo

	// optimized for caching
	bufferPointer uint32
	bufferSize    uint32
}

type edgeComputing struct {
	rt         wazero.Runtime
	registerRW sync.RWMutex
	compiled   map[string]wazero.CompiledModule

	handlerRW     sync.RWMutex
	atomicCounter atomic.Uint64
	handleContext map[uint64]*HandleContext
}

func NewEdgeComputing(rt wazero.Runtime) EdgeComputer {
	c := &edgeComputing{
		rt:            rt,
		compiled:      make(map[string]wazero.CompiledModule),
		handleContext: make(map[uint64]*HandleContext),
	}
	if err := c.initRequestHandler(); err != nil {
		log.Fatalf("Failed to initialize request handler: %v", err)
	}
	return c
}

func (r *edgeComputing) Close() error {
	return r.rt.Close(context.Background())
}

type HandlerFunc func(c context.Context, mod api.Module, offset, size uint32) uint32

var wellKnownMethods map[string]Method

func init() {
	wellKnownMethods = make(map[string]Method, int(Method_Other))
	for i := 0; i < int(Method_Other); i++ {
		method := Method(i)
		wellKnownMethods[method.String()] = method
	}
}

func getRequestInfo(r *http.Request) (*RequestInfo, error) {
	info := &RequestInfo{}
	remoteAddrParsed, err := netip.ParseAddrPort(r.RemoteAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse remote address %s: %w", r.RemoteAddr, err)
	}
	switch r.ProtoMajor {
	case 1:
		if r.TLS == nil {
			info.Protocol.Protocol = Protocol_Http1Plain
		} else {
			info.Protocol.Protocol = Protocol_Http1
		}
	case 2:
		info.Protocol.Protocol = Protocol_H2
	case 3:
		info.Protocol.Protocol = Protocol_H3
	default:
		info.Protocol.Protocol = Protocol_Other
		if !info.Protocol.SetProtocolName([]byte(r.Proto)) {
			return nil, fmt.Errorf("failed to set protocol name for %s", r.Proto)
		}
	}
	if remoteAddrParsed.Addr().Is4() {
		info.RemoteAddr.SetIsV6(false)
		if !info.RemoteAddr.SetAddrV4(remoteAddrParsed.Addr().As4()) {
			return nil, fmt.Errorf("failed to set IPv4 address %s", remoteAddrParsed.Addr().String())
		}
	} else if remoteAddrParsed.Addr().Is6() {
		info.RemoteAddr.SetIsV6(true)
		if !info.RemoteAddr.SetAddrV6(remoteAddrParsed.Addr().As16()) {
			return nil, fmt.Errorf("failed to set IPv6 address %s", remoteAddrParsed.Addr().String())
		}
	} else {
		return nil, fmt.Errorf("unsupported address type: %s", remoteAddrParsed.Addr().String())
	}
	info.RemotePort = remoteAddrParsed.Port()
	if m, ok := wellKnownMethods[r.Method]; ok {
		info.Method.Method = m
	} else {
		info.Method.Method = Method_Other
		if !info.Method.SetMethodName([]byte(r.Method)) {
			return nil, fmt.Errorf("failed to set method name for %s", r.Method)
		}
	}
	var headers = make([]Field, 0, len(r.Header))
	for key, h := range r.Header {
		for _, value := range h {
			var field Field
			if !field.Key.SetData([]byte(key)) {
				return nil, fmt.Errorf("failed to set header key %s", key)
			}
			if !field.Value.SetData([]byte(value)) {
				return nil, fmt.Errorf("failed to set header value for %s", key)
			}
			headers = append(headers, field)
		}
	}
	if !info.Header.SetFields(headers) {
		return nil, fmt.Errorf("failed to set headers for %s", r.URL.Path)
	}
	if !info.Path.Path.SetData([]byte(r.URL.Path)) {
		return nil, fmt.Errorf("failed to set path for %s", r.URL.Path)
	}
	q := r.URL.Query()
	if len(q) > 0 {
		query := make([]Field, 0, len(q))
		for key, v := range q {
			for _, value := range v {
				var f Field
				if !f.Key.SetData([]byte(key)) {
					return nil, fmt.Errorf("failed to set query key %s", key)
				}
				if !f.Value.SetData([]byte(value)) {
					return nil, fmt.Errorf("failed to set query value for %s", key)
				}
				query = append(query, f)
			}
		}
		if !info.Path.SetQuery(query) {
			return nil, fmt.Errorf("failed to set query fields for %s", r.URL.Path)
		}
	}
	return info, nil
}

func getResponseInfo(p *http.Response) (*ResponseInfo, error) {
	info := &ResponseInfo{
		Status: uint16(p.StatusCode),
	}
	var headers = make([]Field, 0, len(p.Header))
	for key, h := range p.Header {
		for _, value := range h {
			var field Field
			if !field.Key.SetData([]byte(key)) {
				return nil, fmt.Errorf("failed to set response header key %s", key)
			}
			if !field.Value.SetData([]byte(value)) {
				return nil, fmt.Errorf("failed to set response header value for %s", key)
			}
			headers = append(headers, field)
		}
	}
	if !info.Header.SetFields(headers) {
		return nil, fmt.Errorf("failed to set response headers for status code %d", p.StatusCode)
	}
	return info, nil
}

func (r *edgeComputing) initRequestHandler() error {
	_, err := wasi_snapshot_preview1.Instantiate(context.Background(), r.rt)
	if err != nil {
		return fmt.Errorf("failed to instantiate wasi_snapshot_preview1: %w", err)
	}
	ncdn := r.rt.NewHostModuleBuilder("ncdn")
	ncdn.NewFunctionBuilder().WithFunc(r.getRequestInfo).Export("get_request_info")
	ncdn.NewFunctionBuilder().WithFunc(r.getResponseInfo).Export("get_response_info")
	ncdn.NewFunctionBuilder().WithFunc(r.logOutput).Export("log_output")
	ncdn.NewFunctionBuilder().WithFunc(r.saveBufferPointer).Export("save_buffer_pointer")
	ncdn.NewFunctionBuilder().WithFunc(r.getBufferPointer).Export("get_buffer_pointer")
	_, err = ncdn.Instantiate(context.Background())
	if err != nil {
		return fmt.Errorf("failed to instantiate ncdn module: %w", err)
	}
	return nil
}

const hookOnRequest = "on_request"
const hookOnResponse = "on_response"
const hookOnFinish = "on_finish"

var hooks = []string{hookOnRequest, hookOnResponse, hookOnFinish}

func (r *edgeComputing) Register(ctx context.Context, method, path string, binary []byte) error {
	mod, err := r.rt.CompileModule(ctx, binary)
	if err != nil {
		return err
	}
	exported := mod.ExportedFunctions()
	hasLeastOne := false
	for _, hook := range hooks {
		if f, ok := exported[hook]; ok {
			params := f.ParamTypes()
			if len(params) != 0 {
				return fmt.Errorf("function %s must not have parameters", hook)
			}
			// return values are ignored, so we don't check them.
			hasLeastOne = true
		}
	}
	if !hasLeastOne {
		return fmt.Errorf("module must export at least one of the following functions: %v", hooks)
	}
	r.registerRW.Lock()
	defer r.registerRW.Unlock()
	if _, ok := r.compiled[method+" "+path]; ok {
		return fmt.Errorf("instance already registered for %s %s", method, path)
	}
	r.compiled[method+" "+path] = mod
	return nil
}

func (r *edgeComputing) Unregister(method, path string) error {
	r.registerRW.Lock()
	defer r.registerRW.Unlock()
	key := method + " " + path
	if _, ok := r.compiled[key]; !ok {
		return fmt.Errorf("no compiled instance found for %s %s", method, path)
	}
	delete(r.compiled, key)
	return nil
}

type requestIDKey struct{}

func (r *edgeComputing) saveBufferPointer(ctx context.Context, _ api.Module, pointer, size uint32) {
	key, ok := ctx.Value(requestIDKey{}).(uint64)
	if !ok {
		log.Printf("No request ID found in context")
		return
	}
	r.handlerRW.Lock()
	defer r.handlerRW.Unlock()
	handleCtx, exists := r.handleContext[key]
	if !exists {
		log.Printf("No handle context found for request ID %d", key)
		return
	}
	handleCtx.bufferPointer = pointer
	handleCtx.bufferSize = size
}

func (r *edgeComputing) getBufferPointer(ctx context.Context, mod api.Module, pointerToPointer uint32, pointerToSize uint32) {
	key, ok := ctx.Value(requestIDKey{}).(uint64)
	if !ok {
		log.Printf("No request ID found in context")
		return
	}
	r.handlerRW.RLock()
	handleCtx, exists := r.handleContext[key]
	r.handlerRW.RUnlock()
	if !exists {
		log.Printf("No handle context found for request ID %d", key)
		return
	}
	mod.Memory().WriteUint32Le(pointerToPointer, handleCtx.bufferPointer)
	mod.Memory().WriteUint32Le(pointerToSize, handleCtx.bufferSize)
}

func (r *edgeComputing) logOutput(c context.Context, mod api.Module, logLevel, pointer, size uint32) uint32 {
	level := LogLevel(logLevel)
	data, ok := mod.Memory().Read(pointer, size)
	if !ok {
		log.Printf("Failed to read log data from memory at pointer %d with size %d", pointer, size)
		return 0
	}
	message := string(data)
	fmt.Printf("[%s] %s\n", level.String(), message)
	return uint32(len(data))
}

type limitedWriter struct {
	buffer []byte
	offset int
}

func (w *limitedWriter) Write(p []byte) (n int, err error) {
	if w.offset+len(p) > len(w.buffer) {
		return 0, fmt.Errorf("buffer overflow: trying to write %d bytes to a buffer of size %d", len(p), len(w.buffer))
	}
	copy(w.buffer[w.offset:], p)
	w.offset += len(p)
	return len(p), nil
}

func (r *edgeComputing) getRequestInfo(c context.Context, mod api.Module, pointer, size uint32) uint32 {
	key, ok := c.Value(requestIDKey{}).(uint64)
	if !ok {
		log.Printf("No request ID found in context")
		return 0
	}
	r.handlerRW.RLock()
	handleCtx, exists := r.handleContext[key]
	r.handlerRW.RUnlock()
	if !exists {
		log.Printf("No handle context found for request ID %d", key)
		return 0
	}
	if handleCtx.req == nil {
		log.Printf("No request info found for request ID %d", key)
		return 0
	}
	buf, ok := mod.Memory().Read(pointer, size)
	if !ok {
		log.Printf("Failed to read request info from memory at pointer %d with size %d", pointer, size)
		return 0
	}
	limited := &limitedWriter{buffer: buf}
	if err := handleCtx.req.Write(limited); err != nil {
		log.Printf("Failed to encode request info for request ID %d: %v", key, err)
		return 0
	}
	return uint32(limited.offset)
}

func (r *edgeComputing) getResponseInfo(c context.Context, mod api.Module, pointer, size uint32) uint32 {
	key, ok := c.Value(requestIDKey{}).(uint64)
	if !ok {
		log.Printf("No request ID found in context")
		return 0
	}
	r.handlerRW.RLock()
	handleCtx, exists := r.handleContext[key]
	r.handlerRW.RUnlock()
	if !exists {
		log.Printf("No handle context found for request ID %d", key)
		return 0
	}
	if handleCtx.resp == nil {
		log.Printf("No response info found for request ID %d", key)
		return 0
	}
	buf, ok := mod.Memory().Read(pointer, size)
	if !ok {
		log.Printf("Failed to read response info from memory at pointer %d with size %d", pointer, size)
		return 0
	}
	limited := &limitedWriter{buffer: buf}
	if err := handleCtx.resp.Write(limited); err != nil {
		log.Printf("Failed to encode response info for request ID %d: %v", key, err)
		return 0
	}
	return uint32(limited.offset)
}

var ErrNoEdgeFunction = errors.New("no edge function registered for this request")

func (r *edgeComputing) StartRequest(ctx context.Context, p *http.Request) (uint64, error) {
	key := p.Method + " " + p.URL.Path
	r.registerRW.RLock()
	compiled, ok := r.compiled[key]
	r.registerRW.RUnlock()
	if !ok {
		return 0, fmt.Errorf("%w: %s %s", ErrNoEdgeFunction, p.Method, p.URL.Path)
	}
	conf := wazero.NewModuleConfig()
	mod, err := r.rt.InstantiateModule(ctx, compiled, conf)
	if err != nil {
		return 0, fmt.Errorf("failed to instantiate module for %s %s: %w", p.Method, p.URL.Path, err)
	}
	handleCtx := &HandleContext{
		mod: mod,
	}
	reqID := r.atomicCounter.Add(1)
	r.handlerRW.Lock()
	if _, exists := r.handleContext[reqID]; exists {
		r.handlerRW.Unlock()
		return 0, fmt.Errorf("request ID %d already exists", reqID)
	}
	r.handleContext[reqID] = handleCtx
	r.handlerRW.Unlock()

	return reqID, nil
}

func (r *edgeComputing) ProcessRequest(ctx context.Context, reqID uint64, p *http.Request) error {
	r.handlerRW.RLock()
	handleCtx, exists := r.handleContext[reqID]
	r.handlerRW.RUnlock()
	if !exists {
		return fmt.Errorf("no handle context found for request ID %d", reqID)
	}
	info, err := getRequestInfo(p)
	if err != nil {
		return fmt.Errorf("failed to get request info: %w", err)
	}
	handleCtx.req = info
	mod := handleCtx.mod
	if mod == nil {
		return fmt.Errorf("no module found for request ID %d", reqID)
	}
	f := mod.ExportedFunction(hookOnRequest)
	if f != nil {
		ctx := context.WithValue(ctx, requestIDKey{}, reqID)
		_, err := f.Call(ctx)
		if err != nil {
			return fmt.Errorf("failed to call on_request function: %w", err)
		}
	}
	return nil
}

func (r *edgeComputing) ProcessResponse(ctx context.Context, reqID uint64, resp *http.Response) error {
	r.handlerRW.RLock()
	handleCtx, exists := r.handleContext[reqID]
	r.handlerRW.RUnlock()
	if !exists {
		return fmt.Errorf("no handle context found for request ID %d", reqID)
	}
	if handleCtx.resp != nil {
		return fmt.Errorf("response already processed for request ID %d", reqID)
	}
	respInfo, err := getResponseInfo(resp)
	if err != nil {
		return fmt.Errorf("failed to get response info: %w", err)
	}
	handleCtx.resp = respInfo
	mod := handleCtx.mod
	f := mod.ExportedFunction(hookOnResponse)
	if f != nil {
		ctx := context.WithValue(ctx, requestIDKey{}, reqID)
		_, err := f.Call(ctx)
		if err != nil {
			return fmt.Errorf("failed to call on_response function: %w", err)
		}
	}
	return nil
}

func (r *edgeComputing) FinishRequest(ctx context.Context, reqID uint64) error {
	r.handlerRW.RLock()
	handleCtx, exists := r.handleContext[reqID]
	r.handlerRW.RUnlock()
	if !exists {
		return fmt.Errorf("no handle context found for request ID %d", reqID)
	}
	defer func() {
		r.handlerRW.Lock()
		defer r.handlerRW.Unlock()
		delete(r.handleContext, reqID)
	}()
	mod := handleCtx.mod
	ctx = context.WithValue(ctx, requestIDKey{}, reqID)
	var err error
	if f := mod.ExportedFunction(hookOnFinish); f != nil {
		_, err = f.Call(ctx)
	}
	err2 := mod.Close(ctx)
	return errors.Join(err, err2)
}
