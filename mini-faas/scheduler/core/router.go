package core

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	uuid "github.com/satori/go.uuid"

	nsPb "aliyun/serverless/mini-faas/nodeservice/proto"
	rmPb "aliyun/serverless/mini-faas/resourcemanager/proto"
	pb "aliyun/serverless/mini-faas/scheduler/proto"

	cmap "github.com/orcaman/concurrent-map"
	"github.com/pkg/errors"
)

var cpuIntensiveType map[string]int	      //cpu密集型函数
var memIntensiveType map[string]int       //内存密集型函数
var shortTimeType    cmap.ConcurrentMap   //执行时间较短型函数
var modifyMap        cmap.ConcurrentMap   //需要修改容器的函数
var lock sync.Mutex                       //获取node的锁

const (
	 MB  = int64(1024 * 1024)
	 GB  = 1024 * MB
	 MS  = int64(1000000)
	 S   = 1000 * MS
	 initNodeNum    = 10                      //初始的node数量
	 statDuration   = 500 * time.Millisecond  //getStat时间周期
	 memUsagePct    = 0.3					  //判定mem-intensive类型的内存比例
	 cpuUsagePct    = 10.0					  //判定cpu-intensive类型的cpu使用率
	 shortTimeLine  = 50 * MS   		  	  //判定短时间类型的执行时间线
	 limitTimeLine  = 65 * MS   		  	  //超出短时间类型的执行时间线
	 cpuRatio       = 0.67					  //cpu的使用率，判定cpu-intensive类型
	 modifyLine     = 0.9                     //cpu-intensive类型需要调整容器的cpu使用率
	 inactiveTime   = 5 * 60 * S              //判定容器不活动的时间线
)


type ContainerInfo struct {
	sync.Mutex
	id         string
	address    string
	port       int64
	nodeId     string
	requests   int
	funcName   string
	usedMem    int64
	totalMem   int64
	handler    string
	accountId  string
	activeTime int64
	modify     bool
	deleting   bool
}

type Router struct {
	nodeMap      		cmap.ConcurrentMap // node_id -> NodeInfo
	functionMap  		cmap.ConcurrentMap // function_name -> ContainerMap (container_id -> ContainerInfo)
	containerMap 		cmap.ConcurrentMap // container_id -> containerInfo
	shortFuncRunning 	cmap.ConcurrentMap // container_id -> int
	ctCreating   		cmap.ConcurrentMap // function_name -> num
	rmClient     		rmPb.ResourceManagerClient
}

func NewRouter(rmClient rmPb.ResourceManagerClient) *Router {
	return &Router{
		nodeMap:      		cmap.New(),
		functionMap:  		cmap.New(),
		containerMap: 		cmap.New(),
		shortFuncRunning: 	cmap.New(),
		ctCreating:   		cmap.New(),
		rmClient:     		rmClient,
	}
}

func (r *Router) Start() {
	go r.getStatus()
	//go r.printNodeInfo()
	go r.releaseInactiveContainers()
	cpuIntensiveType = make(map[string]int)
	memIntensiveType = make(map[string]int)
	shortTimeType    = cmap.New()
	modifyMap        = cmap.New()
}

