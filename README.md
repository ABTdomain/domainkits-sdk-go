# domainkits-sdk-go

Go client for the [DomainKits](https://domainkits.com) REST API.

This is the official Go SDK for the DomainKits API, published and maintained by the DomainKits team. DomainKits is built and operated by Lyalpha GmbH, with domain data and infrastructure provided by [ABTdomain](https://abtdomain.com), our domain intelligence and data aggregation platform. This repository is hosted under the ABTdomain GitHub organisation. Learn more about the relationship at [domainkits.com/about](https://domainkits.com/about).

Every parameter, response field and current limit is documented in the [API reference](https://domainkits.com/dev/api-docs) and the [OpenAPI spec](https://domainkits.com/dev/openapi.yaml). This README only lists what the SDK covers. No dependencies outside the standard library.

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
		"query": "shop",
		"tld":   "com",
	})
	if err != nil {
		panic(err)
	}
	fmt.Println(result.Total, "matches")
	for _, d := range result.Data {
		fmt.Println(d.Domain, d.Created)
	}
}
```

Filters are passed as `Params` with the REST parameter names, verbatim.

## Endpoints

Search, each returning `*SearchResult`; `Paginate` and `Export` take the resource name as a string (`"nrds"`, `"expired"`, ...):

| Method | Endpoint |
|---|---|
| `Expired` | `/search/expired` |
| `NRDs` | `/search/nrds` |
| `NRDsLive` | `/search/nrds-live` |
| `Aged` | `/search/aged` |
| `Active` | `/search/active` |
| `Deleted` | `/search/deleted` |
| `Market` | `/search/market` |

Lookups and reports:

| Method | Endpoint |
|---|---|
| `Whois` | `/whois` |
| `DNS` | `/dns` |
| `IPLookup` | `/ip-lookup` |
| `Registrar` | `/registrar` |
| `StatusGuide` | `/status-guide` |
| `TLDCheck` | `/tld-check` |
| `Typosquat` | `/typosquat` |
| `NSReverse` | `/ns-reverse` |
| `MonitorChanges` | `/monitor/changes` |
| `HostnameSearch` | `/search/hostname` |
| `CTSubdomains` | `/ct/subdomains` |
| `CTCerts` | `/ct/certs` |
| `TLDTrends` | `/trends/tlds/*` |
| `KeywordTrends` | `/trends/keywords/*` |
| `NRDsDownload` | `/nrds/download` |
| `Usage` | `/usage` |
| `SearchStatus` | `/search/status` |
| `Health` | `/health` |

`IPLookup` returns `*IPInfo`, `Registrar` returns `*RegistrarResult` and takes `Params` for paging, `StatusGuide` returns `[]EPPStatus`, `NSReverse` returns `*NSReverseResult`. The rest return `*ListResult` or `map[string]any`.

The [API reference](https://domainkits.com/dev/api-docs) is the authority on every filter, field and limit.

**No PII.** Responses contain no registrant personal data.

## Errors

Failures come back as `*APIError` with `Status`, `Message` and the rate-limit headers. `IsRateLimit()`, `IsAuth()` and `RetryAfter()` cover the usual branches; the client retries 429 and 5xx responses on its own up to `MaxRetries` before returning the error.

```go
result, err := dk.Expired(ctx, domainkits.Params{"tld": "com"})
if err != nil {
	var apiErr *domainkits.APIError
	if errors.As(err, &apiErr) && apiErr.IsRateLimit() {
		time.Sleep(apiErr.RetryAfter())
	}
}
```

## Options

```go
dk := domainkits.New("dk_...")
dk.BaseURL = domainkits.DefaultBaseURL
dk.HTTPClient = &http.Client{Timeout: 60 * time.Second}
dk.MaxRetries = 2
```

## Resources

- [API reference and key management](https://domainkits.com/dev)
- [OpenAPI 3.0 spec](https://domainkits.com/dev/openapi.yaml)
- [About DomainKits and ABTdomain](https://domainkits.com/about)

## License

[MIT](LICENSE.md)
