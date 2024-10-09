package rules_test

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/TradesmanUS/LV8RLABS/rules/pkg/rules"
	"gitlab.com/accumulatenetwork/accumulate/pkg/accumulate"
	"gitlab.com/accumulatenetwork/accumulate/pkg/api/v3/jsonrpc"
	"gitlab.com/accumulatenetwork/accumulate/pkg/url"
)

func ExampleExecuteWithEndpoint() {
	client := jsonrpc.NewClient(accumulate.ResolveWellKnownEndpoint("kermit", "v3"))
	res, err := rules.Execute(context.Background(), client, &rules.Request{
		Identity: url.MustParse("FrankRagnok.acme"),
	})
	if err != nil {
		panic(err)
	}

	b, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		panic(err)
	}

	// Output:
	// {
	//   "denied": true,
	//   "denialReason": [
	//     "Certification failed"
	//   ]
	// }
	fmt.Println(string(b))
}
