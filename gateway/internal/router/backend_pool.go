package router

import "github.com/aceld/zinx/ziface"

// BackendPool routes a key to one of several backend connections.
type BackendPool interface {
	Route(key string) ziface.IConnection
}

// BackendRouterConfig pairs a message ID with a router to register on backend connections.
type BackendRouterConfig struct {
	MsgID  uint32
	Router ziface.IRouter
}
