package pkg

import (
	"cardwar/protocol/pb"
	"strconv"

	"github.com/aceld/zinx/ziface"
	"google.golang.org/protobuf/proto"
)

// Broadcaster sends push messages to Gateway-connected clients.
type Broadcaster interface {
	ToAll(msgID uint32, payload []byte)
	ToPlayer(msgID uint32, targetPlayerID int64, payload []byte)
	ToConn(msgID uint32, connID uint64, payload []byte, gatewayConn ziface.IConnection)
}

type broadcaster struct {
	sendToAll func(msgID uint32, data []byte)
}

// NewGateWayBroadcaster creates a Broadcaster that sends to all Gateway connections
// registered on the given server (via MsgIdServiceIdentity).
func NewGateWayBroadcaster(s ziface.IServer) Broadcaster {
	return &broadcaster{
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

func (b *broadcaster) ToAll(msgID uint32, payload []byte) {
	data, _ := proto.Marshal(&pb.Envelope{ConnId: 0, Data: payload})
	b.sendToAll(msgID, data)
}

func (b *broadcaster) ToPlayer(msgID uint32, targetPlayerID int64, payload []byte) {
	env := &pb.Envelope{
		ConnId:   0,
		Data:     payload,
		ConnTags: map[string]string{"target_player_id": strconv.FormatInt(targetPlayerID, 10)},
	}
	envData, _ := proto.Marshal(env)
	b.sendToAll(msgID, envData)
}

func (b *broadcaster) ToConn(msgID uint32, connID uint64, payload []byte, gatewayConn ziface.IConnection) {
	env, _ := proto.Marshal(&pb.Envelope{ConnId: connID, Data: payload})
	gatewayConn.SendMsg(msgID, env)
}
