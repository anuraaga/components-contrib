package wasm

import (
	"bytes"
	"io"
	"net"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/valyala/fasthttp"

	"github.com/dapr/components-contrib/metadata"
	"github.com/dapr/components-contrib/middleware"
	"github.com/dapr/components-contrib/middleware/http/wasm/basic"
	"github.com/dapr/components-contrib/middleware/http/wasm/flexible"
	"github.com/dapr/components-contrib/middleware/http/wasm/httpwasm"
	"github.com/dapr/components-contrib/middleware/http/wasm/internal/test"
)

var (
	requestsPerConn = 1
	clientsCount    = runtime.NumCPU()
	fakeResponse    = []byte("Hello, world!")
	getRequest      = "GET /v1.0/hi?baz HTTP/1.1\r\nHost: google.com\r\nUser-Agent: aaa/bbb/ccc/ddd/eee Firefox Chrome MSIE Opera\r\n" +
		"Referer: http://example.com/aaa?bbb=ccc\r\nCookie: foo=bar; baz=baraz; aa=aakslsdweriwereowriewroire\r\n\r\n"
)

func Benchmark_native(b *testing.B) {
	benchmarkServerGet(b, func(next fasthttp.RequestHandler) fasthttp.RequestHandler {
		return func(ctx *fasthttp.RequestCtx) {
			if string(ctx.Path()) == "/v1.0/hi" {
				ctx.Request.URI().SetPath("/v1.0/hello")
			}
			next(ctx)
		}
	})
}

func Benchmark_httpwasm_tinygo(b *testing.B) {
	path := "./httpwasm/example/example.wasm"
	benchmark_httpwasm(b, path)
}

func Benchmark_httpwasm_wat(b *testing.B) {
	path := "./httpwasm/internal/testdata/rewrite.wasm"
	benchmark_httpwasm(b, path)
}

func benchmark_httpwasm(b *testing.B, path string) {
	md := metadata.Base{Properties: map[string]string{
		"path":     path,
		"poolSize": "1",
	}}
	l := test.NewLogger()
	handlerFn, err := httpwasm.NewMiddleware(l).GetHandler(middleware.Metadata{Base: md})
	if err != nil {
		b.Fatal(err)
	}
	benchmarkServerGet(b, handlerFn)
}

func Benchmark_wapc(b *testing.B) {
	md := metadata.Base{Properties: map[string]string{
		"path":     "./basic/example/example.wasm",
		"poolSize": "1",
	}}
	l := test.NewLogger()
	handlerFn, err := basic.NewMiddleware(l).GetHandler(middleware.Metadata{Base: md})
	if err != nil {
		b.Fatal(err)
	}
	benchmarkServerGet(b, handlerFn)
}

func Benchmark_wapcflexible(b *testing.B) {
	md := metadata.Base{Properties: map[string]string{
		"path":     "./flexible/example/example.wasm",
		"poolSize": "1",
	}}
	l := test.NewLogger()
	handlerFn, err := flexible.NewMiddleware(l).GetHandler(middleware.Metadata{Base: md})
	if err != nil {
		b.Fatal(err)
	}
	benchmarkServerGet(b, handlerFn)
}

// The below is a port of code in fasthttp benchmarks.
type fakeServerConn struct {
	net.TCPConn
	ln            *fakeListener
	requestsCount int
	pos           int
	closed        uint32
}

func (c *fakeServerConn) Read(b []byte) (int, error) {
	nn := 0
	reqLen := len(c.ln.request)
	for len(b) > 0 {
		if c.requestsCount == 0 {
			if nn == 0 {
				return 0, io.EOF
			}
			return nn, nil
		}
		pos := c.pos % reqLen
		n := copy(b, c.ln.request[pos:])
		b = b[n:]
		nn += n
		c.pos += n
		if n+pos == reqLen {
			c.requestsCount--
		}
	}
	return nn, nil
}

func (c *fakeServerConn) Write(b []byte) (int, error) {
	return len(b), nil
}

var fakeAddr = net.TCPAddr{
	IP:   []byte{1, 2, 3, 4},
	Port: 12345,
}

func (c *fakeServerConn) RemoteAddr() net.Addr {
	return &fakeAddr
}

