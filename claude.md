
# 终端1：启动 ChatSvr
cd D:/Work/warsvr && go run chatsvr/cmd/main.go -conf=config.yml

# 终端2：启动 Gateway
cd D:/Work/warsvr && go run gateway/cmd/main.go -conf=config.yml

# 终端3：启动客户端（可起多个，用不同 playerID）
go run client/cmd/main.go player1
go run client/cmd/main.go player2