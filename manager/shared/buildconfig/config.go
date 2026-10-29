package buildconfig

import (
	"encoding/base64"
	"moo2manager/shared/distribution"
)

// Set by release engineering through -ldflags -X, never from downloaded JSON.
// This default refuses all network updates until a maintainer provisions trust.
var Encoded string

const Version = "0.5.1"

func Trust() (distribution.Trust, error) {
	t := distribution.Trust{Schema: 1, Feed: "moo2-sgc", Channel: "stable", Keys: map[string]string{}, Endpoints: []distribution.Endpoint{}, AllowedHosts: []string{}}
	if Encoded == "" {
		return t, nil
	}
	b, e := base64.StdEncoding.DecodeString(Encoded)
	if e != nil {
		return t, e
	}
	e = distribution.DecodeStrict(b, &t)
	return t, e
}
