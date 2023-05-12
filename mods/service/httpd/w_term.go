package httpd

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	cmap "github.com/orcaman/concurrent-map"
	"github.com/pkg/errors"
	"golang.org/x/crypto/ssh"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func (svr *httpd) handleTermData(ctx *gin.Context) {
	termId := ctx.Param("term_id")
	if len(termId) == 0 {
		ctx.String(http.StatusBadRequest, "invalid termId")
		return
	}
	// current websocket spec requires pass the token through handshake process
	token := ctx.Query("token")
	claim, err := svr.verifyAccessToken(token)
	if err != nil {
		ctx.String(http.StatusUnauthorized, "unauthorized access")
		return
	}
	termLoginName := claim.Subject
	termPassword := svr.neoShellAccount[termLoginName]
	if len(termPassword) == 0 {
		termPassword = "manager"
	}
	termAddress := svr.neoShellAddress
	if len(termAddress) == 0 {
		termAddress = "127.0.0.1:5652"
	}
	termIdleTimeout := time.Duration(0) // 0 - no timeout

	termKey := fmt.Sprintf("%s-%s", termLoginName, termId)

	conn, err := upgrader.Upgrade(ctx.Writer, ctx.Request, nil)
	if err != nil {
		svr.log.Errorf("term ws upgrade fail %s", err.Error())
		ctx.String(http.StatusBadRequest, err.Error())
		return
	}

	_, _, err = net.SplitHostPort(termAddress)
	if err != nil {
		svr.log.Warnf("term invalid address %s", err.Error())
		ctx.String(http.StatusInternalServerError, err.Error())
		return
	}

	term, err := NewTerm(termAddress, termLoginName, termPassword)
	if err != nil {
		svr.log.Warnf("term conn %s", err.Error())
		ctx.String(http.StatusBadGateway, err.Error())
		return
	}

	svr.log.Debugf("term %s register %s", termKey, termAddress)
	terminals.Register(termKey, term)

	defer func() {
		svr.log.Debugf("term %s unregister %s", termKey, termAddress)
		terminals.Unregister(termKey)
		term.Close()
		conn.Close()
	}()

	oneceCloseMessage := sync.Once{}

	go func() {
		b := [termBuffSize]byte{}
		for {
			n, err := term.Stdout.Read(b[:])
			if err != nil {
				if !errors.Is(err, io.EOF) {
					conn.WriteMessage(websocket.TextMessage|websocket.CloseMessage, []byte(fmt.Sprintf("\r\nError: %s\r\n", err.Error())))
					svr.log.Errorf("term %s error %s", termKey, err.Error())
				} else {
					oneceCloseMessage.Do(func() {
						conn.WriteMessage(websocket.TextMessage|websocket.CloseMessage, []byte("\r\nclosed.\r\n"))
					})
				}
				return
			}
			if n == 0 {
				continue
			}
			conn.WriteMessage(websocket.BinaryMessage, b[:n])
		}
	}()

	go func() {
		b := [termBuffSize]byte{}
		for {
			n, err := term.Stderr.Read(b[:])
			if err != nil {
				if !errors.Is(err, io.EOF) {
					conn.WriteMessage(websocket.TextMessage|websocket.CloseMessage, []byte(fmt.Sprintf("\r\nError: %s\r\n", err.Error())))
					svr.log.Errorf("term %s error %s", termKey, err.Error())
				} else {
					oneceCloseMessage.Do(func() {
						conn.WriteMessage(websocket.TextMessage|websocket.CloseMessage, []byte("\r\nclosed.\r\n"))
					})
				}
				return
			}
			if n == 0 {
				continue
			}
			conn.WriteMessage(websocket.BinaryMessage, b[:n])
		}
	}()

	for {
		if termIdleTimeout > 0 {
			conn.SetReadDeadline(time.Now().Add(termIdleTimeout))
		}
		_, message, err := conn.ReadMessage()
		if err != nil {
			if closeErr, ok := err.(*websocket.CloseError); ok {
				svr.log.Debugf("term %s closed by websocket %d %s", termKey, closeErr.Code, closeErr.Text)
			} else if !errors.Is(err, io.EOF) {
				svr.log.Errorf("term %s error %T %s", termKey, err, err.Error())
			}
			conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("\r\nconnection closed. %s\r\n", err.Error())))
			return
		}
		_, err = term.Stdin.Write(message)
		if err != nil {
			if !errors.Is(err, io.EOF) {
				conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("\r\nError: %s\r\n", err.Error())))
				svr.log.Errorf("%s term error %T %s", termKey, err, err.Error())
			}
			return
		}
	}
}

