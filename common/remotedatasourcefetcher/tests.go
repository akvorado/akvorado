package remotedatasourcefetcher

import "github.com/itchyny/gojq"

// MustParseTransformQuery parses a transform query or panic.
func MustParseTransformQuery(src string) TransformQuery {
	q, err := gojq.Parse(src)
	if err != nil {
		panic(err)
	}
	return TransformQuery{q}
}
