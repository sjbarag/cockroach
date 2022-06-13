package server

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/cockroachdb/cockroach/pkg/util/netutil"
	"github.com/cockroachdb/cockroach/pkg/util/stop"
	"github.com/creack/pty"
	"github.com/gliderlabs/ssh"
)

type sshServer struct{}

func newSSHServer() sshServer {
	return sshServer{}
}

func (s *sshServer) start(
	ctx, workersCtx context.Context,
	connManager netutil.Server,
	stopper *stop.Stopper,
) error {
	var listenAddr string = "localhost:22775"
	var advAddr string = "localhost:22775"
	sshLn, err := ListenAndUpdateAddrs(ctx, &listenAddr, &advAddr, "ssh")
	if err != nil {
		return err
	}
	log.Eventf(ctx, "listening on ssh port %s", listenAddr)

	// The SSH listener shutdown worker, which closes everything under
	// the SSH port when the stopper indicates we are shutting down.
	waitQuiesce := func(ctx context.Context) {
		// NB: we can't do this as a Closer because (*Server).ServeWith is
		// running in a worker and usually sits on accept() which unblocks
		// only when the listener closes. In other words, the listener needs
		// to close when quiescing starts to allow that worker to shut down.
		<-stopper.ShouldQuiesce()
		if err := sshLn.Close(); err != nil {
			log.Ops.Fatalf(ctx, "%v", err)
		}
	}

	if err := stopper.RunAsyncTask(workersCtx, "wait-quiesce", waitQuiesce); err != nil {
		waitQuiesce(workersCtx)
		return err
	}

	// Actually do the ssh parts
	return stopper.RunAsyncTask(workersCtx, "server-ssh", func(ctx context.Context) {

		glSsh := ssh.Server{
			Addr: listenAddr,
			Handler: func(s ssh.Session) {
				// TODO: auth sql user by parsing connect URL, prompting for a password if none provided
				argv := []string {
					"sql",
					"--embedded",
					"--url",
				}
				argv = append(argv, s.Command()...)
				cmd := exec.CommandContext(ctx, os.Args[0], argv...)

				sessionPty, _, accepted := s.Pty()
				fmt.Fprintf(os.Stderr, "accepted pty?: %+v, pty = %#v\n", accepted, sessionPty)

				shellPty, err := pty.StartWithSize(cmd, &pty.Winsize{
					Rows: uint16(sessionPty.Window.Height),
					Cols: uint16(sessionPty.Window.Width),
					// X and Y left at zero because they're not consistently knowable over SSH
					// (X11 Forwarding could help potentially)
				})
				if err != nil {
					fmt.Fprintf(os.Stderr, "pty.Start failed with error %+v\n", err)
					return
				}

				// Merge the PTY's stdout and stderr into the SSH session's stdout
				go func(){ _, _ = io.Copy(shellPty, s) }()
				// Copy the SSH session's stdin to the PTY's stdin
				_, _ = io.Copy(s, shellPty)
			},
		}

		netutil.FatalIfUnexpected(glSsh.Serve(sshLn))
	})
}
