// Package epp provides an in-process EPP mock server for unit testing.
//
// The mock server listens on a random TCP port, handles TLS (including mutual
// TLS), and delivers pre-configured scripted responses to incoming EPP frames.
//
// Scripted response API:
//
//	srv := epp.NewMockEPPServer(t)
//	srv.Expect <- greetingXML      // enqueue scripted response
//	frame := <-srv.Received        // read captured request frame
//
// Fault injection: push MalformedFrameFault or WrongResultCodeFault onto
// srv.Expect; call srv.DropConnection() or srv.SetDelay(d) for transport faults.
//
// Each server instance is per-test with automatic t.Cleanup teardown.
// goleak.VerifyTestMain in TestMain detects goroutine leaks across parallel tests.
package epp
