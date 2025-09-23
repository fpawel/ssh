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
		LogInput   bool
		LogOutput  bool
		StdoutOnly bool
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

func (x Client) WithStdoutOnly() Client {
	x.StdoutOnly = true
	return x
}

func (x Client) WithNoLog() Client {
	x.LogOutput = false
	x.LogInput = false
	return x
}

func (x Client) Execute(cmd string) (string, error) {
	tm := time.Now()

	if x.LogInput {
		slog.Debug(fmt.Sprintf("👉 ssh: %s", cmd))
	}
	sshSession, err := x.Client.NewSession()
	if err != nil {
		return "", fmt.Errorf("ssh: open session: %w", err)
	}

	defer func() {
		if err := sshSession.Close(); err != nil && !errors.Is(err, io.EOF) {
			slog.Error(fmt.Sprintf("⚠️ ssh: failed to close session after %s: %s", cmd, err))
		}
	}()

	var b []byte
	if x.StdoutOnly {
		sshSession.Stderr = bytes.NewBuffer(nil)
		b, err = sshSession.Output(cmd)
	} else {
		b, err = sshSession.CombinedOutput(cmd)
	}

	if err != nil {
		if x.StdoutOnly {
			bErr, errStderr := io.ReadAll(sshSession.Stderr.(*bytes.Buffer))
			if errStderr != nil {
				errStderr = fmt.Errorf("read stderr: %w", errStderr)
			} else {
				errStderr = errors.New(string(bErr))
			}
			err = errors.Join(err, errStderr)
		}
		var e *cryptossh.ExitError
		if !errors.As(err, &e) {
			return "", fmt.Errorf("ssh: execute remotely and get output: %w", err)
		}
		slog.Warn(fmt.Sprintf("⚠️ ssh: %s", err))
	}

	if x.LogOutput {
		if s := strings.TrimSpace(string(b)); s != "" {
			slog.Debug("👈 ssh: " + strings.TrimSpace(string(b)) + " " + time.Since(tm).String())
		} else {
			slog.Debug("👈 ssh: " + time.Since(tm).String())
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
	slog.Debug("🍀 Connected to ssh host: " + host)
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
			log.Debug("🍀 SFTP: close connection")
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
			log.Debug("🍀 SFTP: close file")
		}
	}()

	if _, err = io.Copy(file, bytes.NewBuffer(b)); err != nil {
		return fmt.Errorf("SFTP: write file: %w", err)
	}

	return nil
}
