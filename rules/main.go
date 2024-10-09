package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/TradesmanUS/LV8RLABS/rules/pkg/rules"
	"github.com/spf13/cobra"
	"gitlab.com/accumulatenetwork/accumulate/pkg/accumulate"
	"gitlab.com/accumulatenetwork/accumulate/pkg/api/v3/jsonrpc"
	"gitlab.com/accumulatenetwork/accumulate/pkg/url"
)

var flag = struct {
	Network string
}{}

var cmd = &cobra.Command{
	Use:   "rules [listen]",
	Short: "Execute the rules engine",
	Args:  cobra.ExactArgs(1),
	Run:   run,
}

var cmdOnce = &cobra.Command{
	Use:   "once [account]",
	Short: "Execute the rules engine",
	Args:  cobra.ExactArgs(1),
	Run:   runOnce,
}

func main() {
	cmd.AddCommand(cmdOnce)
	cmd.PersistentFlags().StringVarP(&flag.Network, "network", "n", "https://mainnet.accumulatenetwork.io", "The Accumulate network endpoint")
	_ = cmd.Execute()
}

func run(_ *cobra.Command, args []string) {
	// Handle SIGINT (CTRL+C) gracefully.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	l := must1(net.Listen("tcp", args[0]))
	defer l.Close()
	fmt.Println("Listening on", l.Addr())

	client := jsonrpc.NewClient(accumulate.ResolveWellKnownEndpoint(flag.Network, "v3"))
	s := http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req *rules.Request
		var res *rules.Result
		err := json.NewDecoder(r.Body).Decode(&req)
		if err != nil {
			goto error
		}

		if req.Identity == nil {
			err = fmt.Errorf("missing metadata URL")
			goto error
		}

		res, err = rules.Execute(r.Context(), client, req)
		if err != nil {
			goto error
		}

		w.Header().Add("Content-Type", "application/json")
		json.NewEncoder(w).Encode(res)
		return

	error:
		slog.DebugContext(r.Context(), "Request failed", "error", err)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(struct{ Error string }{Error: err.Error()})
	})}
	go func() {
		<-ctx.Done()
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
		defer cancel()
		must(s.Shutdown(ctx))
	}()

	err := s.Serve(l)
	if err != nil {
		fmt.Println(err)
	}

	<-ctx.Done()
	fmt.Println("Shutting down")
}

func runOnce(_ *cobra.Command, args []string) {
	// Handle SIGINT (CTRL+C) gracefully.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	req := &rules.Request{
		Identity: must1(url.Parse(args[0])),
	}

	client := jsonrpc.NewClient(accumulate.ResolveWellKnownEndpoint(flag.Network, "v3"))
	r := must1(rules.Execute(ctx, client, req))
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	must(enc.Encode(r))
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func must1[V any](v V, err error) V {
	must(err)
	return v
}
