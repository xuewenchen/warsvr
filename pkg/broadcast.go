package pkg

import (
	"cardwar/protocol/pb"

	"github.com/aceld/zinx/ziface"
	"google.golang.org/protobuf/proto"
)

// Broadcaster sends messages to all connected Gateways. It filters connections
// by the "conn_type" property set by backends when a Gateway registers itself
// (via MsgIdGatewayRegister on connect).
type Broadcaster struct {
	sendToAll func(msgID uint32, data []byte)
}

// NewBroadcaster creates a Broadcaster that sends to all Gateway connections
// registered on the given server (via MsgIdGatewayRegister).
func NewBroadcaster(s ziface.IServer) *Broadcaster {
	return &Broadcaster{
		sendToAll: func(msgID uint32, data []byte) {
			s.GetConnMgr().Range(func(connID uint64, conn ziface.IConnection, extra interface{}) error {
				if tp, _ := conn.GetProperty("conn_type"); tp == "gateway" {
					conn.SendMsg(msgID, data)
				}
				return nil
			}, nil)
		},
	}
}

// ToAll broadcasts a push message to every client on every known Gateway.
func (b *Broadcaster) ToAll(msgID uint32, payload []byte) {
	data, _ := proto.Marshal(&pb.Envelope{ConnId: 0, Data: payload})
	b.sendToAll(msgID, data)
}

// ToPlayer sends a push message to a specific player across all known Gateways.
// Each Gateway's ResponseRouter checks its own PlayerConns and delivers only if
// the target is connected locally.
func (b *Broadcaster) ToPlayer(msgID uint32, targetPlayerID string, payload []byte) {
	env := &pb.Envelope{
		ConnId:   0,
		Data:     payload,
		ConnTags: map[string]string{"target_player_id": targetPlayerID},
	}
	envData, _ := proto.Marshal(env)
	b.sendToAll(msgID, envData)
}

// ToConn sends a push message to a specific client connection on a specific
// Gateway connection. Used for sender confirmations and direct responses.
func (b *Broadcaster) ToConn(msgID uint32, connID uint64, payload []byte, gatewayConn ziface.IConnection) {
	env, _ := proto.Marshal(&pb.Envelope{ConnId: connID, Data: payload})
	gatewayConn.SendMsg(msgID, env)
}
