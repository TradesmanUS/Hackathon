package main

import (
	"context"
	"embed"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/C3Rules/Go-DTRules/pkg/dt"
	lxml "github.com/C3Rules/Go-DTRules/pkg/legacy/xml"
	"github.com/C3Rules/Go-DTRules/pkg/vm"
	"github.com/spf13/cobra"
	"gitlab.com/accumulatenetwork/accumulate/pkg/accumulate"
	"gitlab.com/accumulatenetwork/accumulate/pkg/api/v3"
	"gitlab.com/accumulatenetwork/accumulate/pkg/api/v3/jsonrpc"
	"gitlab.com/accumulatenetwork/accumulate/pkg/url"
	"gitlab.com/accumulatenetwork/accumulate/protocol"
)

//go:embed *.xml
var files embed.FS

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

var entities = must1(loadAnd("edd.xml", lxml.EDD.Compile))
var tables = must1(loadAnd("dt.xml", lxml.DT.Compile))

type Request struct {
	Metadata *url.URL
}

type Result struct {
	Denied       bool `json:"denied"`
	DenialReason any `json:"denialReason"`
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
		var req Request
		var res *Result
		err := json.NewDecoder(r.Body).Decode(&req)
		if err != nil {
			goto error
		}

		if req.Metadata == nil {
			err = fmt.Errorf("missing metadata URL")
			goto error
		}

		res, err = execute(r.Context(), client, req.Metadata)
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

	client := jsonrpc.NewClient(accumulate.ResolveWellKnownEndpoint(flag.Network, "v3"))
	r := must1(execute(ctx, client, must1(url.Parse(args[0]))))
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	must(enc.Encode(r))
}

func execute(ctx context.Context, client api.Querier, acctUrl *url.URL) (*Result, error) {
	// Get the latest entry for the account
	Q := api.Querier2{Querier: client}
	r, err := Q.QueryDataEntry(ctx, acctUrl, &api.DataQuery{})
	if err != nil {
		return nil, fmt.Errorf("fetch latest entry for %s: %w", acctUrl, err)
	}

	var entry [][]byte
	switch body := r.Value.Message.Transaction.Body.(type) {
	case *protocol.WriteData:
		entry = body.Entry.GetData()
	case *protocol.SyntheticWriteData:
		entry = body.Entry.GetData()
	default:
		b, _ := json.Marshal(body)
		panic(fmt.Errorf("not a data transaction: %s", b))
	}

	if len(entry) == 0 {
		return nil, fmt.Errorf("latest entry is empty")
	}

	var userData struct {
		CountryCode string
	}
	err = json.Unmarshal(entry[0], &userData)
	if err != nil {
		return nil, fmt.Errorf("cannot decode entry: %w", err)
	}

	user := entities["user"].New("user")
	must(user.Set("countryCode", userData.CountryCode))

	result := entities["result"].New("result")

	s := vm.New()
	s.SetContext(ctx)
	must(s.Entity().Push(tables, user, result))

	err = vm.ExecuteString(s, "ValidateUser")
	if err != nil {
		return nil, err
	}

	return &Result{
		Denied:       getFieldAs(s, result, "denied", vm.AsBool),
		DenialReason: getFieldAs(s, result, "denialReason", vm.AsAny),
	}, nil
}

func getFieldAs[V any](s vm.State, entity *dt.Entity, name string, as func(vm.Value) (V, error)) V {
	field, _ := entity.Field(vm.LiteralName(name))
	value := must1(field.Load(s))
	return must1(as(value))
}

func loadAnd[V, U any](filename string, and func(V) (U, error)) (U, error) {
	f, err := files.Open(filename)
	if err != nil {
		var z U
		return z, err
	}
	b, err := io.ReadAll(f)
	if err != nil {
		var z U
		return z, err
	}

	var v V
	err = xml.Unmarshal(b, &v)
	err = xml.Unmarshal(b, &v)
	if err != nil {
		var z U
		return z, err
	}

	return and(v)
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
