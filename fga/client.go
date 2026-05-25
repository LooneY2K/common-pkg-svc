// client.go is the process-level wrapper around the OpenFGA Go SDK.
//
// Why a wrapper?
//
//  1. The raw `openfga/go-sdk` client exposes a fluent builder API that is
//     awkward to call from clean-arch services and impossible to mock without
//     interfaces. We expose a small interface (`Client`) the application /
//     middleware layer can depend on; the production type (`*SDKClient`)
//     talks to the real SDK, while unit tests inject a fake.
//
//  2. Every call here pins the `AuthorizationModelId` (read from
//     `FGA_MODEL_ID`). This is mandatory per the rollout protocol in
//     `docs/authorization-model.md` §6 — services must NOT silently follow the
//     latest uploaded model. Pinning at the wrapper level means individual
//     services cannot accidentally omit it.
//
//  3. Seven call shapes cover all current needs (WriteTuple, WriteTuples,
//     DeleteTuple, Check, BatchCheck, ListObjects, Read). SCRUM-121 added
//     Read to support the reverse-lookup required by ListProjectMembers.
//     Anything more exotic should either extend this file (with the same
//     pinning guarantee) or be raised for review.
package fga

import (
	"context"
	"fmt"

	openfga "github.com/openfga/go-sdk"
	"github.com/openfga/go-sdk/client"
	"github.com/openfga/go-sdk/credentials"
)

// Client is what the application layer depends on. All methods take a
// context.Context so callers can propagate cancellation / deadlines / OTel
// spans into the underlying HTTP call.
type Client interface {
	WriteTuple(ctx context.Context, user, relation, object string) error
	WriteTuples(ctx context.Context, tuples []TupleRequest) error
	DeleteTuple(ctx context.Context, user, relation, object string) error
	Check(ctx context.Context, user, relation, object string) (bool, error)
	BatchCheck(ctx context.Context, user string, checks []CheckRequest) (map[string]bool, error)
	ListObjects(ctx context.Context, user, relation, objectType string) ([]string, error)
	// Read returns all tuples matching the partial filter. To list all members
	// of an object, pass Object only (leave User/Relation empty). Pages are
	// walked automatically via the SDK's continuation token; the hard cap is
	// 1000 total tuples to prevent runaway calls on large stores.
	Read(ctx context.Context, filter ReadFilter) ([]TupleRecord, error)
}

// SDKClient is the production implementation of Client. It owns one
// `*client.OpenFgaClient` instance for the process lifetime — the SDK client
// is goroutine-safe and amortises an HTTP transport + connection pool.
type SDKClient struct {
	sdk     *client.OpenFgaClient
	modelID string
}

// Compile-time guarantee that *SDKClient satisfies the interface. If this
// breaks (e.g. you change a method signature), the build fails here with a
// readable error instead of producing a runtime DI failure deep inside main.go.
var _ Client = (*SDKClient)(nil)

// fgaWriteBatchLimit is the per-call cap on tuples in a single Write request.
// The OpenFGA server caps this at 100 transactional writes; exceeding it
// returns HTTP 400. We chunk WriteTuples calls to stay under this limit.
const fgaWriteBatchLimit = 100

// NewClient constructs a wrapper around the OpenFGA SDK. All four parameters
// are required; missing values produce an error rather than a degraded
// client, which would otherwise surface much later as a confusing
// "store not found" 404 on the first Check.
//
// Token authentication: OpenFGA's "preshared key" mode is the only auth flow
// we support in dev/prod (OIDC client-credentials is on the SDK but not
// configured here). The token comes from `FGA_AUTH_TOKEN`.
func NewClient(apiURL, storeID, modelID, authToken string) (*SDKClient, error) {
	if apiURL == "" {
		return nil, fmt.Errorf("fga: apiURL is required")
	}
	if storeID == "" {
		return nil, fmt.Errorf("fga: storeID is required")
	}
	if modelID == "" {
		return nil, fmt.Errorf("fga: modelID is required (pinning is mandatory)")
	}
	if authToken == "" {
		return nil, fmt.Errorf("fga: authToken is required")
	}

	cfg := &client.ClientConfiguration{
		ApiUrl:               apiURL,
		StoreId:              storeID,
		AuthorizationModelId: modelID,
		Credentials: &credentials.Credentials{
			Method: credentials.CredentialsMethodApiToken,
			Config: &credentials.Config{
				ApiToken: authToken,
			},
		},
	}

	sdk, err := client.NewSdkClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("fga: build sdk client: %w", err)
	}

	return &SDKClient{sdk: sdk, modelID: modelID}, nil
}

