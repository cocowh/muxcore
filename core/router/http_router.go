package router

import (
	"net/http"
	"sync"

	"github.com/cocowh/muxcore/pkg/logger"
)

// HandlerFunc HTTP处理函数类型
type HandlerFunc func(http.ResponseWriter, *http.Request)

// Route 表示一个路由
type Route struct {
	Method  string
	Path    string
	Handler HandlerFunc
}

// HTTPRouter HTTP路由器
type HTTPRouter struct {
	routes     []Route
	mutex      sync.RWMutex
}

// NewHTTPRouter 创建HTTP路由器
func NewHTTPRouter() *HTTPRouter {
	return &HTTPRouter{
		routes:  make([]Route, 0),
	}
}

// AddRoute 添加路由
func (r *HTTPRouter) AddRoute(method, path string, handler HandlerFunc) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.routes = append(r.routes, Route{
		Method:  method,
		Path:    path,
		Handler: handler,
	})

	logger.Info("Added route: ", method, " ", path)
}

// ServeHTTP 实现http.Handler接口
func (r *HTTPRouter) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	// 查找匹配的路由
	for _, route := range r.routes {
		if route.Method == req.Method && route.Path == req.URL.Path {
			logger.Debug("Matched route: ", route.Method, " ", route.Path)
			route.Handler(w, req)
			return
		}
	}

	// 未找到路由
	logger.Warn("No route found for: ", req.Method, " ", req.URL.Path)
	http.NotFound(w, req)
}

// GET 添加GET路由
func (r *HTTPRouter) GET(path string, handler HandlerFunc) {
	r.AddRoute("GET", path, handler)
}

// POST 添加POST路由
func (r *HTTPRouter) POST(path string, handler HandlerFunc) {
	r.AddRoute("POST", path, handler)
}

// PUT 添加PUT路由
func (r *HTTPRouter) PUT(path string, handler HandlerFunc) {
	r.AddRoute("PUT", path, handler)
}

// DELETE 添加DELETE路由
func (r *HTTPRouter) DELETE(path string, handler HandlerFunc) {
	r.AddRoute("DELETE", path, handler)
}