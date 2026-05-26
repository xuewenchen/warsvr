package router

import (
	"cardwar/common"
	"encoding/json"
	"fmt"

	"github.com/aceld/zinx/ziface"
	"github.com/aceld/zinx/znet"
)

type LoginRspRouter struct {
	znet.BaseRouter
}

func (r *LoginRspRouter) Handle(request ziface.IRequest) {
	var rsp common.LoginRspMsg
	if err := json.Unmarshal(request.GetData(), &rsp); err != nil {
		fmt.Println("LoginRsp parse error:", err)
		return
	}
	fmt.Printf("Login response: player=%s success=%v msg=%s\n", rsp.PlayerID, rsp.Success, rsp.Message)
}