type setTerminalSizeRequest struct {
	Rows int `query:"rows" form:"rows" json:"rows"`
	Cols int `query:"cols" form:"cols" json:"cols"`
}

func (svr *httpd) handleTermWindowSize(ctx *gin.Context) {
	termId := ctx.Param("term_id")
	termLoginName := "sys"

	req := &setTerminalSizeRequest{}
	if err := ctx.Bind(req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"success": false, "reason": err.Error()})
		return
	}
	if req.Rows == 0 || req.Cols == 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"success": false, "reason": "rows or cols can't be zero"})
		return
	}
	termKey := fmt.Sprintf("%s-%s", termLoginName, termId)
	if term, ok := terminals.Find(termKey); !ok {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"reason":  fmt.Sprintf("term '%s' not found", termKey)})
		return
	} else {
		err := term.SetWindowSize(req.Rows, req.Cols)
		if err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"success": false, "reason": err.Error()})
			return
		}
		// svr.log.Debugf("term %s resize %d %d", termKey, req.Rows, req.Cols)
	}
	ctx.JSON(http.StatusOK, gin.H{"success": true, "reason": "success"})
}

const termBuffSize = 4096 // 8192

var terminals = &Terminals{
	list: cmap.New(),
}

type Terminals struct {
	list cmap.ConcurrentMap
}

func (terms *Terminals) Register(termKey string, term *Term) {
	terms.list.Set(termKey, term)
}

func (terms *Terminals) Unregister(termKey string) {
	terms.list.Remove(termKey)
}

func (terms *Terminals) Find(termKey string) (*Term, bool) {
	if v, ok := terms.list.Get(termKey); ok {
		if term, ok := v.(*Term); ok {
			return term, true
		}
	}
	return nil, false
}

type Term struct {
	Type   string
	Rows   int
	Cols   int
	Stdout io.Reader
	Stderr io.Reader
	Stdin  io.WriteCloser
	Since  time.Time

	conn    *ssh.Client
	session *ssh.Session
}

func NewTerm(hostPort string, user string, password string) (*Term, error) {
	conn, err := ssh.Dial("tcp", hostPort, &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.Password(password),
		},
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error { return nil },
	})
	if err != nil {
		return nil, errors.Wrap(err, "NewTerm dial")
	}

	// Creating a session from the connection
	session, err := conn.NewSession()
	if err != nil {
		conn.Close()
		return nil, errors.Wrap(err, "NewTerm new session")
	}
	term := &Term{
		Type:    "xterm",
		Rows:    25,
		Cols:    80,
		Since:   time.Now(),
		conn:    conn,
		session: session,
	}
	term.Stdout, err = session.StdoutPipe()
	if err != nil {
		return nil, errors.Wrap(err, "NewTerm stdout pipe")
	}
	term.Stderr, err = session.StderrPipe()
	if err != nil {
		return nil, errors.Wrap(err, "NewTerm stderr pipe")
	}
	term.Stdin, err = session.StdinPipe()
	if err != nil {
		return nil, errors.Wrap(err, "NewTerm stdin pipe")
	}

	// request pty
	err = session.RequestPty(term.Type, term.Rows, term.Cols, ssh.TerminalModes{
		ssh.ECHO: 1, // disable echoing
	})
	if err != nil {
		term.Stdin.Close()
		session.Close()
		return nil, errors.Wrap(err, "NewTerm pty")
	}
	// request shell
	err = session.Shell()
	if err != nil {
		term.Stdin.Close()
		session.Close()
		conn.Close()
		return nil, errors.Wrap(err, "NewTerm shell")
	}

	return term, nil
}

func (term *Term) SetWindowSize(rows, cols int) error {
	err := term.session.WindowChange(rows, cols)
	if err != nil {
		return errors.Wrap(err, "SetWindowSize")
	}
	term.Rows, term.Cols = rows, cols
	return nil
}

func (term *Term) Close() {
	if term.Stdin != nil {
		term.Stdin.Close()
	}
	if term.session != nil {
		term.session.Signal(ssh.SIGKILL)
		term.session.Close()
	}
	if term.conn != nil {
		term.conn.Close()
	}
}
