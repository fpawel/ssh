package ssh

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/fpawel/errorx"
	sshConfig "github.com/fpawel/ssh/config"
	sftpClient "github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	cryptossh "golang.org/x/crypto/ssh"
)

type (
	Client struct {
		*ssh.Client
		LogInput  bool
		LogOutput bool
	}
	Config = sshConfig.Config
)

func (x Client) WithNoLogOutput() Client {
	x.LogOutput = false
	return x
}

func (x Client) WithNoLogInput() Client {
	x.LogInput = false
	return x
}

func (x Client) WithNoLog() Client {
	return Client{Client: x.Client}
}

func (x Client) Execute(cmd string) (string, error) {
	tm := time.Now()

	if x.LogInput {
		slog.Info(fmt.Sprintf("👉 %s", cmd))
	}
	sshSession, err := x.Client.NewSession()
	if err != nil {
		return "", fmt.Errorf("SSH: open session: %w", err)
	}

	defer func() {
		if err := sshSession.Close(); err != nil && !errors.Is(err, io.EOF) {
			slog.Error(fmt.Sprintf("⚠️ SSH: failed to close session after %s: %s", cmd, err))
		} else {
			slog.Debug(fmt.Sprintf("🍀 SSH: close session after %s", cmd))
		}
	}()

	b, err := sshSession.CombinedOutput(cmd)
	if err != nil {
		var e *cryptossh.ExitError
		if !errors.As(err, &e) {
			return "", fmt.Errorf("SSH: execute remotely and get output: %w", err)
		}
		slog.Warn("⚠️ " + err.Error())
	}

	if x.LogOutput {
		if s := strings.TrimSpace(string(b)); s != "" {
			slog.Info("👈 " + strings.TrimSpace(string(b)) + " " + time.Since(tm).String())
		} else {
			slog.Info("👈 " + time.Since(tm).String())
		}
	}
	return string(b), nil
}

func Connect(c Config) (_ Client, err error) {
	if c.Username == "" {
		c.Username = "root"
	}
	if c.Port == "" {
		c.Port = "22"
	}
	host := c.Host + ":" + c.Port

	SSH := Client{
		LogOutput: true,
		LogInput:  true,
	}
	var eb errorx.ErrorBuilder

	auth := []ssh.AuthMethod{
		ssh.Password(c.Password),
	}
	if c.Password == "" {
		signer, keyFile, err := newSigner(c.KeyFile)
		if err != nil {
			return SSH, err
		}
		auth = []ssh.AuthMethod{ssh.PublicKeys(signer)}
		eb = eb.WithArgs("keyFile", keyFile)
	} else {
		eb = eb.WithArgs("password", c.Password)
	}

	SSH.Client, err = ssh.Dial("tcp", host, &ssh.ClientConfig{
		User:            c.Username,
		Auth:            auth,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	})
	if err != nil {
		return SSH, eb.ExtendPrefix("dial").Wrap(err)
	}
	return SSH, nil
}

func (x Client) OpenSFTPFile(path string) (r SFTPRemoteFile, err error) {
	r.SFTP, err = sftpClient.NewClient(x.Client)
	if err != nil {
		return r, fmt.Errorf("create new SFTP client on conn: %w", err)
	}
	r.File, err = r.SFTP.Open(path)
	if err != nil {
		err = errors.Join(err, r.SFTP.Close())
		return r, fmt.Errorf("open SFTP file: %w", err)
	}

	return r, nil
}

func (x Client) CreateSFTPFile(path string, b []byte) error {
	log := slog.Default().With("path", path)
	sftp, err := sftpClient.NewClient(x.Client)
	if err != nil {
		return fmt.Errorf("SFTP: create new client on conn: %w", err)
	}
	defer func() {
		if err = sftp.Close(); err != nil {
			log.Error(fmt.Sprintf("❌ SFTP: failed to close  connection: %s", err))
		} else {
			log.Info("🍀 SFTP: close connection")
		}
	}()
	file, err := sftp.Create(path)
	if err != nil {
		return fmt.Errorf("SFTP: create file: %w", err)
	}
	defer func() {
		if err = file.Close(); err != nil {
			log.Error(fmt.Sprintf("❌ SFTP: failed to close file: %s", err))
		} else {
			log.Info("🍀 SFTP: close file")
		}
	}()

	if _, err = io.Copy(file, bytes.NewBuffer(b)); err != nil {
		return fmt.Errorf("SFTP: write file: %w", err)
	}

	return nil
}
