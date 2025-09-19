package ssh

import (
	"errors"
	"io"

	"github.com/fpawel/errorx"
	"github.com/pkg/sftp"
)

type SFTPRemoteFile struct {
	*sftp.File
	SFTP *sftp.Client
}

var _ io.ReadCloser = SFTPRemoteFile{}

func (x SFTPRemoteFile) Close() error {
	if err := errors.Join(x.File.Close(), x.SFTP.Close()); err != nil {
		return errorx.NewBuilder("close remote file " + x.Name()).Wrap(err)
	}
	return nil
}