func (r *Router) AcquireContainer(req *pb.AcquireContainerRequest) (*pb.AcquireContainerReply, error) {
	var res *ContainerInfo
	funcName := req.FunctionName

	r.functionMap.SetIfAbsent(funcName, cmap.New())
	fmObj, _ := r.functionMap.Get(funcName)
	containerMap := fmObj.(cmap.ConcurrentMap)

	res = r.getExistedContainer(containerMap, funcName)

	//如果当前有短时间函数运行，则非短时间的cpu-intensive和mem-intensive类型函数等待其运行完，或者等待超时后继续运行
	if shortTimeType.Has(funcName) && res != nil {
		r.shortFuncRunning.SetIfAbsent(res.id, 1)
	} else if (memIntensiveType[funcName] == 1 || cpuIntensiveType[funcName] == 1) && !shortTimeType.Has(funcName) && res != nil && !r.shortFuncRunning.IsEmpty() {
		fmt.Printf("[INFO] req:%s, func{%s} wait for short-time func to finish or time out\n", req.RequestId, funcName)
		now := time.Now()
		for !r.shortFuncRunning.IsEmpty() && time.Since(now).Nanoseconds() <= shortTimeLine {
			time.Sleep(3 * time.Millisecond)
		}
		fmt.Printf("[INFO] req:%s, func{%s} wait is over.waiting time is %d\n", req.RequestId, funcName, time.Since(now).Nanoseconds())
	}

	if res == nil {
		lock.Lock()
		r.ctCreating.SetIfAbsent(funcName, 0)
		ctObj, _ := r.ctCreating.Get(funcName)
		ctNum := ctObj.(int)
		if ctNum >= 20 && memIntensiveType[funcName] != 1 && cpuIntensiveType[funcName] != 1 {
			lock.Unlock()
			for res == nil {
				res = r.getUnconstrainedContainer(containerMap)
				time.Sleep(5 * time.Millisecond)
			}
		} else {
			node, err := r.getNode(req.AccountId, req.FunctionConfig.MemoryInBytes, funcName)
			lock.Unlock()
			if err != nil {
				fmt.Printf("[ERROR] failed to get node. req:%s\n", req.RequestId)
				return nil, nil
			}
			res, err = r.createContainer(node, req)
			if err != nil {
				fmt.Printf("[ERROR] failed to create container. req:%s, node:%s\n", req.RequestId, node.nodeID)
				return nil, nil
			}
			res.requests = 1
			node.containerMap.Set(res.id, res)
			containerMap.Set(res.id, res)
			r.containerMap.Set(res.id, res)
		}
	}
	return &pb.AcquireContainerReply{
		NodeId:          res.nodeId,
		NodeAddress:     res.address,
		NodeServicePort: res.port,
		ContainerId:     res.id,
	}, nil
}

func (r *Router) ReturnContainer(req *pb.ReturnContainerRequest) error {

	ctObj, ok := r.containerMap.Get(req.GetContainerId())
	if !ok {
		return nil
	}
	container := ctObj.(*ContainerInfo)

	r.shortFuncRunning.Remove(req.ContainerId)

	if req.ErrorCode != "" || req.ErrorMessage != "" {
		//if the function invoke fails, remove the container
		r.handleContainerInvocationErr(req.RequestId, container)
	} else {
		container.Lock()
		defer container.Unlock()
		container.requests--
		container.activeTime = time.Now().UnixNano()
		container.usedMem = req.MaxMemoryUsageInBytes

		//根据函数执行时间判断是否为短时间类型函数。如果已经判断为短时间类型，并且当前执行时间超过上限，取消认定该函数为短时间类型
		if req.DurationInNanos <= shortTimeLine && !shortTimeType.Has(container.funcName) {
			fmt.Printf("[INFO] analyzed funcName{%s} is short-time type.\n", container.funcName)
			shortTimeType.SetIfAbsent(container.funcName, 1)
		} else if shortTimeType.Has(container.funcName) && req.DurationInNanos > limitTimeLine {
			fmt.Printf("[INFO] analyzed funcName{%s} isn't short-time type.\n", container.funcName)
			shortTimeType.Remove(container.funcName)
		}

		//根据getStat的分析，调整容器的资源
		if modifyMap.Has(container.funcName) && container.requests == 0 {
			mmObj, _ := modifyMap.Get(container.funcName)
			memory := mmObj.(int64)
			if container.totalMem < memory {
				cmObj, _ := r.functionMap.Get(container.funcName)
				containerMap := cmObj.(cmap.ConcurrentMap)
				nmObj, ok := r.nodeMap.Get(container.nodeId)
				if ok {
					node := nmObj.(*NodeInfo)
					go r.modifyContainer(container.nodeId, containerMap, container, memory, node)
				}
			}
		}
	}
	return nil
}

