package utils

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"k8s.io/apimachinery/pkg/util/duration"
)

func Age(d time.Duration) string {
	if d.Milliseconds() == 0 {
		return "0ms"
	}
	if d.Milliseconds() < 1000 {
		return fmt.Sprintf("%0.dms", d.Milliseconds())
	}
	return duration.HumanDuration(d)
}

// SetDifference returns the list of elements present in a but not present in b
func SetDifference[T comparable](a, b []T) []T {
	mb := make(map[T]struct{}, len(b))
	for _, x := range b {
		mb[x] = struct{}{}
	}
	var diff []T
	for _, x := range a {
		if _, found := mb[x]; !found {
			diff = append(diff, x)
		}
	}
	return diff
}

// GenerateJSONMD5Hash marshals the object into JSON and generates its md5 hash
func GenerateJSONMD5Hash(obj interface{}) (string, error) {
	data, err := json.Marshal(obj)
	if err != nil {
		return "", err
	}
	hash := md5.Sum(data)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(hash[:]), nil
}

func Contains[T comparable](arr []T, item T) bool {
	for _, elem := range arr {
		if elem == item {
			return true
		}
	}
	return false

func UUIDsToStrings(in []uuid.UUID) []string {
	out := make([]string, len(in))
	for i, u := range in {
		out[i] = u.String()
	}

	return out
}
