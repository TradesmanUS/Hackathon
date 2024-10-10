package rules

import (
	"context"
	"embed"
	"encoding/xml"
	"fmt"
	"io"

	"github.com/C3Rules/Go-DTRules/pkg/dt"
	lxml "github.com/C3Rules/Go-DTRules/pkg/legacy/xml"
	"github.com/C3Rules/Go-DTRules/pkg/vm"
	"gitlab.com/accumulatenetwork/accumulate/pkg/accumulate"
	"gitlab.com/accumulatenetwork/accumulate/pkg/api/v3"
	"gitlab.com/accumulatenetwork/accumulate/pkg/api/v3/jsonrpc"
	"gitlab.com/accumulatenetwork/accumulate/pkg/url"
)

//go:embed *.xml
var files embed.FS

var entities = must1(loadAnd("compiled_edd.xml", lxml.EDD.Compile))
var tables = must1(loadAnd("compiled_dt.xml", lxml.DT.Compile))

type Request struct {
	Identity *url.URL
}

type Result struct {
	Denied       bool `json:"denied"`
	DenialReason any  `json:"denialReason"`
}

func Main(endpoint string, adi string) (*Result, error) {
	client := jsonrpc.NewClient(accumulate.ResolveWellKnownEndpoint(endpoint, "v3"))
	return Execute(context.Background(), client, &Request{
		Identity: url.MustParse(adi),
	})
}

func Execute(ctx context.Context, client api.Querier, req *Request) (*Result, error) {
	if req.Identity == nil {
		return nil, fmt.Errorf("missing metadata URL")
	}

	id, err := FetchAmlCertID(ctx, client, req.Identity)
	if err != nil {
		return nil, err
	}

	cert, err := FetchAmlCert(ctx, client, id)
	if err != nil {
		return nil, err
	}

	s := vm.New()
	s.SetContext(ctx)
	result := entities["result"].New("result")
	must(s.Entity().Push(tables, result, cert))

	err = vm.ExecuteString(s, "Validate_Certificate")
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