func (r *Router) getExistedContainer(containerMap cmap.ConcurrentMap, funcName string) *ContainerInfo {
	var res *ContainerInfo

	if containerMap.Count() == 0 {
		return nil
	}

	for i, key := range containerMap.Keys() {
		cmObj, ok := containerMap.Get(key)
		if !ok {
			continue
		}
		r.ctCreating.SetIfAbsent(funcName, 0)
		ctObj, _ := r.ctCreating.Get(funcName)
		ctNum := ctObj.(int)

		container := cmObj.(*ContainerInfo)
		container.Lock()
		/*if container.deleting {
			container.Unlock()
			continue
		}*/
		if i == containerMap.Count() - 1 && ctNum < 20 {
			go r.reserveContainersInAdvance(containerMap, container)
		}
		if container.requests < 1 {
			res = container
			res.requests++
			container.Unlock()
			break
		}
		container.Unlock()
	}
	return res
}

func (r *Router) getUnconstrainedContainer(containerMap cmap.ConcurrentMap) *ContainerInfo {
	if containerMap.Count() == 0 {
		return nil
	}
	for _, key := range containerMap.Keys() {
		cmObj, ok := containerMap.Get(key)
		if !ok {
			continue
		}
		container := cmObj.(*ContainerInfo)
		container.Lock()
		/*if container.deleting {
			container.Unlock()
			continue
		}*/
		container.requests++
		container.Unlock()
		return container
	}
	return nil
}

func (r *Router) getNode(accountId string, memoryReq int64, functionName string) (*NodeInfo, error) {
	var res *NodeInfo

	res = r.getExistedNode(functionName, memoryReq)
	if res != nil {
		return res, nil
	}

	res = r.getNewNode(accountId)
	if res == nil {
		return nil, nil
	}

	//首次请求时，启动rescheduleContainers
	/*if r.nodeMap.IsEmpty() {
		go r.rescheduleContainers()
	}*/

	res.usageMem += memoryReq
	res.availableMem -= memoryReq
	res.functionMap.Set(functionName, 1)
	r.nodeMap.Set(res.nodeID, res)

	r.ctCreating.SetIfAbsent(functionName, 0)
	ct0bj, _ := r.ctCreating.Get(functionName)
	ctnum := ct0bj.(int)
	r.ctCreating.Set(functionName, ctnum + 1)

	fmt.Printf("[INFO] get node success. node:%v, funcName:%s, cur node num is %d\n", res, functionName, r.nodeMap.Count())
	return res, nil
}

func (r *Router) getNewNode(accountId string) *NodeInfo {
	var res *NodeInfo

	ctxR, cancelR := context.WithTimeout(context.Background(), 30 * time.Second)
	defer cancelR()
	replyRn, err := r.rmClient.ReserveNode(ctxR, &rmPb.ReserveNodeRequest{
		AccountId: accountId,
	})
	if replyRn == nil {
		return nil
	}
	nodeDesc := replyRn.Node
	res, err = NewNode(nodeDesc.Id, nodeDesc.Address, nodeDesc.NodeServicePort, nodeDesc.MemoryInBytes)
	if err != nil {
		//release the node if can't connect to node
		r.releaseNode(nodeDesc.Id)
		return nil
	}
	return res
}

