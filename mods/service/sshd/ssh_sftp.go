package sshd

import (
	"io"

	"github.com/gliderlabs/ssh"
	"github.com/pkg/sftp"
)

func (svr *sshd) SftpHandler(sess ssh.Session) {
	debugStream := io.Discard
	serverOptions := []sftp.ServerOption{
		sftp.WithDebug(debugStream),
	}
	server, err := sftp.NewServer(
		sess,
		serverOptions...,
	)
	if err != nil {
		svr.log.Warn("sftp server init error:", err)
		return
	}
	svr.log.Debug("sftp client start session")
	if err := server.Serve(); err == io.EOF {
		// FIXME: sess doesn't return io.EOF when client disconnects
		server.Close()
		svr.log.Debug("sftp client exited session")
	} else if err != nil {
		svr.log.Warn("sftp server completed with error:", err)
	}
}
