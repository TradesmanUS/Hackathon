# AML Rules Executor

Build with `./rules/build.bash`. Execute once:
```shell
$ ./bin/rules once --network=kermit FrankRagnok.acme
{
  "denied": true,
  "denialReason": [
    "Certification failed"
  ]
}
```

Run as a server:
```shell
$ ./bin/rules --network=kermit :8080
Listening on [::]:8080

$ curl localhost:8080 --data-raw '{"identity": "FrankRagnok.acme"}'
{
  "denied": true,
  "denialReason": [
    "Certification failed"
  ]
}
```

Call directly:
```go
package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/TradesmanUS/LV8RLABS/rules/pkg/rules"
	"gitlab.com/accumulatenetwork/accumulate/pkg/url"
)

func main() {
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
```