func (r *Router) getExistedNode(funcName string, memoryReq int64/*, exclude string, expand bool*/) *NodeInfo {
	var res *NodeInfo

	//1）如果并发请求数超过当前node数量且不超过initNodeNum，则获取新的node，可以更快的创建容器和更快的响应时间
	//2）否则尽量分布均匀，首先以node内该函数容器的数量排序，优先数量少的node，如果数量相同，优先可用内存大的node
	//3）如果initNode不能满足请求，则获取新的node来满足请求，规则如2）
	if r.nodeMap.Count() < initNodeNum /*&& expand*/ {
		for _, key := range r.nodeMap.Keys() {
			nmObj, ok := r.nodeMap.Get(key)
			if !ok {
				continue
			}
			node := nmObj.(*NodeInfo)
			node.Lock()
			if !node.functionMap.Has(funcName) && node.availableMem >= memoryReq {
				res = node
			}
			node.Unlock()
		}
	}  else {
		var bestFuncNum int
		var bestAvailMem int64
		for _, key := range r.nodeMap.Keys() {
			nmObj, ok := r.nodeMap.Get(key)
			if !ok {
				continue
			}
			node := nmObj.(*NodeInfo)
			/*if node.nodeID == exclude {
				continue
			}*/
			node.Lock()

			var funcNum int
			fmObj, ok := node.functionMap.Get(funcName)
			if ok {
				funcNum = fmObj.(int)
			} else {
				funcNum = 0
			}
			if node.availableMem >= memoryReq &&
				(res == nil || funcNum < bestFuncNum || (funcNum == bestFuncNum && node.availableMem > bestAvailMem)) {
				res = node
				bestFuncNum = funcNum
				bestAvailMem = node.availableMem
			}
			node.Unlock()
		}
	}
	if res != nil {
		res.Lock()
		res.functionMap.SetIfAbsent(funcName, 0)
		fmObj, _ := res.functionMap.Get(funcName)
		funcNum := fmObj.(int)
		res.functionMap.Set(funcName, funcNum + 1)
		res.availableMem -= memoryReq
		res.usageMem += memoryReq
		res.Unlock()
		r.ctCreating.SetIfAbsent(funcName, 0)
		ct0bj, _ := r.ctCreating.Get(funcName)
		ctnum := ct0bj.(int)
		r.ctCreating.Set(funcName, ctnum + 1)
		return res
	}
	return nil
}

func (r *Router) handlerGetNodeErr(memoryReq int64) (*NodeInfo, bool) {
	for _, key := range r.nodeMap.Keys() {
		nmObj, ok := r.nodeMap.Get(key)
		if !ok {
			continue
		}
		node := nmObj.(*NodeInfo)
		node.Lock()
		node.usageMem += memoryReq
		node.Unlock()
		return node, true
	}
	return nil, false
}

func (r *Router) releaseNode(nodeId string) {
	ctxR, cancelR := context.WithTimeout(context.Background(), 30 * time.Second)
	defer cancelR()
	_, err := r.rmClient.ReleaseNode(ctxR, &rmPb.ReleaseNodeRequest{
		RequestId: uuid.NewV4().String(),
		Id:        nodeId,
	})
	if err != nil {
		fmt.Printf("[ERROR] failed to release node. nodeId:%s\n", nodeId)
		return
	}
	r.nodeMap.Remove(nodeId)
	fmt.Printf("[INFO] release node. nodeId:%s\n", nodeId)
}

func (r *Router) releaseContainer(container *ContainerInfo, containerMap cmap.ConcurrentMap, node *NodeInfo) {
	//remove container and plus node available memory
	containerMap.Remove(container.id)
	r.containerMap.Remove(container.id)

	node.Lock()
	node.containerMap.Remove(container.id)

	ctxR, cancelR := context.WithTimeout(context.Background(), 30 * time.Second)
	defer cancelR()
	_, err := node.RemoveContainer(ctxR, &nsPb.RemoveContainerRequest{
		//RequestId: node.nodeID,
		RequestId:   uuid.NewV4().String(),
		ContainerId: container.id,
	})
	if err != nil {
		fmt.Printf("[ERROR] failed to remove container! containerId:%s\n", container.id)
		return
	}
	node.availableMem += container.totalMem
	node.usageMem -= container.totalMem
	functionName := container.funcName
	fnObj, _ := node.functionMap.Get(functionName)
	if fnObj != nil {
		functionNum := fnObj.(int)
		if functionNum == 1 {
			node.functionMap.Remove(functionName)
		} else {
			node.functionMap.Set(functionName, functionNum - 1)
		}
	}
	node.Unlock()
	fmt.Printf("[INFO] release container. funcName:%s, nodeId:%s, containerId:%s\n", functionName, node.nodeID, container.id)
}

