// Dummy no-op pluggable transport client. Works only as a managed proxy.
//
// Usage (in torrc):
// 	UseBridges 1
// 	Bridge dummy X.X.X.X:YYYY
// 	ClientTransportPlugin dummy exec dummy-client
//
// Because this transport doesn't do anything to the traffic, you can use any
// ordinary relay's ORPort in the Bridge line; it doesn't have to declare
// support for the dummy transport.
package main

import (
	"net"
	"os"
	"os/exec"
	"os/signal"
	// "sync"
	"syscall"
)

import "git.torproject.org/pluggable-transports/goptlib.git"

var logfile *os.File

var ptInfo pt.ClientInfo

// When a connection handler starts, +1 is written to this channel; when it
// ends, -1 is written.
var handlerChan = make(chan int)

var msgChan = make(chan string)

func copyLoop(a, b net.Conn) {
	// a = 127.0.0.1:54861 (random port...)
	// b = 127.0.0.1:5353 (server)

	logfile.WriteString("copy\n")
	logfile.WriteString(a.LocalAddr().String())
	logfile.WriteString("\n")
	logfile.WriteString(b.LocalAddr().String())
	logfile.WriteString("\n")
	logfile.WriteString(a.RemoteAddr().String())
	logfile.WriteString("\n")
	logfile.WriteString(b.RemoteAddr().String())
	logfile.WriteString("\n")
	logfile.WriteString("copy\n")

	logfile.WriteString("running command\n")

	cmd := exec.Command("/Users/irvinzhan/Documents/open-source/tor/dnscat2/client/dnscat", 
		"--host", "52.5.198.6",
		"--port", "53",
		"--console")
	cmd.Stdin = a
	cmd.Stdout = a
	err := cmd.Start()
	if err != nil {
		logfile.WriteString(err.Error())
	}

	logfile.WriteString("continuing command\n")

	cmd.Wait()
}

func handler(conn *pt.SocksConn) error {
	logfile.WriteString("handler\n")
	handlerChan <- 1
	defer func() {
		handlerChan <- -1
	}()

	defer conn.Close()
	remote, err := net.Dial("tcp", conn.Req.Target)

	logfile.WriteString(conn.Req.Target)

	if err != nil {
		conn.Reject()
		return err
	}
	defer remote.Close()
	err = conn.Grant(remote.RemoteAddr().(*net.TCPAddr))
	if err != nil {
		return err
	}

	copyLoop(conn, remote)

	return nil
}

func acceptLoop(ln *pt.SocksListener) error {
	logfile.WriteString("accept\n")

	defer ln.Close()
	for {
		logfile.WriteString("before accept\n")
		conn, err := ln.AcceptSocks()
		logfile.WriteString("after accept\n")
		if err != nil {
			if e, ok := err.(net.Error); ok && !e.Temporary() {
				return err
			}
			continue
		}
		go handler(conn)
	}
}

func main() {

	logfile, _ = os.Create("/Users/irvinzhan/Documents/open-source/tor/goptlib/examples/dummy-client/logs/client.log")
	defer logfile.Close()

	var err error

	ptInfo, err = pt.ClientSetup([]string{"dummy"})
	if err != nil {
		os.Exit(1)
	}

	listeners := make([]net.Listener, 0)
	for _, methodName := range ptInfo.MethodNames {
		switch methodName {
		case "dummy":
			ln, err := pt.ListenSocks("tcp", "127.0.0.1:0")
			if err != nil {
				pt.CmethodError(methodName, err.Error())
				break
			}
			go acceptLoop(ln)
			pt.Cmethod(methodName, ln.Version(), ln.Addr())
			listeners = append(listeners, ln)
		default:
			pt.CmethodError(methodName, "no such method")
		}
	}
	pt.CmethodsDone()

	var numHandlers int = 0
	var sig os.Signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// wait for first signal
	sig = nil
	for sig == nil {
		select {
		case n := <-handlerChan:
			numHandlers += n
		case sig = <-sigChan:
		}
	}
	for _, ln := range listeners {
		ln.Close()
	}

	if sig == syscall.SIGTERM {
		return
	}

	// wait for second signal or no more handlers
	sig = nil
	for sig == nil && numHandlers != 0 {
		select {
		case n := <-handlerChan:
			numHandlers += n
		case sig = <-sigChan:
		}
	}
}
