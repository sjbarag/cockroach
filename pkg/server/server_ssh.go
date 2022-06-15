package server

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/cockroachdb/cockroach/pkg/cli/clicfg"
	"github.com/cockroachdb/cockroach/pkg/cli/clisqlcfg"
	"github.com/cockroachdb/cockroach/pkg/cli/clisqlclient"
	"github.com/cockroachdb/cockroach/pkg/cli/clisqlexec"
	"github.com/cockroachdb/cockroach/pkg/server/pgurl"
	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/cockroachdb/cockroach/pkg/util/netutil"
	"github.com/cockroachdb/cockroach/pkg/util/stop"
	"github.com/gliderlabs/ssh"
	"github.com/creack/pty"
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

		const roach = `#
# __aaawwmqmqmwwwaas,,_        .__aaawwwmqmqmwwaaa,,
# "VT?!"""^~~^"""??T$Wmqaa,_auqmWBT?!"""^~~^^""??YV^
#                     "?##mW##?"-
#                    _am#Z??A#ma,
#                  _ummY"    "9#ma,
#                 vm#Z(        )Xmms
#               .j####mmm#####mm#m##6.
#               jmm###mm######m#mmm##6
#              ]#me*Xm#m#mm##m#m##SX##c
#              dm#||+*$##m#mm#m#Svvn##m
#             :mmE=|+||S##m##m#1nvnnX##;
#             :m#h+|+++=Xmm#m#1nvnnvdmm;
#              $#m>+|+|||##m#1nvnnnnmm#
#              ]##z+|+|+|3#mEnnnnvnd##f
#               4##c|+|+|]m#kvnvnno##P
#                4#ma+|++]mmhvnnvq##P'
#                 ?$#q%+|dmmmvnnm##!
#                  -4##wu#mm#pw##7'
#                    -?$##m####Y'
#                       "Y##Y"-
#
`

		const welcomeMessage = `#
# Welcome to the CockroachDB SQL shell, served over SSH oh wow, neat.
# All statements must be terminated by a semicolon.
# To exit, type: \q.
#
`

		glSsh := ssh.Server{
			Addr: listenAddr,
			Handler: func(s ssh.Session) {
				sessionPty, _, accepted := s.Pty()
				fmt.Fprintf(os.Stderr, "accepted pty?: %+v, pty = %#v\n", accepted, sessionPty)
				if !accepted {
					io.WriteString(s, "# For the best experience, please request a TTY.\n")
					io.WriteString(s, "# Typically that's by adding '-t' to your SSH command, e.g.:\n")
					io.WriteString(s, "#     ssh -t -p 22775 example.com 'postgresql://foo:passwd@/bar'\n")
					io.WriteString(s, "#         ^^\n\n")
				}

				thePty, theTty, err := pty.Open()

				cliCtx := &clicfg.Context{
					IsInteractive: true,
					EmbeddedMode:  true,
				}
				cfg := &clisqlcfg.Context{
					CliCtx:  cliCtx,
					ConnCtx: &clisqlclient.Context{CliCtx: cliCtx, DebugMode: true},
					ExecCtx: &clisqlexec.Context{CliCtx: cliCtx},
				}
				cfg.LoadDefaults(theTty, theTty)

				// TODO: determine if we can do full CLI argument parsing
				parsed, err := pgurl.Parse(s.Command()[0])
				if err != nil {
					fmt.Fprintf(os.Stderr, "unable to parse connect string '%s': %+v\n", s.Command()[0], err)
					return
				}
				fmt.Fprintf(os.Stderr, "Parsed url = %#v\n", parsed)

				connURL := parsed.WithDefaultUsername(s.User())

				// TODO: consider bringing this back so we have control over the ServerHost
				//var copts clientsecopts.ClientOptions
				//copts.ServerHost = "localhost"
				//copts.User = parsed.GetUsername()
				//copts.Database = parsed.GetDatabase()
				//
				// connURL, err := clientsecopts.MakeClientConnURL(copts)
				// if err != nil {
				// 	fmt.Fprintf(os.Stderr, "Unable to make client connection URL: %+v\n", err)
				// 	return
				// }
				fmt.Fprintf(os.Stderr, "Generated url = %+v\n", connURL)

				closeFn, err := cfg.Open(theTty)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error calling cfg.Open(): %+v\n", err)
					return
				}
				defer closeFn()

				conn, err := cfg.MakeConn(connURL.ToPQ().String())
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error calling cfg.MakeConn(): %+v\n", err)
					return
				}

				if sessionPty.Window.Width >= 52 && sessionPty.Window.Height >= 30 {
					io.WriteString(theTty, roach)
				}
				io.WriteString(theTty, welcomeMessage)

				// Copy the SSH session's stdin to the PTY's stdin.
				go func() { _, _ = io.Copy(thePty, s) }()
				// Copy the PTY's stdout and stderr to the SSH session's stdout.
				go func() { _, _ = io.Copy(s, thePty) }()

				err = cfg.Run(ctx, conn)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error calling run(): %+v\n", err)
					s.Exit(255)
					return
				}
			},
		}

		netutil.FatalIfUnexpected(glSsh.Serve(sshLn))
	})
}