func (r *Router) createContainer(node *NodeInfo, req *pb.AcquireContainerRequest) (*ContainerInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30 * time.Second)
	defer cancel()
	replyC, err := node.CreateContainer(ctx, &nsPb.CreateContainerRequest{
		//Name : node.nodeID,
		Name: req.FunctionName + uuid.NewV4().String(),
		FunctionMeta: &nsPb.FunctionMeta{
			FunctionName:  req.FunctionName,
			Handler:       req.FunctionConfig.Handler,
			TimeoutInMs:   req.FunctionConfig.TimeoutInMs,
			MemoryInBytes: req.FunctionConfig.MemoryInBytes,
		},
		RequestId: req.RequestId,
	})
	if err != nil {
		r.handleContainerErr(node, req.FunctionConfig.MemoryInBytes)
		return nil, errors.Wrapf(err, "failed to create container on %s", node.address)
	}
	fmt.Printf("[INFO] create container success. nodeId:%s, containerId:%s, functionName:%s, requestId:%s\n",
			node.nodeID, replyC.ContainerId, req.FunctionName, req.RequestId)
	res := &ContainerInfo{
		id:        replyC.ContainerId,
		address:   node.address,
		port:      node.port,
		nodeId:    node.nodeID,
		requests:  0,
		funcName:  req.FunctionName,
		totalMem:  req.FunctionConfig.MemoryInBytes,
		usedMem:   req.FunctionConfig.MemoryInBytes,
		handler:   req.FunctionConfig.Handler,
		accountId: req.AccountId,
	}
	return res, nil
}

func (r *Router) modifyContainer(newNodeId string, containerMap cmap.ConcurrentMap, container *ContainerInfo, memSize int64, node *NodeInfo) {
	//先创建新容器，再删除旧容器，实现转移容器
	r.createNewContainer(newNodeId, containerMap, container, memSize)
	container.Lock()
	container.deleting = true
	container.Unlock()
	for container.requests > 0 {
		time.Sleep(5 * time.Millisecond)
	}
	r.releaseContainer(container, containerMap, node)
	fmt.Printf("[INFO] modify the container memory size. funcName:%s, memSize:%d\n", container.funcName, memSize)
}

func (r *Router) createNewContainer(newNodeId string, containerMap cmap.ConcurrentMap, container *ContainerInfo, memSize int64) {
	nmObj, _ := r.nodeMap.Get(newNodeId)
	node := nmObj.(*NodeInfo)

	node.Lock()
	node.availableMem -= memSize
	node.usageMem += memSize
	node.functionMap.SetIfAbsent(container.funcName, 0)
	fnObj, _ := node.functionMap.Get(container.funcName)
	funcNum := fnObj.(int)
	node.functionMap.Set(container.funcName, funcNum + 1)
	node.Unlock()

	req := &pb.AcquireContainerRequest {
		FunctionName: container.funcName,
		RequestId:    uuid.NewV4().String(),
		FunctionConfig: &pb.FunctionConfig{
			MemoryInBytes: memSize,
			TimeoutInMs:   60000,
			Handler:       container.handler,
		},
	}
	res, err := r.createContainer(node, req)
	if err != nil {
		node.Lock()
		node.availableMem += memSize
		node.usageMem -= memSize
		fnObj, _ := node.functionMap.Get(container.funcName)
		funcNum := fnObj.(int)
		if funcNum == 1 {
			node.functionMap.Remove(container.funcName)
		} else {
			node.functionMap.Set(container.funcName, funcNum - 1)
		}
		node.Unlock()
		fmt.Printf("[ERROR] failed to create container. nodeId:%s, funcName:%s\n", node.nodeID, container.funcName)
		return
	}
	node.containerMap.Set(res.id, res)
	containerMap.Set(res.id, res)
	r.containerMap.Set(res.id, res)
}

