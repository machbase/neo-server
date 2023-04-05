package sshd

import "github.com/gliderlabs/ssh"

func (svr *sshd) reversePortForwardingCallback(ctx ssh.Context, bindHost string, bindPort uint32) bool {
	svr.log.Infof("start reverse port forwarding bindHost:%s bindPort:%d", bindHost, bindPort)
	go func() {
		<-ctx.Done()
		svr.log.Infof("done  reverse port forwarding bindHost:%s bindPort:%d", bindHost, bindPort)
	}()
	return true
}

func (svr *sshd) portForwardingCallback(ctx ssh.Context, destinationHost string, destinationPort uint32) bool {
	svr.log.Infof("start port forwarding destHost:%s destPort:%d", destinationHost, destinationPort)
	go func() {
		<-ctx.Done()
		svr.log.Infof("done  port forwarding destHost:%s destPort:%d", destinationHost, destinationPort)
	}()
	return true
}
