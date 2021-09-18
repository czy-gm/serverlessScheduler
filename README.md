# serverlessScheduler

天池首届云原生编程挑战赛亚军-复赛方案
排名（2/11060） 正常赛（0.98752） 加时赛（0.95105）

[赛题链接](https://tianchi.aliyun.com/competition/entrance/231793/introduction)

主体代码在scheduler，文件结构如下，方案及PPT在ducuments

```.
├── Dockerfile
├── client
│   └── client.go
├── config
│   ├── config.go
│   ├── config.json
│   ├── dev
│   │   ├── config.json
│   │   └── log.xml
│   └── log.xml
├── core
│   ├── node.go			节点信息
│   └── router.go		主要文件，策略实现
├── main 
├── main.go			程序入口
├── model
│   └── request.go			
├── proto
│   ├── scheduler.pb.go	        grpc生成文件
│   └── scheduler.proto	        scheduler接口描述文件
├── server
│   └── server.go		scheduler接口
└── utils
    ├── env
    │   └── env.go				
    ├── global
    │   └── global.go
    └── logger
        ├── cflog.go
        ├── logger.go
        ├── seelog
        │   └── seelog.go
        ├── static.go
        └── stdlog.go