func (r *Router) getStatus() {
	for {
		for _, key := range r.nodeMap.Keys() {
			nmObj, ok := r.nodeMap.Get(key)
			if !ok {
				continue
			}
			node := nmObj.(*NodeInfo)

			ctx, cancel := context.WithTimeout(context.Background(), 5 * time.Second)
			reply, err := node.GetStats(ctx, &nsPb.GetStatsRequest{
				//RequestId: node.nodeID,
				RequestId: uuid.NewV4().String(),
			})
			if err != nil {
				fmt.Println("[ERROR] failed to get container status")
				continue
			}
			fmt.Printf("get stat. node:%s, totalMem:%d, availMem:%d\n", node.nodeID, reply.NodeStats.TotalMemoryInBytes, reply.NodeStats.AvailableMemoryInBytes)
			//记录当前node的可用内存和已使用内存
			if reply.NodeStats.AvailableMemoryInBytes != 0 {
				node.Lock()
				node.availableMem = reply.NodeStats.AvailableMemoryInBytes
				node.usageMem = reply.NodeStats.MemoryUsageInBytes
				node.Unlock()
			}

			for _, stat := range reply.ContainerStatsList {

				//根据getStat获取的信息，使用内存比例超过定额则认定为mem-intensive类型，使用cpu比例超过定额则认定为cpu-intensive类型
				if float64(stat.MemoryUsageInBytes) / float64(stat.TotalMemoryInBytes) >= memUsagePct {
					ctObj, ok := r.containerMap.Get(stat.ContainerId)
					if !ok{
						continue
					}
					container := ctObj.(*ContainerInfo)
					if memIntensiveType[container.funcName] != 1 {
						fmt.Printf("[INFO] analyzed funcName{%s} is mem-intensive type.\n", container.funcName)
						memIntensiveType[container.funcName] = 1
					}
				} else if stat.CpuUsagePct >= cpuUsagePct {
					ctObj, ok := r.containerMap.Get(stat.ContainerId)
					if !ok {
						continue
					}
					container := ctObj.(*ContainerInfo)
					if cpuIntensiveType[container.funcName] != 1 {
						fmt.Printf("[INFO] analyzed funcName{%s} is cpu-intensive type.\n", container.funcName)
						cpuIntensiveType[container.funcName] = 1
					}
					//如果cpu-intensive类型的函数，执行时cpu使用/cpu分配 大于90%以上，表示当前分配的cpu满足不了该函数
					//则调整该函数的容器资源，替换成当前内存2倍的容器，达到增加cpu资源的效果
					if stat.CpuUsagePct >= r.getAllocationCpuPct(stat.TotalMemoryInBytes) * modifyLine && !modifyMap.Has(container.funcName) && memIntensiveType[container.funcName] != 1 {
						fmt.Printf("[INFO] %s cpu resources are insufficient, the memory needs to be readjusted. oldMem:%d, newMem:%d\n",
							container.funcName, container.totalMem, container.totalMem * 2)
						modifyMap.SetIfAbsent(container.funcName, container.totalMem * 2)
					}
				}
			}
			cancel()
		}
		time.Sleep(statDuration)
	}
}

/*
 * 如果当前容器满载，则提前申请一个预留容器，当请求并发数增加时，省去创建容器的时间，减少响应时间
 */
