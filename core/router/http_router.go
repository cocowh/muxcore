package router

import (
	"context"
	"net/http"
	"regexp"
	"strings"
	"sync"

	"github.com/cocowh/muxcore/pkg/logger"
)

// HandlerFunc HTTP处理函数类型
type HandlerFunc func(http.ResponseWriter, *http.Request)

// MiddlewareFunc 中间件函数类型
type MiddlewareFunc func(HandlerFunc) HandlerFunc

// Route 表示一个路由
type Route struct {
	Method      string
	Path        string
	Pattern     *regexp.Regexp
	Handler     HandlerFunc
	Middlewares []MiddlewareFunc
	Params      []string
}

// RouterConfig 路由器配置
type RouterConfig struct {
	CaseSensitive     bool
	StrictSlash       bool
	UseEncodedPath    bool
	SkipClean         bool
	NotFoundHandler   HandlerFunc
	MethodNotAllowed  HandlerFunc
	PanicHandler      func(http.ResponseWriter, *http.Request, interface{})
}

// HTTPRouter HTTP路由器
type HTTPRouter struct {
	routes          []*Route
	middlewares     []MiddlewareFunc
	config          *RouterConfig
	mutex           sync.RWMutex
	radixTree       *RadixTree
	methodTrees     map[string]*RadixTree
}

// 默认处理函数
func defaultNotFoundHandler(w http.ResponseWriter, r *http.Request) {
	http.NotFound(w, r)
}

func defaultMethodNotAllowedHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusMethodNotAllowed)
	w.Write([]byte("Method Not Allowed"))
}

func defaultPanicHandler(w http.ResponseWriter, r *http.Request, p interface{}) {
	logger.Error("Panic in HTTP handler: ", p)
	w.WriteHeader(http.StatusInternalServerError)
	w.Write([]byte("Internal Server Error"))
}

// NewHTTPRouter 创建HTTP路由器
func NewHTTPRouter() *HTTPRouter {
	return NewHTTPRouterWithConfig(&RouterConfig{
		CaseSensitive:    false,
		StrictSlash:      false,
		UseEncodedPath:   false,
		SkipClean:        false,
		NotFoundHandler:  defaultNotFoundHandler,
		MethodNotAllowed: defaultMethodNotAllowedHandler,
		PanicHandler:     defaultPanicHandler,
	})
}

// NewHTTPRouterWithConfig 使用配置创建HTTP路由器
func NewHTTPRouterWithConfig(config *RouterConfig) *HTTPRouter {
	return &HTTPRouter{
		routes:      make([]*Route, 0),
		middlewares: make([]MiddlewareFunc, 0),
		config:      config,
		radixTree:   NewRadixTree(),
		methodTrees: make(map[string]*RadixTree),
	}
}

// AddRoute 添加路由
func (r *HTTPRouter) AddRoute(method, path string, handler HandlerFunc) {
	r.AddRouteWithMiddleware(method, path, handler, nil)
}

// AddRouteWithMiddleware 添加带中间件的路由
func (r *HTTPRouter) AddRouteWithMiddleware(method, path string, handler HandlerFunc, middlewares []MiddlewareFunc) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	route := &Route{
		Method:      method,
		Path:        path,
		Handler:     handler,
		Middlewares: middlewares,
	}

	// 编译路径模式
	if pattern, params := r.compilePattern(path); pattern != nil {
		route.Pattern = pattern
		route.Params = params
	}

	r.routes = append(r.routes, route)

	// 添加到方法树
	if _, exists := r.methodTrees[method]; !exists {
		r.methodTrees[method] = NewRadixTree()
	}
	r.methodTrees[method].Insert(path, []string{path})

	logger.Info("Added route: ", method, " ", path)
}

// ServeHTTP 实现http.Handler接口
func (r *HTTPRouter) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	// 恢复panic
	defer func() {
		if p := recover(); p != nil {
			if r.config.PanicHandler != nil {
				r.config.PanicHandler(w, req, p)
			} else {
				defaultPanicHandler(w, req, p)
			}
		}
	}()

	// 查找匹配的路由
	route, params := r.matchRoute(req.Method, req.URL.Path)
	if route != nil {
		// 将参数添加到请求上下文
		if len(params) > 0 {
			ctx := req.Context()
			for key, value := range params {
				ctx = context.WithValue(ctx, key, value)
			}
			req = req.WithContext(ctx)
		}

		logger.Debug("Matched route: ", route.Method, " ", route.Path)
		handler := r.buildHandler(route)
		handler(w, req)
		return
	}

	// 未找到路由
	logger.Warn("No route found for: ", req.Method, " ", req.URL.Path)
	if r.config.NotFoundHandler != nil {
		r.config.NotFoundHandler(w, req)
	} else {
		http.NotFound(w, req)
	}
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

// Use 添加全局中间件
func (r *HTTPRouter) Use(middleware MiddlewareFunc) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.middlewares = append(r.middlewares, middleware)
}

// compilePattern 编译路径模式
func (r *HTTPRouter) compilePattern(path string) (*regexp.Regexp, []string) {
	// 简单的参数匹配实现
	if !strings.Contains(path, ":") && !strings.Contains(path, "*") {
		return nil, nil
	}

	var params []string
	pattern := path

	// 处理参数 :param
	paramRegex := regexp.MustCompile(`:([^/]+)`)
	matches := paramRegex.FindAllStringSubmatch(path, -1)
	for _, match := range matches {
		params = append(params, match[1])
		pattern = strings.Replace(pattern, match[0], "([^/]+)", 1)
	}

	// 处理通配符 *
	pattern = strings.Replace(pattern, "*", "(.*)", -1)

	compiled, err := regexp.Compile("^" + pattern + "$")
	if err != nil {
		logger.Error("Failed to compile pattern: ", err)
		return nil, nil
	}

	return compiled, params
}

// matchRoute 匹配路由
func (r *HTTPRouter) matchRoute(method, path string) (*Route, map[string]string) {
	// 首先尝试精确匹配
	for _, route := range r.routes {
		if route.Method == method && route.Path == path {
			return route, nil
		}
	}

	// 然后尝试模式匹配
	for _, route := range r.routes {
		if route.Method == method && route.Pattern != nil {
			if matches := route.Pattern.FindStringSubmatch(path); matches != nil {
				params := make(map[string]string)
				for i, param := range route.Params {
					if i+1 < len(matches) {
						params[param] = matches[i+1]
					}
				}
				return route, params
			}
		}
	}

	return nil, nil
}

// buildHandler 构建处理器链
func (r *HTTPRouter) buildHandler(route *Route) HandlerFunc {
	handler := route.Handler

	// 应用路由级中间件
	for i := len(route.Middlewares) - 1; i >= 0; i-- {
		handler = route.Middlewares[i](handler)
	}

	// 应用全局中间件
	for i := len(r.middlewares) - 1; i >= 0; i-- {
		handler = r.middlewares[i](handler)
	}

	return handler
}