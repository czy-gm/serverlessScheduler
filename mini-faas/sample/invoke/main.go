package main

import (
	pb "aliyun/serverless/mini-faas/apiserver/proto"

	"google.golang.org/grpc"

	"context"
	"flag"
	"fmt"
	"time"
)

type input struct {
	functionName string
	param        string
}

var sampleEvents = map[string]string {
	"dev_function_1":"1.2",
	"dev_function_2":"",
	"dev_function_3":"",
	"dev_function_4":"50",
	"dev_function_5":"",
}

func main() {
	var apiserverEndpointPtr = flag.String("apiserverEndpoint", "0.0.0.0:10500", "endpoint of the apiserver")
	var functionNameStr = flag.String("functionName", "", "function name")
	var eventStr = flag.String("event", "hello, cloud native!", "function input event")
	flag.Parse()

	conn, err := grpc.Dial(*apiserverEndpointPtr, grpc.WithInsecure())
	if err != nil {
		panic(err)
	}
	asClient:= pb.NewAPIServerClient(conn)

	lfReply, err := asClient.ListFunctions(context.Background(), &pb.ListFunctionsRequest{})
	if err != nil {
		panic(err)
	}

	if *functionNameStr != "" {
		invokeFunction(asClient, *functionNameStr, []byte(*eventStr))
		return
	}

	for _, f := range lfReply.Functions {
		e := sampleEvents[f.FunctionName]
		event := fmt.Sprintf(`{"functionName": "%s", "param": "%s"}`, f.FunctionName, e)
		invokeFunction(asClient, f.FunctionName, []byte(event))
	}
}

func invokeFunction(asClient pb.APIServerClient, functionName string, event []byte) {
	fmt.Printf("Invoking function %s with event %s\n", functionName, string(event))
	ctxAc, cancelAc := context.WithTimeout(context.Background(), 30*time.Second)
	invkReply, err := asClient.InvokeFunction(ctxAc, &pb.InvokeFunctionRequest{
		AccountId:    "test-account-id",
		FunctionName: functionName,
		Event:        event,
	})
	cancelAc()
	if err != nil {
		fmt.Printf("Failed to invoke function %s due to %+v\n", functionName, err)
		return
	}

	fmt.Printf("Invoke function reply %+v\n", invkReply)
}
