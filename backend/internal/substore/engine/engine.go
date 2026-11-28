// Package engine 在 goja 纯 Go JS 引擎中加载 Sub-Store 转换核心 bundle，
// 提供 Parse(raw) 与 Produce(proxies, target, opts) 两个同步入口。
// bundle 完全自包含：不依赖 Node API / Buffer / TextEncoder / process。
package engine

import (
	_ "embed"
	"fmt"
	"sync"

	"github.com/dop251/goja"
)

//go:embed bundle/substore.js
var bundleSrc string

// Engine 持有一个隔离的 goja 运行时。goja 运行时不是并发安全的，
// 因此通过 pool 复用，保证同一时刻只有一个 goroutine 使用某个实例。
type Engine struct {
	vm     *goja.Runtime
	parse  goja.Callable
	produce goja.Callable
}

var (
	engineOnce sync.Once
	engineInst *Engine
	engineErr  error
)

// New 创建并初始化一个转换引擎。
func New() (*Engine, error) {
	vm := goja.New()
	// bundle 只需要 console；给一个静默实现避免依赖宿主。
	vm.Set("console", map[string]interface{}{
		"log":   func(...interface{}) {},
		"info":  func(...interface{}) {},
		"warn":  func(...interface{}) {},
		"error": func(...interface{}) {},
		"debug": func(...interface{}) {},
	})

	if _, err := vm.RunString(bundleSrc); err != nil {
		return nil, fmt.Errorf("load substore bundle: %w", err)
	}

	core := vm.Get("SubStoreCore")
	if core == nil || goja.IsUndefined(core) || goja.IsNull(core) {
		return nil, fmt.Errorf("SubStoreCore global not found")
	}
	obj := core.ToObject(vm)

	parseFn, ok := goja.AssertFunction(obj.Get("parse"))
	if !ok {
		return nil, fmt.Errorf("SubStoreCore.parse is not callable")
	}
	produceFn, ok := goja.AssertFunction(obj.Get("produce"))
	if !ok {
		return nil, fmt.Errorf("SubStoreCore.produce is not callable")
	}

	return &Engine{vm: vm, parse: parseFn, produce: produceFn}, nil
}

// Default 返回进程级共享引擎（懒初始化）。调用方如需并发请用 Pool。
func Default() (*Engine, error) {
	engineOnce.Do(func() {
		engineInst, engineErr = New()
	})
	return engineInst, engineErr
}

// Parse 把订阅原文解析为代理数组，返回 goja.Value（可交给 Produce）。
func (e *Engine) Parse(raw string) (goja.Value, error) {
	v, err := e.parse(goja.Undefined(), vmString(e.vm, raw))
	if err != nil {
		return nil, fmt.Errorf("substore parse: %w", err)
	}
	return v, nil
}

// Produce 把代理数组转换为目标客户端格式。proxies 应为 Parse 返回的 goja.Value。
func (e *Engine) Produce(proxies goja.Value, target string, opts map[string]interface{}) (string, error) {
	var optVal goja.Value = goja.Undefined()
	if opts != nil {
		optVal = e.vm.ToValue(opts)
	}
	v, err := e.produce(goja.Undefined(), proxies, vmString(e.vm, target), optVal)
	if err != nil {
		return "", fmt.Errorf("substore produce(%s): %w", target, err)
	}
	if v == nil || goja.IsUndefined(v) || goja.IsNull(v) {
		return "", nil
	}
	return v.String(), nil
}

// Convert 一步完成 parse + produce。raw 为订阅原文，target 为目标平台。
func (e *Engine) Convert(raw, target string, opts map[string]interface{}) (string, error) {
	proxies, err := e.Parse(raw)
	if err != nil {
		return "", err
	}
	return e.Produce(proxies, target, opts)
}

func vmString(vm *goja.Runtime, s string) goja.Value {
	return vm.ToValue(s)
}

// Pool 是并发安全的引擎池。goja.Runtime 非并发安全，每个 goroutine
// 从池中借出一个独立实例，用完归还。
type Pool struct {
	pool sync.Pool
}

// NewPool 创建一个引擎池。首次使用时惰性创建实例。
func NewPool() *Pool {
	return &Pool{
		pool: sync.Pool{
			New: func() interface{} {
				e, err := New()
				if err != nil {
					panic(fmt.Sprintf("create substore engine: %v", err))
				}
				return e
			},
		},
	}
}

// With 借出一个引擎执行 fn，用完自动归还。fn 内不可再调用 With（会死锁在极端情况）。
func (p *Pool) With(fn func(*Engine) error) error {
	e := p.pool.Get().(*Engine)
	defer p.pool.Put(e)
	return fn(e)
}

// defaultPool 进程级共享池。
var defaultPool = NewPool()

// DefaultPool 返回进程级共享引擎池。
func DefaultPool() *Pool {
	return defaultPool
}
