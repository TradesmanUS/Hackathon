package main

import (
	"context"
	"embed"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
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
	Identity *url.URL
}

type Result struct {
	Denied       bool `json:"denied"`
	DenialReason any  `json:"denialReason"`
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

		if req.Identity == nil {
			err = fmt.Errorf("missing metadata URL")
			goto error
		}

		res, err = execute(r.Context(), client, req.Identity)
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

func execute(ctx context.Context, client api.Querier, identity *url.URL) (*Result, error) {
	// Get the latest entry for the identity's metadata
	personalBank, err := fetchDataAs[any](ctx, client, identity.JoinPath("discoveryV1ClientDefault_personalbank", "Info_V1"), &api.DataQuery{})
	if err != nil {
		return nil, fmt.Errorf("fetch personal bank metadata: %w", err)
	}

	// Extract the certificate ID
	idStr, err := getJsonField[string](
		personalBank,
		objectField("segements"),
		findValue("data").For(objectField("segmentType")),
		objectField("config"),
		objectField("dataItems"),
		findValue("primaryAml").For(objectField("target")),
		objectField("certificateUrl"),
	)
	if err != nil {
		return nil, fmt.Errorf("locate certificate ID: %w", err)
	}
	idStr = strings.TrimPrefix(idStr, "acc://")
	idBytes, err := hex.DecodeString(idStr)
	if err != nil {
		return nil, fmt.Errorf("invalid certificate ID: %w", err)
	}
	if len(idBytes) != 32 {
		return nil, fmt.Errorf("invalid certificate ID: want 32 bytes, got %d", len(idBytes))
	}

	// Find the certificate entry
	certData, err := fetchDataAs[any](ctx, client, protocol.UnknownUrl(), &api.MessageHashSearchQuery{Hash: [32]byte(idBytes)})
	if err != nil {
		return nil, fmt.Errorf("fetch certificate: %w", err)
	}

	// Extract the certificate
	cert, err := getJsonField[map[string]any](
		certData,
		objectField("segements"),
		findValue("data").For(objectField("segmentType")),
		objectField("config"),
		objectField("dataItems"),
		findValue("main").For(objectField("target")),
	)
	if err != nil {
		return nil, fmt.Errorf("locate certificate data: %w", err)
	}

	s := vm.New()
	s.SetContext(ctx)
	result := entities["result"].New("result")
	must(s.Entity().Push(
		tables,
		result,
		&jEntity{"certificate", cert},
	))

	err = vm.ExecuteString(s, "ValidateCertificate")
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

func fetchDataAs[V any](ctx context.Context, client api.Querier, account *url.URL, query api.Query) (V, error) {
	Q := api.Querier2{Querier: client}

	var txn *protocol.Transaction
	switch query := query.(type) {
	case *api.DataQuery:
		r, err := Q.QueryDataEntry(ctx, account, &api.DataQuery{})
		if err != nil {
			var z V
			return z, err
		}
		txn = r.Value.Message.Transaction

	case *api.MessageHashSearchQuery:
		r, err := Q.QueryTransaction(ctx, account.WithTxID(query.Hash), nil)
		if err != nil {
			var z V
			return z, err
		}
		txn = r.Message.Transaction
	}

	var entry [][]byte
	switch body := txn.Body.(type) {
	case *protocol.WriteData:
		entry = body.Entry.GetData()
	case *protocol.SyntheticWriteData:
		entry = body.Entry.GetData()
	default:
		var z V
		return z, fmt.Errorf("invalid transaction: want data, got %v", body.Type())
	}

	if len(entry) == 0 {
		var z V
		return z, fmt.Errorf("latest entry is empty")
	}

	var v V
	err := json.Unmarshal(entry[0], &v)
	if err != nil {
		return v, fmt.Errorf("cannot decode entry: %w", err)
	}
	return v, nil
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
