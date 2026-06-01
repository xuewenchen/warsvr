package corouter

import (
	"cardwar/pkg/connkey"
	"cardwar/protocol"
	"cardwar/protocol/pb"

	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/znet"
	"google.golang.org/protobuf/proto"
)

// ServiceIdentityRouter handles service identity messages sent by dialing services.
// It sets the conn_type property on the connection to the caller's identity string
// (e.g. "gateway"), which downstream code like NewGateWayBroadcaster uses for filtering.
// It also replies with a ServiceHello announcing which msgIDs this backend handles and
// sends, enabling the caller to dynamically build its routing table.
type ServiceIdentityRouter struct {
	znet.BaseRouter
	Service       string   // this backend's service name, e.g. "chatsvr"
	ForwardMsgIDs []uint32 // msgIDs this backend handles (forward)
	SendMsgIDs    []uint32 // msgIDs this backend may send (response/push)
}

func (r *ServiceIdentityRouter) Handle(request ziface.IRequest) {
	identity := string(request.GetData())
	request.GetConnection().SetProperty(connkey.PropConnType, identity)

	if len(r.ForwardMsgIDs) == 0 && len(r.SendMsgIDs) == 0 {
		return
	}
	hello, _ := proto.Marshal(&pb.ServiceHello{
		Service:    r.Service,
		MsgIds:     r.ForwardMsgIDs,
		SendMsgIds: r.SendMsgIDs,
	})
	request.GetConnection().SendMsg(protocol.MsgIdServiceHello, hello)
}