func (c *fakeServerConn) Close() error {
	if atomic.AddUint32(&c.closed, 1) == 1 {
		c.ln.ch <- c
	}
	return nil
}

func (c *fakeServerConn) SetReadDeadline(t time.Time) error {
	return nil
}

func (c *fakeServerConn) SetWriteDeadline(t time.Time) error {
	return nil
}

type fakeListener struct {
	lock            sync.Mutex
	requestsCount   int
	requestsPerConn int
	request         []byte
	ch              chan *fakeServerConn
	done            chan struct{}
	closed          bool
}

func (ln *fakeListener) Accept() (net.Conn, error) {
	ln.lock.Lock()
	if ln.requestsCount == 0 {
		ln.lock.Unlock()
		for len(ln.ch) < cap(ln.ch) {
			time.Sleep(10 * time.Millisecond)
		}
		ln.lock.Lock()
		if !ln.closed {
			close(ln.done)
			ln.closed = true
		}
		ln.lock.Unlock()
		return nil, io.EOF
	}
	requestsCount := ln.requestsPerConn
	if requestsCount > ln.requestsCount {
		requestsCount = ln.requestsCount
	}
	ln.requestsCount -= requestsCount
	ln.lock.Unlock()

	c := <-ln.ch
	c.requestsCount = requestsCount
	c.closed = 0
	c.pos = 0

	return c, nil
}

func (ln *fakeListener) Close() error {
	return nil
}

func (ln *fakeListener) Addr() net.Addr {
	return &fakeAddr
}

func newFakeListener(requestsCount, clientsCount, requestsPerConn int, request string) *fakeListener {
	ln := &fakeListener{
		requestsCount:   requestsCount,
		requestsPerConn: requestsPerConn,
		request:         []byte(request),
		ch:              make(chan *fakeServerConn, clientsCount),
		done:            make(chan struct{}),
	}
	for i := 0; i < clientsCount; i++ {
		ln.ch <- &fakeServerConn{
			ln: ln,
		}
	}
	return ln
}

func benchmarkServerGet(b *testing.B, mw func(fasthttp.RequestHandler) fasthttp.RequestHandler) {
	ch := make(chan struct{}, b.N)
	s := &fasthttp.Server{
		Handler: mw(func(ctx *fasthttp.RequestCtx) {
			if !ctx.IsGet() {
				b.Fatalf("Unexpected request method: %q", ctx.Method())
			}
			if !bytes.Equal(ctx.Path(), []byte("/v1.0/hello")) {
				b.Fatalf("Expected wasm to rewrite path: %s", ctx.Path())
			}
			ctx.Success("text/plain", fakeResponse)
			if requestsPerConn == 1 {
				ctx.SetConnectionClose()
			}
			registerServedRequest(b, ch)
		}),
		Concurrency: 16 * clientsCount,
	}
	benchmarkServer(b, s, clientsCount, requestsPerConn, getRequest)
	verifyRequestsServed(b, ch)
}

func registerServedRequest(b *testing.B, ch chan<- struct{}) {
	select {
	case ch <- struct{}{}:
	default:
		b.Fatalf("More than %d requests served", cap(ch))
	}
}

func verifyRequestsServed(b *testing.B, ch <-chan struct{}) {
	requestsServed := 0
	for len(ch) > 0 {
		<-ch
		requestsServed++
	}
	requestsSent := b.N
	for requestsServed < requestsSent {
		select {
		case <-ch:
			requestsServed++
		case <-time.After(100 * time.Millisecond):
			b.Fatalf("%s, unexpected number of requests served %d. Expected %d", b.Name(), requestsServed, requestsSent)
		}
	}
}

type realServer interface {
	Serve(ln net.Listener) error
}

func benchmarkServer(b *testing.B, s realServer, clientsCount, requestsPerConn int, request string) {
	ln := newFakeListener(b.N, clientsCount, requestsPerConn, request)
	ch := make(chan struct{})
	go func() {
		s.Serve(ln) //nolint:errcheck
		ch <- struct{}{}
	}()

	<-ln.done

	select {
	case <-ch:
	case <-time.After(10 * time.Second):
		b.Fatalf("Server.Serve() didn't stop")
	}
}
