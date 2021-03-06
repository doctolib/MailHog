package smtp

// http://www.rfc-editor.org/rfc/rfc5321.txt

import (
	"io"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/doctolib/MailHog/pkg/data"
	"github.com/doctolib/MailHog/pkg/monkey"
	"github.com/doctolib/MailHog/pkg/storage"
	"github.com/ian-kent/linkio"
)

// Session represents a SMTP session using net.TCPConn
type Session struct {
	conn          io.ReadWriteCloser
	proto         *Protocol
	storage       storage.Storage
	messageChan   chan *data.Message
	remoteAddress string
	isTLS         bool
	line          string
	link          *linkio.Link

	reader io.Reader
	writer io.Writer
	monkey monkey.ChaosMonkey
}

// Accept starts a new SMTP session using io.ReadWriteCloser
func Accept(remoteAddress string, conn io.ReadWriteCloser, storage storage.Storage, messageChan chan *data.Message, hostname string, monkey monkey.ChaosMonkey) {
	defer conn.Close()

	proto := NewProtocol()
	proto.Hostname = hostname
	var link *linkio.Link
	reader := io.Reader(conn)
	writer := io.Writer(conn)
	if monkey != nil {
		linkSpeed := monkey.LinkSpeed()
		if linkSpeed != nil {
			link = linkio.NewLink(*linkSpeed * linkio.BytePerSecond)
			reader = link.NewLinkReader(io.Reader(conn))
			writer = link.NewLinkWriter(io.Writer(conn))
		}
	}

	session := &Session{conn, proto, storage, messageChan, remoteAddress, false, "", link, reader, writer, monkey}
	proto.MessageReceivedHandler = session.acceptMessage
	proto.ValidateSenderHandler = session.validateSender
	proto.ValidateRecipientHandler = session.validateRecipient
	proto.ValidateAuthenticationHandler = session.validateAuthentication
	proto.GetAuthenticationMechanismsHandler = func() []string { return []string{"PLAIN"} }

	session.log().Info("Starting session")
	session.Write(proto.Start())
	for session.Read() {
		if monkey != nil && monkey.Disconnect() {
			session.conn.Close()
			break
		}
	}
	session.log().Info("Session ended")
}

func (c *Session) validateAuthentication(mechanism string, args ...string) (errorReply *Reply, ok bool) {
	if c.monkey != nil {
		ok := c.monkey.ValidAUTH(mechanism, args...)
		if !ok {
			// FIXME better error?
			return ReplyUnrecognisedCommand(), false
		}
	}
	return nil, true
}

func (c *Session) validateRecipient(to string) bool {
	if c.monkey != nil {
		ok := c.monkey.ValidRCPT(to)
		if !ok {
			return false
		}
	}
	return true
}

func (c *Session) validateSender(from string) bool {
	if c.monkey != nil {
		ok := c.monkey.ValidMAIL(from)
		if !ok {
			return false
		}
	}
	return true
}

func (c *Session) acceptMessage(msg *data.SMTPMessage) (id string, err error) {
	m := msg.Parse(c.proto.Hostname)
	c.log().Debugf("Storing message %s", m.ID)
	id, err = c.storage.Store(m)
	c.messageChan <- m
	return
}

func (c *Session) log() *log.Entry {
	return log.WithFields(
		log.Fields{"smtp_remote_address": c.remoteAddress},
	)
}

// Read reads from the underlying net.TCPConn
func (c *Session) Read() bool {
	buf := make([]byte, 1024)
	n, err := c.reader.Read(buf)

	if n == 0 {
		c.log().Info("Connection closed by remote host")
		io.Closer(c.conn).Close() // not sure this is necessary?
		return false
	}

	if err != nil {
		c.log().Errorf("Error reading from socket: %s\n", err)
		return false
	}

	text := string(buf[0:n])
	logText := strings.Replace(text, "\n", "\\n", -1)
	logText = strings.Replace(logText, "\r", "\\r", -1)
	c.log().Tracef("Received %d bytes: '%s'\n", n, logText)

	c.line += text

	for strings.Contains(c.line, "\r\n") {
		line, reply := c.proto.Parse(c.line)
		c.line = line

		if reply != nil {
			c.Write(reply)
			if reply.Status == 221 {
				io.Closer(c.conn).Close()
				return false
			}
		}
	}

	return true
}

// Write writes a reply to the underlying net.TCPConn
func (c *Session) Write(reply *Reply) {
	lines := reply.Lines()
	for _, l := range lines {
		logText := strings.Replace(l, "\n", "\\n", -1)
		logText = strings.Replace(logText, "\r", "\\r", -1)
		c.log().Tracef("Sent %d bytes: '%s'", len(l), logText)
		c.writer.Write([]byte(l))
	}
}