func (r *Router) reserveContainersInAdvance(containerMap cmap.ConcurrentMap, containerInfo *ContainerInfo) {
	lock.Lock()
	node := r.getExistedNode(containerInfo.funcName, containerInfo.totalMem)
	lock.Unlock()
	if node == nil {
		fmt.Printf("[ERROR]failed to get node. funcName:%s\n", containerInfo.funcName)
		return
	}
	req := &pb.AcquireContainerRequest{
		FunctionName: containerInfo.funcName,
		RequestId:    uuid.NewV4().String(),
		FunctionConfig: &pb.FunctionConfig{
			MemoryInBytes: containerInfo.totalMem,
			TimeoutInMs:   60000,
			Handler:       containerInfo.handler,
		},
	}
	container, err := r.createContainer(node, req)
	if err != nil {
		return
	}
	node.containerMap.Set(container.id, container)
	containerMap.Set(container.id, container)
	r.containerMap.Set(container.id, container)
	fmt.Printf("[INFO] create a container in advance. funcName:%s, nodeId:%s, containerId:%s\n",
		container.funcName, node.nodeID, container.id)
}

func (r *Router) handleContainerErr(node *NodeInfo, functionMem int64) {
	node.Lock()
	node.usageMem -= functionMem
	node.Unlock()
}

/*
 * 如果容器执行失败，则回收该容器，防止之后的请求失败
 */
func (r *Router) handleContainerInvocationErr(requestId string, container *ContainerInfo) {
	containerId := container.id
	fmObj, ok := r.functionMap.Get(container.funcName)
	if !ok {
		return
	}
	containerMap := fmObj.(cmap.ConcurrentMap)
	fmt.Printf("[ERROR] api invoke function error. remove the container. requestId:%s, containerId:%s\n", requestId, containerId)
	cnObj, ok := r.nodeMap.Get(container.nodeId)
	if !ok {
		return
	}
	node := cnObj.(*NodeInfo)
	r.releaseContainer(container, containerMap, node)
}

func (r *Router) printNodeInfo() {
	for {
		for _, key := range r.functionMap.Keys() {
			fmObj, ok := r.functionMap.Get(key)
			if !ok {
				continue
			}
			containerMap := fmObj.(cmap.ConcurrentMap)
			fmt.Printf("[INFO] %s num: %d\n", key, containerMap.Count())
		}
		for _, key := range r.nodeMap.Keys() {
			nmObj, ok := r.nodeMap.Get(key)
			if !ok {
				continue
			}
			node := nmObj.(*NodeInfo)
			fmt.Printf("[INFO] %s:%d\n", key, node.containerMap.Count())
		}
		time.Sleep(time.Second)
	}
}

func (r *Router) getAllocationCpuPct(memory int64) float64 {
	return float64(memory) / float64(GB) * cpuRatio * 100
}

/*
 * 根据每个node的availMem，判断是否需要减少node的使用，进行动态缩减，但分数没有提升，暂不使用
 */
func (r *Router) rescheduleContainers() {
	time.Sleep(140 * time.Second)
	for {
		migrateNum := 0
		cpuIntensiveCtNum := 0
		for key, _ := range cpuIntensiveType {
			cmObj, ok := r.functionMap.Get(key)
			if !ok {
				continue
			}
			containerMap := cmObj.(cmap.ConcurrentMap)
			cpuIntensiveCtNum += containerMap.Count()
		}
		//根据cpu-intensive类型的容器数量，初步判断缩放的node数量
		for migrateNum < r.nodeMap.Count() - int(math.Ceil(float64(cpuIntensiveCtNum) / 6)) {
			var res *NodeInfo
			for _, key := range r.nodeMap.Keys() {
				nmObj, ok := r.nodeMap.Get(key)
				if !ok {
					continue
				}
				node := nmObj.(*NodeInfo)
				if res == nil || res.availableMem < node.availableMem {
					res = node
				}
			}
			if res != nil && r.checkMigrationConditions(res) {
				fmt.Printf("[INFO] migration begin. node:%s\n", res.nodeID)
				for _, containerId := range res.containerMap.Keys() {
					cnObj, ok := res.containerMap.Get(containerId)
					if !ok {
						continue
					}
					container := cnObj.(*ContainerInfo)
					lock.Lock()
					targetNode := r.getExistedNode(container.funcName, container.usedMem)
					lock.Unlock()
					if targetNode == nil {
						fmt.Printf("[ERROR] failed to migrate container. container:%s, func:%s\n", container.id, container.funcName)
						continue
					}
					cmObj, _ := r.functionMap.Get(container.funcName)
					containerMap := cmObj.(cmap.ConcurrentMap)
					nmObj, _ := r.nodeMap.Get(res.nodeID)
					node := nmObj.(*NodeInfo)
					fmt.Printf("[INFO] migrate container. container:%s, func:%s\n", container.id, container.funcName)
					go r.modifyContainer(targetNode.nodeID, containerMap, container, container.totalMem, node)
				}

				for !res.containerMap.IsEmpty() {
					time.Sleep(10 * time.Millisecond)
				}
				r.releaseNode(res.nodeID)
				migrateNum++
				fmt.Printf("[INFO] migration end. node:%s\n", res.nodeID)
			}
			time.Sleep(5 * time.Second)
		}
		time.Sleep(10 * time.Second)
	}
}

