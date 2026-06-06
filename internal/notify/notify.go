package notify

import (
	"fmt"
	"time"

	"github.com/wneessen/go-mail"

	"github.com/vesper/lobeam/internal/config"
)

type Service struct {
	cfg *config.Config
}

func NewService(cfg *config.Config) *Service {
	return &Service{cfg: cfg}
}

func (s *Service) isEnabled() bool {
	return s.cfg.SMTPHost != ""
}

func (s *Service) SendTransferReady(toEmail, transferID, transferName string) error {
	if !s.isEnabled() || toEmail == "" {
		return nil
	}

	downloadURL := fmt.Sprintf("%s/d/%s", s.cfg.PublicURL, transferID)
	subject := fmt.Sprintf("Files received: %s", transferName)
	body := fmt.Sprintf(`
You have received files from %s.

Transfer: %s
Files: %s

Download link: %s

This link will expire based on the sender's settings.
`, transferName, transferName, downloadURL, downloadURL)

	return s.send(toEmail, subject, body)
}

func (s *Service) SendTransferEmail(toEmail, fromEmail, subject, message, downloadURL, transferName string) error {
	if !s.isEnabled() || toEmail == "" {
		return nil
	}
	if fromEmail == "" {
		fromEmail = s.cfg.SMTPFrom
	}
	if subject == "" {
		subject = fmt.Sprintf("Files shared via LoBeam: %s", transferName)
	}
	body := fmt.Sprintf("Files have been shared with you via LoBeam.\n\nTransfer: %s\nDownload: %s", transferName, downloadURL)
	if message != "" {
		body = fmt.Sprintf("%s\n\nMessage:\n%s", body, message)
	}
	body = fmt.Sprintf("%s\n\nThis link will expire based on the sender's settings.", body)

	return s.SendRaw(toEmail, fromEmail, subject, body)
}

func (s *Service) SendDownloadNotification(senderEmail, transferID, fileName string) error {
	if !s.isEnabled() || senderEmail == "" {
		return nil
	}

	subject := fmt.Sprintf("Your file has been downloaded: %s", fileName)
	body := fmt.Sprintf(`
Your transfer "%s" has been downloaded.

File: %s
Transfer ID: %s
Time: %s
`, fileName, fileName, transferID, time.Now().Format(time.RFC1123))

	return s.send(senderEmail, subject, body)
}

func (s *Service) SendTransferExpiring(toEmail, transferID, transferName string, expiresAt time.Time) error {
	if !s.isEnabled() || toEmail == "" {
		return nil
	}

	subject := fmt.Sprintf("Transfer expiring soon: %s", transferName)
	body := fmt.Sprintf(`
Your transfer "%s" is expiring soon.

Transfer ID: %s
Expires at: %s

Download before it expires: %s/d/%s
`, transferName, transferID, expiresAt.Format(time.RFC1123), s.cfg.PublicURL, transferID)

	return s.send(toEmail, subject, body)
}

// SendRaw sends a raw email with custom from address. Used for auto-notifications.
func (s *Service) SendRaw(to, from, subject, body string) error {
	if !s.isEnabled() {
		return nil
	}

	client, err := mail.NewClient(s.cfg.SMTPHost,
		mail.WithPort(s.cfg.SMTPPort),
		mail.WithTLSPortPolicy(mail.TLSMandatory),
		mail.WithSMTPAuth(mail.SMTPAuthPlain),
		mail.WithUsername(s.cfg.SMTPUsername),
		mail.WithPassword(s.cfg.SMTPPassword),
		mail.WithTimeout(10*time.Second),
	)
	if err != nil {
		return fmt.Errorf("create smtp client: %w", err)
	}

	msg := mail.NewMsg()
	if err := msg.From(from); err != nil {
		return fmt.Errorf("set from: %w", err)
	}
	if err := msg.AddTo(to); err != nil {
		return fmt.Errorf("set to: %w", err)
	}
	msg.Subject(subject)
	msg.SetBodyString(mail.TypeTextPlain, body)

	if err := client.DialAndSend(msg); err != nil {
		return fmt.Errorf("send email: %w", err)
	}

	return nil
}

func (s *Service) send(to, subject, body string) error {
	if !s.isEnabled() {
		return nil
	}

	client, err := mail.NewClient(s.cfg.SMTPHost,
		mail.WithPort(s.cfg.SMTPPort),
		mail.WithTLSPortPolicy(mail.TLSMandatory),
		mail.WithSMTPAuth(mail.SMTPAuthPlain),
		mail.WithUsername(s.cfg.SMTPUsername),
		mail.WithPassword(s.cfg.SMTPPassword),
		mail.WithTimeout(10*time.Second),
	)
	if err != nil {
		return fmt.Errorf("create smtp client: %w", err)
	}

	msg := mail.NewMsg()
	if err := msg.From(s.cfg.SMTPFrom); err != nil {
		return fmt.Errorf("set from: %w", err)
	}
	if err := msg.AddTo(to); err != nil {
		return fmt.Errorf("set to: %w", err)
	}
	msg.Subject(subject)
	msg.SetBodyString(mail.TypeTextPlain, body)

	if err := client.DialAndSend(msg); err != nil {
		return fmt.Errorf("send email: %w", err)
	}

	return nil
}