// WriteTuple inserts a single relationship tuple. Returns an error if the
// underlying call fails. Duplicate-write errors from OpenFGA surface as a
// transport error and are propagated to the caller — the membership service
// catches them and translates to 409 Conflict.
func (c *SDKClient) WriteTuple(ctx context.Context, user, relation, object string) error {
	return c.WriteTuples(ctx, []TupleRequest{{User: user, Relation: relation, Object: object}})
}

// WriteTuples writes a batch of tuples. Calls are chunked at
// fgaWriteBatchLimit (100) — the SDK does not auto-chunk on its own. Any
// chunk that errors aborts the operation; partial success is not unwound
// (OpenFGA transactional Write is atomic per chunk but NOT across chunks).
// Callers that need fully-atomic multi-tuple writes must keep batch ≤ 100.
func (c *SDKClient) WriteTuples(ctx context.Context, tuples []TupleRequest) error {
	if len(tuples) == 0 {
		return nil
	}

	for start := 0; start < len(tuples); start += fgaWriteBatchLimit {
		end := start + fgaWriteBatchLimit
		if end > len(tuples) {
			end = len(tuples)
		}

		chunk := tuples[start:end]
		body := make(client.ClientWriteTuplesBody, 0, len(chunk))
		for _, t := range chunk {
			body = append(body, openfga.TupleKey{
				User:     t.User,
				Relation: t.Relation,
				Object:   t.Object,
			})
		}

		_, err := c.sdk.WriteTuples(ctx).Body(body).Execute()
		if err != nil {
			return fmt.Errorf("fga: write tuples [%d..%d]: %w", start, end, err)
		}
	}

	return nil
}

// DeleteTuple removes a single relationship tuple. OpenFGA returns success
// only if the tuple exists; deletion of an absent tuple surfaces as an SDK
// error. Idempotent-delete semantics (treating "tuple not found" as success)
// are the caller's responsibility — see the membership repository's
// RevokeRole for the established pattern.
func (c *SDKClient) DeleteTuple(ctx context.Context, user, relation, object string) error {
	body := client.ClientDeleteTuplesBody{
		openfga.TupleKeyWithoutCondition{
			User:     user,
			Relation: relation,
			Object:   object,
		},
	}
	_, err := c.sdk.DeleteTuples(ctx).Body(body).Execute()
	if err != nil {
		return fmt.Errorf("fga: delete tuple (%s, %s, %s): %w", user, relation, object, err)
	}
	return nil
}

// Check evaluates a single permission. Returns true if `user` has `relation`
// on `object`, false otherwise. The model id is pinned by the constructor;
// callers cannot override it.
func (c *SDKClient) Check(ctx context.Context, user, relation, object string) (bool, error) {
	resp, err := c.sdk.Check(ctx).Body(client.ClientCheckRequest{
		User:     user,
		Relation: relation,
		Object:   object,
	}).Execute()
	if err != nil {
		return false, fmt.Errorf("fga: check (%s, %s, %s): %w", user, relation, object, err)
	}
	return resp.GetAllowed(), nil
}

// BatchCheck evaluates many permission checks in a single round trip. Used by
// `GET /auth/permissions` to fan out all 9 computed permissions on a project
// for a single user. Returns a map keyed by "{relation}:{object}" so callers
// can look up results without re-pairing them with the request list.
//
// Correlation ids: the OpenFGA BatchCheck API returns results keyed by a
// user-supplied correlation id. We synthesise one per check ("c{index}") and
// translate the result map back to relation:object keys before returning.
func (c *SDKClient) BatchCheck(ctx context.Context, user string, checks []CheckRequest) (map[string]bool, error) {
	results := make(map[string]bool, len(checks))
	if len(checks) == 0 {
		return results, nil
	}

	items := make([]client.ClientBatchCheckItem, 0, len(checks))
	indexToKey := make(map[string]string, len(checks))

	for i, ch := range checks {
		correlationID := fmt.Sprintf("c%d", i)
		items = append(items, client.ClientBatchCheckItem{
			User:          user,
			Relation:      ch.Relation,
			Object:        ch.Object,
			CorrelationId: correlationID,
		})
		indexToKey[correlationID] = fmt.Sprintf("%s:%s", ch.Relation, ch.Object)
	}

	resp, err := c.sdk.BatchCheck(ctx).Body(client.ClientBatchCheckRequest{Checks: items}).Execute()
	if err != nil {
		return nil, fmt.Errorf("fga: batch check (user=%s, n=%d): %w", user, len(checks), err)
	}

	for cid, res := range resp.GetResult() {
		key, ok := indexToKey[cid]
		if !ok {
			continue
		}
		if res.Allowed != nil {
			results[key] = *res.Allowed
		} else {
			results[key] = false
		}
	}

	// Ensure every requested check has a deterministic entry (defaults to
	// false). Without this, downstream callers that range over the expected
	// permission keys would see "missing" entries on partial responses.
	for _, key := range indexToKey {
		if _, present := results[key]; !present {
			results[key] = false
		}
	}

	return results, nil
}

