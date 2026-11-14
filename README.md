# domainkits-sdk-go

Go client for the [DomainKits](https://domainkits.com) domain data API. It runs on the standard library, with no other dependencies.

## What it does

It brings domain data into your Go code, so your services, scheduled jobs and pipelines can search it, look it up and download it as part of their own logic:

- **Lifecycle**: [newly registered domains](https://github.com/ABTdomain/newly-registered-domains) as a search or as the daily list, [expired domains](https://github.com/ABTdomain/expired-domains) as a search or as an exported file, and deleted, aged, active and for-sale domains
- **Registration**: WHOIS data, the registrar, and what each EPP status code means
- **DNS and hosting**: DNS records, the network behind an IP, and other domains on the same nameserver
- **Certificates**: subdomains, certificates and newly seen hostnames from CT logs
- **Footprint**: the same name across TLDs, and typosquat variants around a brand
- **Trends**: keyword and TLD registration trends
- **Changes**: transfers, drops, renewals and nameserver updates on high-value domains

▶ [See how it is used](#use-it-for)

## Endpoints

Search, each returning `*SearchResult`; `Paginate` and `Export` take the resource name as a string (`"nrds"`, `"expired"`, ...):

| Method | Endpoint |
|---|---|
| `Expired` | `/search/expired` |
| `NRDs` | `/search/nrds` |
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

Filters, fields and limits for each endpoint are in the [API reference](https://domainkits.com/dev/api-docs).

**No PII.** Responses contain no registrant personal data.

## Install

```bash
go get github.com/ABTdomain/domainkits-sdk-go
```

Set your key and make a call:

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

The same key works for the REST API, the SDKs, the MCP server and the CLI.

Sign up at [domainkits.com](https://domainkits.com/register) and create a key on your [dashboard](https://domainkits.com/dashboard). `NRDsDownload` works on a free account; the other calls need a plan that includes the API, see the [API reference](https://domainkits.com/dev/api-docs).

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

## Use it for

### [Newly registered domains](https://github.com/ABTdomain/newly-registered-domains)

```go
dk.NRDs(ctx, domainkits.Params{"query": "ai", "tld": "com"})                   // search by keyword
dk.NRDsDownload(ctx, domainkits.Params{"tld": "com", "type": "domains_only"})  // pull the daily list
```

<p align="center">
<a href="https://www.youtube.com/watch?v=8bXfKV5-eGw"><img src="https://img.youtube.com/vi/8bXfKV5-eGw/maxresdefault.jpg" alt="Video: How to get a newly registered domains feed with the DomainKits API" width="640"></a><br>
<a href="https://www.youtube.com/watch?v=8bXfKV5-eGw">▶ Watch on YouTube: How to Get a Newly Registered Domains Feed with an API</a>
</p>

### [Expired domains](https://github.com/ABTdomain/expired-domains)

```go
dk.Expired(ctx, domainkits.Params{"query": "shop", "status": "pending_delete", "age_min": "10"})  // pending delete names with a long history
dk.Export(ctx, "expired", domainkits.Params{"tld": "app", "status": "redemption"})                // the redemption stage of one TLD as CSV
```

## Related

- [ABTdomain/domainkits-sdk-py](https://github.com/ABTdomain/domainkits-sdk-py): Python client for the DomainKits domain data API
- [ABTdomain/domainkits-sdk-js](https://github.com/ABTdomain/domainkits-sdk-js): TypeScript client for the DomainKits domain data API
- [ABTdomain/domainkits-cli](https://github.com/ABTdomain/domainkits-cli): command-line client for the DomainKits domain data API
- [ABTdomain/domainkits-mcp](https://github.com/ABTdomain/domainkits-mcp): hosted MCP server for domain data

## Resources

- [API reference and key management](https://domainkits.com/dev)
- [OpenAPI 3.0 spec](https://domainkits.com/dev/openapi.yaml)
- [About DomainKits and ABTdomain](https://domainkits.com/about)

## License

[MIT](LICENSE.md)
