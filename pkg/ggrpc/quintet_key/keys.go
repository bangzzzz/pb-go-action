// Package quintet_key defines the gRPC metadata key constants for the five-tuple.
package quintet_key

const (
	CallerService = "x-caller-service"
	CallerCluster = "x-caller-cluster"
	CalleeService = "x-callee-service"
	CalleeCluster = "x-callee-cluster"
	CalleeMethod  = "x-callee-method"
)