// ListObjects returns the set of object IDs of type `objectType` on which the
// given user has the given relation. Used by the membership service to
// enumerate project memberships (which users have any role on this project)
// and to support the FE "my projects" view in SCRUM-96.
//
// OpenFGA's ListObjects returns fully-qualified object identifiers ("project:
// {id}"). We do NOT strip the prefix here — the membership service needs the
// type for repository lookups, and stripping would forfeit type information.
// Note: ListObjects performance scales with the number of tuples the user has
// in the store; for read-heavy use cases consider StreamedListObjects (not
// wired here because the membership service operates on bounded result sets).
func (c *SDKClient) ListObjects(ctx context.Context, user, relation, objectType string) ([]string, error) {
	resp, err := c.sdk.ListObjects(ctx).Body(client.ClientListObjectsRequest{
		User:     user,
		Relation: relation,
		Type:     objectType,
	}).Execute()
	if err != nil {
		return nil, fmt.Errorf("fga: list objects (user=%s, relation=%s, type=%s): %w", user, relation, objectType, err)
	}
	return resp.GetObjects(), nil
}

// fgaReadHardCap is the maximum number of tuples Read will accumulate across
// all pages. Prevents runaway pagination on stores with very large tuple sets
// (e.g. a project with thousands of members — not expected but safe to cap).
const fgaReadHardCap = 1000

// fgaReadPageSize is the number of tuples requested per Read page. OpenFGA
// allows up to 100; we use 100 to minimise round-trips.
const fgaReadPageSize = 100

// Read returns all tuples matching the partial filter, walking pages
// automatically via the continuation token returned by OpenFGA. Stops at
// fgaReadHardCap (1000) tuples and logs a warning if the cap is reached —
// callers should treat a capped result as potentially incomplete and plan
// accordingly.
//
// Filter semantics (all fields optional):
//   - Filter.User     — pin to a specific user (e.g. "user:abc123")
//   - Filter.Relation — pin to a specific relation (e.g. "survey_manager")
//   - Filter.Object   — pin to a specific object (e.g. "project:xyz"); this
//     is the primary use-case for listing all members of a project.
//
// The returned TupleRecords preserve the raw FGA string format for User,
// Relation, and Object — the membership repository is responsible for
// stripping type prefixes before returning domain types to the service layer.
func (c *SDKClient) Read(ctx context.Context, filter ReadFilter) ([]TupleRecord, error) {
	var (
		results           []TupleRecord
		continuationToken string
	)

	pageSize := int32(fgaReadPageSize)

	for {
		body := client.ClientReadRequest{}
		if filter.User != "" {
			body.User = &filter.User
		}
		if filter.Relation != "" {
			body.Relation = &filter.Relation
		}
		if filter.Object != "" {
			body.Object = &filter.Object
		}

		opts := client.ClientReadOptions{
			PageSize: &pageSize,
		}
		if continuationToken != "" {
			opts.ContinuationToken = &continuationToken
		}

		resp, err := c.sdk.Read(ctx).Body(body).Options(opts).Execute()
		if err != nil {
			return nil, fmt.Errorf("fga: read (filter=%+v, token=%q): %w", filter, continuationToken, err)
		}

		for _, t := range resp.GetTuples() {
			k := t.GetKey()
			results = append(results, TupleRecord{
				User:      k.GetUser(),
				Relation:  k.GetRelation(),
				Object:    k.GetObject(),
				Timestamp: t.GetTimestamp(),
			})
			if len(results) >= fgaReadHardCap {
				// Safety cap reached — log and return what we have.
				// Using fmt.Fprintf to stderr because SDKClient does not
				// hold a logger. Callers that need structured logging should
				// check len(result) == fgaReadHardCap on return.
				fmt.Printf("fga: Read hard cap (%d) reached for filter %+v — results may be incomplete\n", fgaReadHardCap, filter)
				return results, nil
			}
		}

		token := resp.GetContinuationToken()
		if token == "" {
			break
		}
		continuationToken = token
	}

	return results, nil
}
