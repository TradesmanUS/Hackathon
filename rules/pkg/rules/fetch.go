package rules

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/C3Rules/Go-DTRules/pkg/vm"
	"gitlab.com/accumulatenetwork/accumulate/pkg/api/v3"
	"gitlab.com/accumulatenetwork/accumulate/pkg/url"
	"gitlab.com/accumulatenetwork/accumulate/protocol"
)

func FetchAmlCertID(ctx context.Context, client api.Querier, identity *url.URL) ([32]byte, error) {
	// Get the latest entry for the identity's metadata
	personalBank, err := fetchDataAs[any](ctx, client, identity.JoinPath("discoveryV1ClientDefault_personalbank", "Info_V1"), &api.DataQuery{})
	if err != nil {
		return [32]byte{}, fmt.Errorf("fetch personal bank metadata: %w", err)
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
		return [32]byte{}, fmt.Errorf("locate certificate ID: %w", err)
	}
	idStr = strings.TrimPrefix(idStr, "acc://")
	idBytes, err := hex.DecodeString(idStr)
	if err != nil {
		return [32]byte{}, fmt.Errorf("invalid certificate ID: %w", err)
	}
	if len(idBytes) != 32 {
		return [32]byte{}, fmt.Errorf("invalid certificate ID: want 32 bytes, got %d", len(idBytes))
	}

	return [32]byte(idBytes), nil
}

func FetchAmlCert(ctx context.Context, client api.Querier, id [32]byte) (vm.Entity, error) {
	// Find the certificate entry
	certData, err := fetchDataAs[any](ctx, client, protocol.UnknownUrl(), &api.MessageHashSearchQuery{Hash: id})
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
	return &jEntity{"certificate", cert}, nil
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
