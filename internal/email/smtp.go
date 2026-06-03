package email

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"mime/multipart"
	"net/smtp"
	"net/textproto"
	"time"
)

type SMTPSender struct {
	addr     string // host:port
	auth     smtp.Auth
	from     string
	sendFunc func(addr string, a smtp.Auth, from string, to []string, msg []byte) error // defaults to smtp.SendMail
}

func (s *SMTPSender) Send(ctx context.Context, msg Message) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	// Validate fields
	if msg.To == "" {
		return errors.New("recipient address (To) is required")
	}

	from := msg.From
	if from == "" {
		from = s.from
	}
	if from == "" {
		return errors.New("sender address (From) is required")
	}

	if hasHeaderInjection(msg.To) || hasHeaderInjection(from) || hasHeaderInjection(msg.Subject) {
		return fmt.Errorf("invalid header value (contains newline)")
	}

	if msg.Text == "" && msg.HTML == "" {
		return errors.New("at least one of HTML or Text body must be provided")
	}

	// Format RFC 5322 / MIME message
	var buf bytes.Buffer

	// Standard Headers
	buf.WriteString(fmt.Sprintf("From: %s\r\n", from))
	buf.WriteString(fmt.Sprintf("To: %s\r\n", msg.To))
	buf.WriteString(fmt.Sprintf("Subject: %s\r\n", msg.Subject))
	buf.WriteString(fmt.Sprintf("Date: %s\r\n", time.Now().Format(time.RFC1123Z)))
	buf.WriteString("MIME-Version: 1.0\r\n")

	if msg.Text != "" && msg.HTML != "" {
		// multipart/alternative
		mpWriter := multipart.NewWriter(&buf)
		boundary := mpWriter.Boundary()
		buf.WriteString(fmt.Sprintf("Content-Type: multipart/alternative; boundary=%s\r\n\r\n", boundary))

		// Part 1: text/plain
		partTextHeaders := make(textproto.MIMEHeader)
		partTextHeaders.Set("Content-Type", "text/plain; charset=UTF-8")
		partTextHeaders.Set("Content-Transfer-Encoding", "7bit")
		partText, err := mpWriter.CreatePart(partTextHeaders)
		if err != nil {
			return fmt.Errorf("failed to create text part: %w", err)
		}
		if _, err := partText.Write([]byte(msg.Text)); err != nil {
			return fmt.Errorf("failed to write text part: %w", err)
		}

		// Part 2: text/html
		partHTMLHeaders := make(textproto.MIMEHeader)
		partHTMLHeaders.Set("Content-Type", "text/html; charset=UTF-8")
		partHTMLHeaders.Set("Content-Transfer-Encoding", "7bit")
		partHTML, err := mpWriter.CreatePart(partHTMLHeaders)
		if err != nil {
			return fmt.Errorf("failed to create html part: %w", err)
		}
		if _, err := partHTML.Write([]byte(msg.HTML)); err != nil {
			return fmt.Errorf("failed to write html part: %w", err)
		}

		if err := mpWriter.Close(); err != nil {
			return fmt.Errorf("failed to close multipart writer: %w", err)
		}
	} else if msg.Text != "" {
		// text/plain
		buf.WriteString("Content-Type: text/plain; charset=UTF-8\r\n\r\n")
		buf.WriteString(msg.Text)
	} else {
		// text/html
		buf.WriteString("Content-Type: text/html; charset=UTF-8\r\n\r\n")
		buf.WriteString(msg.HTML)
	}

	// Again, check context before the network call
	if err := ctx.Err(); err != nil {
		return err
	}

	sendFn := s.sendFunc
	if sendFn == nil {
		sendFn = smtp.SendMail
	}

	to := []string{msg.To}
	if err := sendFn(s.addr, s.auth, from, to, buf.Bytes()); err != nil {
		return fmt.Errorf("smtp send: %w", err)
	}
	return nil
}
