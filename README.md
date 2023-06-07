# p2pquic

QUIC/ICE demo using quic-go and pion/ice. Based on the [quic-go echo
example](https://github.com/quic-go/quic-go/blob/master/example/echo/echo.go)
and the [pion/ice
example](https://github.com/pion/ice/tree/master/examples/ping-pong).

## Build and Run

Start two terminals. In the first, run:

```shell
go run cmd/main.go -server -ice
```

In the second one run:

```shell
go run cmd/main.go -ice -local 9001 -remote 9000
```

When both processes are running, press enter in both terminals. You should see
some log messages about ICE connection states and candidates. When the
connection is established, the QUIC client sends the message `foobar`, which is
then echoed back by the server.

