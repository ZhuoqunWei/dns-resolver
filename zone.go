package main

import "strings"

type Zone struct {
	Origin  string
	Records map[string]ARecord
}

func (z Zone) contains(name string) bool {
	name = canonicalName(name)
	origin := canonicalName(z.Origin)
	if origin == "" {
		return false
	}

	return name == origin || strings.HasSuffix(name, "."+origin)
}

func (z Zone) nameExists(name string) bool {
	name = canonicalName(name)
	if name == canonicalName(z.Origin) {
		return true
	}

	_, exists := z.Records[name]
	return exists
}
