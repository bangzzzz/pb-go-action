package ggrpc

// Quintet is the five-tuple that identifies a gRPC call context.
// It describes both the caller (client) and callee (server) sides.
type Quintet struct {
	// CallerService is the name of the calling service (client side).
	CallerService string
	// CallerCluster is the cluster the caller belongs to.
	CallerCluster string
	// CalleeService is the name of the service being called (server side).
	CalleeService string
	// CalleeMethod is the specific method being called on the server.
	CalleeMethod string
	// CalleeCluster is the cluster the callee belongs to.
	CalleeCluster string
}
