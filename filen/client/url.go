package client

import (
	"math/rand"
	"strings"
)

var (
	gatewayURLs = []string{
		"https://gateway.filen.io",
		/*"https://gateway.filen.net",
		"https://gateway.filen-1.net",
		"https://gateway.filen-2.net",
		"https://gateway.filen-3.net",
		"https://gateway.filen-4.net",
		"https://gateway.filen-5.net",
		"https://gateway.filen-6.net",*/
	}
	egestURLs = []string{
		"https://egest.filen.io",
		/*"https://egest.filen.net",
		"https://egest.filen-1.net",
		"https://egest.filen-2.net",
		"https://egest.filen-3.net",
		"https://egest.filen-4.net",
		"https://egest.filen-5.net",
		"https://egest.filen-6.net",*/
	}
	ingestURLs = []string{
		"https://ingest.filen.io",
		/*"https://ingest.filen.net",
		"https://ingest.filen-1.net",
		"https://ingest.filen-2.net",
		"https://ingest.filen-3.net",
		"https://ingest.filen-4.net",
		"https://ingest.filen-5.net",
		"https://ingest.filen-6.net",*/
	}
)

const (
	URLTypeIngest  = 1
	URLTypeEgest   = 2
	URLTypeGateway = 3
)

type FilenURL struct {
	Type      int
	Path      string
	CachedUrl string
}

func GatewayURL(path string) *FilenURL {
	return &FilenURL{
		Type:      URLTypeGateway,
		Path:      path,
		CachedUrl: "",
	}
}

func (url *FilenURL) String() string {
	if url.CachedUrl == "" {
		var builder strings.Builder
		switch url.Type {
		case URLTypeIngest:
			builder.WriteString(ingestURLs[rand.Intn(len(ingestURLs))])
		case URLTypeEgest:
			builder.WriteString(egestURLs[rand.Intn(len(egestURLs))])
		case URLTypeGateway:
			builder.WriteString(gatewayURLs[rand.Intn(len(gatewayURLs))])
		}
		builder.WriteString(url.Path)
		url.CachedUrl = builder.String()
	}

	return url.CachedUrl
}
