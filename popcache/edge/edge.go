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
	mod         api.Module
	req         *RequestInfo
	encodedReq  []byte
	resp        *ResponseInfo
	encodedResp []byte
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

func getRequestInfo(r *http.Request) (*RequestInfo, error) {
	info := &RequestInfo{}
	remoteAddrParsed, err := netip.ParseAddr(r.RemoteAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse remote address %s: %w", r.RemoteAddr, err)
	}
	info.RemoteAddr = remoteAddrParsed.As16()
	switch r.Method {
	case http.MethodGet:
		info.Method.Method = Method_Get
	case http.MethodPost:
		info.Method.Method = Method_Post
	case http.MethodPut:
		info.Method.Method = Method_Put
	default:
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
		for k, v := range q {
			var f Field
			if !f.Key.SetData([]byte(k)) {
				return nil, fmt.Errorf("failed to set query key %s", k)
			}
			if !f.Value.SetData([]byte(v[0])) {
				return nil, fmt.Errorf("failed to set query value for %s", k)
			}
			query = append(query, f)
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
	ncdn := r.rt.NewHostModuleBuilder("ncdn")
	ncdn.NewFunctionBuilder().WithFunc(r.getRequestInfo).Export("get_request_info")
	ncdn.NewFunctionBuilder().WithFunc(r.getResponseInfo).Export("get_response_info")
	_, err := ncdn.Instantiate(context.Background())
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
	if handleCtx.encodedReq == nil {
		encoded, err := handleCtx.req.Encode()
		if err != nil {
			log.Printf("Failed to marshal request info for request ID %d: %v", key, err)
			return 0
		}
		handleCtx.encodedReq = encoded
	}
	length := len(handleCtx.encodedReq)
	if length >= int(size) {
		length = int(size)
	}
	if !mod.Memory().Write(pointer, handleCtx.encodedReq[:length]) {
		log.Printf("Failed to write request info to module memory for request ID %d", key)
		return 0
	}
	return uint32(length)
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
	if handleCtx.encodedResp == nil {
		encoded, err := handleCtx.resp.Encode()
		if err != nil {
			log.Printf("Failed to marshal response info for request ID %d: %v", key, err)
			return 0
		}
		handleCtx.encodedResp = encoded
	}
	length := len(handleCtx.encodedResp)
	if length >= int(size) {
		length = int(size)
	}
	if !mod.Memory().Write(pointer, handleCtx.encodedResp[:length]) {
		log.Printf("Failed to write response info to module memory for request ID %d", key)
		return 0
	}
	return uint32(length)
}

var NoEdgeFunctionError = errors.New("no edge function registered for this request")

func (r *edgeComputing) StartRequest(ctx context.Context, p *http.Request) (uint64, error) {
	key := p.Method + " " + p.URL.Path
	r.registerRW.RLock()
	compiled, ok := r.compiled[key]
	r.registerRW.RUnlock()
	if !ok {
		return 0, fmt.Errorf("%w: %s %s", NoEdgeFunctionError, p.Method, p.URL.Path)
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
	mod := handleCtx.mod
	ctx = context.WithValue(ctx, requestIDKey{}, reqID)
	var err error
	if f := mod.ExportedFunction(hookOnFinish); f != nil {
		_, err = f.Call(ctx)
	}
	err2 := mod.Close(ctx)
	r.handlerRW.Lock()
	defer r.handlerRW.Unlock()
	delete(r.handleContext, reqID)
	return errors.Join(err, err2)
}
