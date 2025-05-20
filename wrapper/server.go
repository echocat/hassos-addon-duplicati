package main

import (
	"bufio"
	"bytes"
	"context"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"time"

	log "github.com/echocat/slf4g"
	"github.com/echocat/slf4g/level"
	sdk "github.com/echocat/slf4g/sdk/bridge"
	"github.com/tdewolff/minify/v2"
	"github.com/tdewolff/minify/v2/js"
)

const (
	upstreamPort = processPort
	serverPort   = 8080
)

func newServer(opt options) (srv *server, err error) {
	srv = &server{
		options: opt,
		logger:  log.GetLogger("server"),
	}
	srv.reverseProxy.Rewrite = srv.rewriteProxyRequest
	srv.reverseProxy.ErrorHandler = srv.handleProxyError
	srv.reverseProxy.ErrorLog = sdk.NewWrapper(srv.logger, level.Error)
	srv.reverseProxy.ModifyResponse = srv.interceptResponse
	srv.impl.Handler = http.HandlerFunc(srv.handleWrapper)
	srv.impl.Addr = fmt.Sprintf(":%d", serverPort)

	if srv.upstreamUrl, err = url.Parse(fmt.Sprintf("http://localhost:%d", upstreamPort)); err != nil {
		return nil, fmt.Errorf("cannot parse target url: %w", err)
	}

	if srv.listener, err = net.Listen("tcp", srv.impl.Addr); err != nil {
		return nil, fmt.Errorf("cannot listen to %s: %w", srv.impl.Addr, err)
	}

	return srv, nil
}

type server struct {
	options      options
	logger       log.Logger
	reverseProxy httputil.ReverseProxy
	upstreamUrl  *url.URL

	impl     http.Server
	listener net.Listener
}

func (srv *server) serve() error {
	srv.logger.
		With("addr", srv.impl.Addr).
		Info("wrapper listening...")
	err := srv.impl.Serve(srv.listener)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func (srv *server) shutdown() error {
	return srv.impl.Shutdown(context.Background())
}

func (srv *server) Close() error {
	return srv.shutdown()
}

func (srv *server) handleWrapper(ow http.ResponseWriter, r *http.Request) {
	rw := &httpResponseWriter{ow, http.StatusOK}
	started := time.Now()
	defer func() {
		srv.logger.With("uri", r.RequestURI).
			With("method", r.Method).
			With("remote", r.RemoteAddr).
			With("duration", time.Now().Sub(started).Truncate(time.Millisecond)).
			With("status", rw.status).
			Debug("request")
	}()
	if r.URL == nil {
		http.Error(rw, "Bad Request", http.StatusBadRequest)
		return
	}
	srv.handle(rw, r)
}

func (srv *server) handle(rw http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/api/v1/auth/refresh":
		srv.handlerAuthRefresh(rw, r)
	default:
		srv.reverseProxy.ServeHTTP(rw, r)
	}
}

func (srv *server) handlerAuthRefresh(rw http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET", "POST":
		_, _ = fmt.Fprintf(rw, `{"AccessToken":null}`)
	default:
		http.Error(rw, "Bad Request", http.StatusMethodNotAllowed)
	}
}

func (srv *server) rewriteProxyRequest(pr *httputil.ProxyRequest) {
	pr.SetURL(srv.upstreamUrl)
	pr.SetXForwarded()
	pr.Out.Host = pr.In.Host
	pr.Out.Header.Set("Authorization", "PreAuth "+srv.options.webservicePreAuthTokens)
}

func (srv *server) handleProxyError(rw http.ResponseWriter, _ *http.Request, err error) {
	srv.logger.WithError(err).Error()
	http.Error(rw, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
}

var (
	//go:embed server_rewritePrefixJs.js
	rewritePrefixJs string

	rewritePrefixJsCompressed = func(in string) string {
		m := minify.New()
		var buf strings.Builder
		if err := js.Minify(m, &buf, strings.NewReader(in), nil); err != nil {
			panic(err)
		}
		return buf.String()
	}(rewritePrefixJs)
)

func (srv *server) fixJsRequestsScript(ingressPath string) string {
	return `<script>const __wrapperPrefix__="` + ingressPath + `";` + rewritePrefixJsCompressed + `</script>`
}

func (srv *server) interceptResponse(rsp *http.Response) error {
	if rsp.Request.Method != http.MethodGet {
		return nil
	}
	ingressPath := rsp.Request.Header.Get("X-Ingress-Path")
	ingressPath = strings.TrimSuffix(ingressPath, "/")
	if ingressPath == "" {
		return nil
	}
	if !strings.HasPrefix(rsp.Header.Get("Content-Type"), "text/html") {
		return nil
	}

	b, err := io.ReadAll(rsp.Body)
	if err != nil {
		return fmt.Errorf("cannot buffer HTML body: %w", err)
	}
	if err := rsp.Body.Close(); err != nil {
		return fmt.Errorf("cannot close upstream response body: %w", err)
	}

	if bytes.Contains(b, []byte(`<base href="`)) {
		b = bytes.Replace(b, []byte(`<base href="`), []byte(srv.fixJsRequestsScript(ingressPath)+`<base href="`+ingressPath), 1)
	}
	rsp.Body = io.NopCloser(bytes.NewReader(b))
	rsp.ContentLength = int64(len(b))
	rsp.Header.Set("Content-Length", strconv.Itoa(len(b)))
	return nil
}

type httpResponseWriter struct {
	http.ResponseWriter
	status int
}

func (hrw *httpResponseWriter) WriteHeader(statusCode int) {
	hrw.status = statusCode
	hrw.ResponseWriter.WriteHeader(statusCode)
}

func (hrw *httpResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := hrw.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("can't switch protocols using non-Hijacker ResponseWriter type %T", hrw.ResponseWriter)
	}
	return h.Hijack()
}
