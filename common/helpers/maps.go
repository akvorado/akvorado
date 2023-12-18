package helpers

import "reflect"

// IsMapSubset validates that a map is a proper subset of another parent map.
func IsMapSubset(mapSet interface{}, mapSubset interface{}) bool {

	mapSetValue := reflect.ValueOf(mapSet)
	mapSubsetValue := reflect.ValueOf(mapSubset)

	if mapSetValue.Kind() != reflect.Map || mapSubsetValue.Kind() != reflect.Map {
		return false
	}
	if reflect.TypeOf(mapSetValue) != reflect.TypeOf(mapSubsetValue) {
		return false
	}
	if len(mapSubsetValue.MapKeys()) == 0 {
		return true
	}

	iterMapSubset := mapSubsetValue.MapRange()

	for iterMapSubset.Next() {
		k := iterMapSubset.Key()
		v := iterMapSubset.Value()

		if v2 := mapSetValue.MapIndex(k); !v2.IsValid() || v.Interface() != v2.Interface() {
			return false
		}
	}
	return true
}