/*
 * 检查是否满足缩放时迁移的条件
 */
func (r *Router) checkMigrationConditions(sourceNode *NodeInfo) bool {
	totalAvailMem := int64(0)
	availMemMap := make(map[string]int64)
	for _, key := range r.nodeMap.Keys() {
		if key != sourceNode.nodeID {
			nmObj, ok := r.nodeMap.Get(key)
			if !ok {
				continue
			}
			node := nmObj.(*NodeInfo)
			availMemMap[key] = node.availableMem
			totalAvailMem += node.availableMem
		}
	}
	if sourceNode.usageMem > totalAvailMem {
		return false
	}
	isMigrate := true
	for _, key := range sourceNode.containerMap.Keys() {
		cnObj, ok := sourceNode.containerMap.Get(key)
		if !ok {
			continue
		}
		container := cnObj.(*ContainerInfo)
		if memIntensiveType[container.funcName] == 1 {
			isSatisfy := false
			for key, availMem := range availMemMap {
				if availMem >= container.usedMem {
					isSatisfy = true
					availMemMap[key] -= container.usedMem
					break;
				}
			}
			if !isSatisfy {
				isMigrate = false
				break;
			}
		}
	}
	return isMigrate
}

/*
 * 删除长时间不活动的容器，回收资源，但每个函数会有一个保留容器，由于执行时间较短，暂不考虑保留容器的释放。当node内没有容器，则释放该node
 */
func (r *Router) releaseInactiveContainers() {
	for {
		funcMap := cmap.New()
		for _, key := range r.containerMap.Keys() {
			cnObj, ok := r.containerMap.Get(key)
			if !ok {
				continue
			}
			container := cnObj.(*ContainerInfo)
			if container.activeTime != 0 && time.Now().UnixNano() - container.activeTime > inactiveTime && container.requests == 0 {
				if funcMap.Has(container.funcName) {
					cmObj, _ := r.functionMap.Get(container.funcName)
					containerMap := cmObj.(cmap.ConcurrentMap)
					nmObj, ok := r.nodeMap.Get(container.nodeId)
					if !ok {
						continue
					}
					node := nmObj.(*NodeInfo)
					fmt.Printf("[INFO] release inactive container. containerId:%s, funcName:%s\n", container.id, container.funcName)
					r.releaseContainer(container, containerMap, node)
				}
				funcMap.SetIfAbsent(container.funcName, 1)
			}
		}
		for _, key := range r.nodeMap.Keys() {
			nmObj, ok := r.nodeMap.Get(key)
			if !ok {
				continue
			}
			node := nmObj.(*NodeInfo)
			totalNum := 0
			for _, key := range node.functionMap.Keys() {
				fnObj, ok := node.functionMap.Get(key)
				if !ok {
					continue
				}
				num := fnObj.(int)
				totalNum += num
			}
			if totalNum == 0 {
				r.releaseNode(node.nodeID)
			}
		}
		time.Sleep(10 * time.Second)
	}
}