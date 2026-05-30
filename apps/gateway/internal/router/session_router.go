package router

import (
	"cardwar/protocol"

	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/znet"
)

// SessionResponseRouter handles responses from SessionSvr (SessionGet, SessionReconnect).
// These have msgIDs outside the 1-1000 range that ResponseRouter covers.
type SessionResponseRouter struct {
	znet.BaseRouter
	GW *GatewayRef
}

func (r *SessionResponseRouter) Handle(request ziface.IRequest) {
	switch request.GetMsgID() {
	case protocol.MsgIdSessionGet: // 这个msgId是来自SessionSvr
		r.GW.HandleSessionGet(request)
	case protocol.MsgIdSessionReconnect:
		r.GW.HandleSessionGet(request)
	}
}
