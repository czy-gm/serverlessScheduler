package model

type RequestInfo struct {
	ID               string
	FunctionName     string
	RequestTimeoutMs int64
	AccountID        string
}

type ResponseInfo struct {
	ID           string
	FunctionName string
	ContainerId  string
}
