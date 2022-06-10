package server

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/cockroachdb/cockroach/pkg/util/netutil"
	"github.com/cockroachdb/cockroach/pkg/util/stop"
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
				// reader, writer := io.Pipe()
				argv := []string {
					"sql",
					"--embedded",
					"--url",
				}
				argv = append(argv, s.Command()...)
				cmd := exec.CommandContext(ctx, os.Args[0], argv...)
				cmd.Stdin = s
				cmd.Stdout = s.Stderr()
				cmd.Stderr = s.Stderr()

				pty, _, accepted := s.Pty()
				fmt.Printf("accepted pty?: %+v, pty = %#v", accepted, pty)

				//TODO: allocate a tty with tty.Open() https://pkg.go.dev/github.com/mattn/go-tty#section-readme

				io.WriteString(s, strings.TrimSpace(`
				BANNER
				BANNER
				BANNER
				BANNER`) + "\n")

				if err := cmd.Start(); err != nil {
					fmt.Printf("cmd.Start failed with error %+v", err)
					return
				}

				cmd.Wait()
			},
		}

		netutil.FatalIfUnexpected(glSsh.Serve(sshLn))
	})
}
