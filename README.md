# domainkits-sdk-go

Go client for the [DomainKits](https://domainkits.com) REST API. Tracks API version 0.3.5.

DomainKits is one API with a shared key across every endpoint. This package covers all of them, six domain search types, WHOIS, DNS, reverse nameserver, Certificate Transparency, safety, trends and bulk download, with automatic paging and rate-limit aware retries. No dependencies outside the standard library.

## Requirements

The REST API is for Premium and Platinum accounts; unauthenticated requests are rejected with 401. Keys start with `dk_` and come from [domainkits.com](https://domainkits.com/pricing).

## Install

```bash
go get github.com/ABTdomain/domainkits-sdk-go
```

## Usage

```go
package main

import (
	"context"
	"fmt"
	"os"

	domainkits "github.com/ABTdomain/domainkits-sdk-go"
)

func main() {
	dk := domainkits.New(os.Getenv("DOMAINKITS_API_KEY"))

	result, err := dk.NRDs(context.Background(), domainkits.Params{
		"query":             "shop",
		"tld":               "com",
		"create_date_start": "2026-07-10",
		"create_date_end":   "2026-07-10",
		"has_number":        "false",
	})
	if err != nil {
		panic(err)
	}

	fmt.Printf("%d matches\n", result.Total)
	for _, d := range result.Data {
		fmt.Println(d.Domain, d.Created, d.Expires)
	}
}
```

### Paging

A single request returns at most 500 results. `Paginate` walks the whole result set; return `false` from the callback to stop early:

```go
err := dk.Paginate(ctx, "nrds", domainkits.Params{"query": "shop", "tld": "com"}, func(d domainkits.Domain) bool {
	fmt.Println(d.Domain)
	return true
})
```

### Export

`Export` pulls up to 50,000 rows as CSV in one request:

```go
csv, err := dk.Export(ctx, "expired", domainkits.Params{"tld": "com", "status": "pending_delete"})
```

This runs on a separate, much smaller quota with a monthly cap; call `Usage` for your account's numbers. It is for occasional bulk pulls, not for a scheduled job.

## Search types

| Method | What it searches |
|---|---|
| `NRDs` | Newly registered domains, last 60 days, from the zone files |
| `NRDsLive` | Newly registered domains, last 3 days, from Certificate Transparency |
| `Expired` | Domains in the deletion cycle: expired, redemption, pending delete |
| `Aged` | Domains with 5 to 20+ years of registration history |
| `Active` | Currently registered domains |
| `Deleted` | Dropped domains |
| `Market` | Domains listed for sale on marketplaces |

Filters are passed as `Params` and match the REST parameter names. The vocabulary follows a fixed grammar:

- Numeric ranges are `_min`/`_max` pairs (`length_min`/`length_max`, `age_min`/`age_max`), date ranges are `_start`/`_end` pairs (`create_date_start`/`create_date_end`, `found_date_start`/`found_date_end`); either side may be omitted, equal bounds select an exact value.
- Composition filters are booleans that accept only the literals `true`/`false`: `has_number`, `all_number`, `all_alpha`, `has_hyphen`, `has_sale`.
- `query` matches a substring of the name portion; `position` (`start`/`end`/`middle`) narrows where and requires `query`. `exclude_query` takes comma-separated negative keywords. `tld` is comma-separated on the search endpoints.
- Unrecognized parameter names and values return 400, and the error message lists what the endpoint supports, so a failed call tells you how to fix it.

The full parameter and field reference per endpoint is the [OpenAPI spec](https://domainkits.com/dev/openapi.yaml) and the [API docs](https://domainkits.com/dev/api-docs); this README does not duplicate it.

### NRDs and NRDsLive

Two registration feeds, read from different places, so they answer different questions.

`NRDs` reads the zone files and holds 60 days. It is the complete view for the generic TLDs and the one to use for anything that looks back more than a few days.

`NRDsLive` holds the last 3 days with live updates, so it surfaces names registered hours ago that `NRDs` cannot show yet, and it also reaches `.ai` and `.io`. Same parameter vocabulary; `tld` takes a single value there, `Created`/`Expires` carry full timestamps rather than dates, rows never fill `TLDCount`, and the endpoint runs on a smaller per-minute quota. `Paginate` and `Export` take the resource as a string, so pass `"nrds-live"` there.

## Other endpoints

```go
dk.Whois(ctx, "example.com")
dk.DNS(ctx, "example.com")
dk.Safety(ctx, "example.com")
dk.NSReverse(ctx, "ns1.example.com", domainkits.Params{"tld": "com"})
dk.TLDCheck(ctx, "yourbrand", nil)
dk.Typosquat(ctx, "example.com", nil)
dk.IPLookup(ctx, "8.8.8.8")
dk.Registrar(ctx, "godaddy")
dk.StatusGuide(ctx, "clientHold")
dk.MonitorChanges(ctx, domainkits.Params{"tld": "com", "reason": "transfer"})
dk.CTSubdomains(ctx, "example.com", nil)
dk.CTCerts(ctx, domainkits.Params{"domain": "example.com"})
dk.CTSearch(ctx, "example", nil)
dk.TLDTrends(ctx, "newly", domainkits.Params{"tld": "com"})
dk.KeywordTrends(ctx, "hot", nil)
dk.Usage(ctx)
dk.SearchStatus(ctx)
```

## Coverage

**gTLDs only** for the zone based domain search endpoints. The index covers generic TLDs: `.com`, `.net`, `.org`, `.info`, `.biz`, `.xyz`, `.online`, `.site`, `.top`, `.club`, `.live`, `.app`, `.dev` and others. Country-code TLDs are not indexed: a query for `.de`, `.co` or `.us` returns an empty result set, not an error. `NRDsLive` is the exception and also carries `.ai` and `.io`.

`Whois`, `DNS`, `Safety`, `IPLookup` and the Certificate Transparency endpoints work on any domain, ccTLDs included.

**No PII.** Responses contain no personal data. WHOIS results are limited to registrar, dates, status codes and nameservers; registrant names, emails, addresses and phone numbers are not returned.

## Errors

```go
result, err := dk.Expired(ctx, domainkits.Params{"tld": "com"})
if err != nil {
	var apiErr *domainkits.APIError
	if errors.As(err, &apiErr) {
		if apiErr.IsRateLimit() {
			fmt.Println("quota exhausted, retry in", apiErr.RetryAfter())
		}
		fmt.Println(apiErr.Status, apiErr.Message, apiErr.RateLimit)
	}
}
```

Every error carries the `x-ratelimit-limit`, `x-ratelimit-remaining` and `x-ratelimit-reset` values as a parsed `RateLimit`. 429 and 5xx responses are retried automatically, twice by default, waiting until the rate-limit window resets when that is under two minutes. Set `MaxRetries` to 0 to handle it yourself.

## Options

```go
dk := domainkits.New(os.Getenv("DOMAINKITS_API_KEY"))
dk.BaseURL = "https://premium-api.domainkits.com/api/v1"
dk.MaxRetries = 2
dk.HTTPClient = &http.Client{Timeout: 60 * time.Second}
```

## Quotas

Call `Usage` for the live picture on your account; every endpoint reports its own per-minute, daily and monthly allowance alongside what you have already spent.

Daily quotas reset at 00:00 UTC, monthly quotas on the 1st. Current limits per plan: [domainkits.com/pricing](https://domainkits.com/pricing).

## Resources

- [DomainKits API reference](https://domainkits.com/dev/api-docs)
- [@domainkits/sdk](https://www.npmjs.com/package/@domainkits/sdk), the same API for TypeScript
- [domainkits](https://pypi.org/project/domainkits/), the same API for Python
- [n8n-nodes-domainkits](https://www.npmjs.com/package/n8n-nodes-domainkits), the same API for n8n

## License

[MIT](LICENSE.md)
