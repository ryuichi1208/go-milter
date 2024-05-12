package milter

import (
	"context"
	"errors"
	"net"
	"net/textproto"
)

// Milter protocol version implemented by the server.
//
// Note: Not exported as we might want to support multiple versions
// transparently in the future.
var serverProtocolVersion uint32 = 2

// ErrServerClosed is returned by the Server's Serve method after a call to
// Close.
var ErrServerClosed = errors.New("milter: server closed")

// Milter is an interface for milter callback handlers.
type Milter interface {
	// Connect is called to provide SMTP connection data for incoming message.
	// Suppress with OptNoConnect.
	Connect(ctx context.Context, host string, family string, port uint16, addr net.IP, m *Modifier) (Response, error)

	// Helo is called to process any HELO/EHLO related filters. Suppress with
	// OptNoHelo.
	Helo(ctx context.Context, name string, m *Modifier) (Response, error)

	// MailFrom is called to process filters on envelope FROM address. Suppress
	// with OptNoMailFrom.
	MailFrom(ctx context.Context, from string, m *Modifier) (Response, error)

	// RcptTo is called to process filters on envelope TO address. Suppress with
	// OptNoRcptTo.
	RcptTo(ctx context.Context, rcptTo string, m *Modifier) (Response, error)

	// Header is called once for each header in incoming message. Suppress with
	// OptNoHeaders.
	Header(ctx context.Context, name string, value string, m *Modifier) (Response, error)

	// Headers is called when all message headers have been processed. Suppress
	// with OptNoEOH.
	Headers(ctx context.Context, h textproto.MIMEHeader, m *Modifier) (Response, error)

	// BodyChunk is called to process next message body chunk data (up to 64KB
	// in size). Suppress with OptNoBody.
	BodyChunk(ctx context.Context, chunk []byte, m *Modifier) (Response, error)

	// Body is called at the end of each message. All changes to message's
	// content & attributes must be done here.
	Body(ctx context.Context, m *Modifier) (Response, error)

	// Abort is called is the current message has been aborted. All message data
	// should be reset to prior to the Helo callback. Connection data should be
	// preserved.
	Abort(ctx context.Context, m *Modifier) error
}

// NoOpMilter is a dummy Milter implementation that does nothing.
type NoOpMilter struct{}

var _ Milter = NoOpMilter{}

func (NoOpMilter) Connect(ctx context.Context, host string, family string, port uint16, addr net.IP, m *Modifier) (Response, error) {
	return RespContinue, nil
}

func (NoOpMilter) Helo(ctx context.Context, name string, m *Modifier) (Response, error) {
	return RespContinue, nil
}

func (NoOpMilter) MailFrom(ctx context.Context, from string, m *Modifier) (Response, error) {
	return RespContinue, nil
}

func (NoOpMilter) RcptTo(ctx context.Context, rcptTo string, m *Modifier) (Response, error) {
	return RespContinue, nil
}

func (NoOpMilter) Header(ctx context.Context, name string, value string, m *Modifier) (Response, error) {
	return RespContinue, nil
}

func (NoOpMilter) Headers(ctx context.Context, h textproto.MIMEHeader, m *Modifier) (Response, error) {
	return RespContinue, nil
}

func (NoOpMilter) BodyChunk(ctx context.Context, chunk []byte, m *Modifier) (Response, error) {
	return RespContinue, nil
}

func (NoOpMilter) Body(ctx context.Context, m *Modifier) (Response, error) {
	return RespAccept, nil
}

func (NoOpMilter) Abort(ctx context.Context, m *Modifier) error {
	return nil
}

// Server is a milter server.
type Server struct {
	NewMilter func() Milter
	Actions   OptAction
	Protocol  OptProtocol

	listeners []net.Listener
	closed    bool
}

// Serve starts the server.
func (s *Server) Serve(ln net.Listener) error {
	defer ln.Close()

	s.listeners = append(s.listeners, ln)

	for {
		conn, err := ln.Accept()
		if err != nil {
			if s.closed {
				return ErrServerClosed
			}
			return err
		}

		session := milterSession{
			server:   s,
			actions:  s.Actions,
			protocol: s.Protocol,
			conn:     conn,
			backend:  s.NewMilter(),
		}
		go session.HandleMilterCommands()
	}
}

func (s *Server) Close() error {
	s.closed = true
	for _, ln := range s.listeners {
		if err := ln.Close(); err != nil {
			return err
		}
	}
	return nil
}
