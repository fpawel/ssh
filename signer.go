package ssh

import (
	"os"
	"path/filepath"

	"github.com/fpawel/errorx"
	"golang.org/x/crypto/ssh"
)

func newSigner(keyFile string) (ssh.Signer, string, error) {
	eb := errorx.NewBuilder("failed to create SSH signer")

	if keyFile == "" {
		homeDirPath, err := os.UserHomeDir()
		if err != nil {
			return nil, "", eb.ExtendPrefix("get home dir path").Wrap(err)
		}

		keyFile = filepath.Join(homeDirPath, ".ssh", "id_rsa")
	}

	eb = eb.WithArgs("keyFile", keyFile)

	key, err := os.ReadFile(keyFile)
	if err != nil {
		return nil, "", eb.ExtendPrefix("read private key").Wrap(err)
	}

	// Create the Signer for this private key.
	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return nil, "", eb.ExtendPrefix("parse private key").Wrap(err)
	}
	return signer, keyFile, nil
}
