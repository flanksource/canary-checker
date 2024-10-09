package utils

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"path/filepath"
	"sync"
	"time"

	"github.com/flanksource/commons/logger"

	"github.com/google/uuid"
	"golang.org/x/sync/semaphore"
	"k8s.io/apimachinery/pkg/util/duration"
)

type NamedLock struct {
	locks sync.Map
}

type Unlocker interface {
	Release()
}

type unlocker struct {
	lock *semaphore.Weighted
}

func (u *unlocker) Release() {
	u.lock.Release(1)
}

func (n *NamedLock) TryLock(name string, timeout time.Duration) Unlocker {
	o, _ := n.locks.LoadOrStore(name, semaphore.NewWeighted(1))
	lock := o.(*semaphore.Weighted)

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(timeout))
	defer cancel()
	if err := lock.Acquire(ctx, 1); err != nil {
		return nil
	}
	return &unlocker{lock}
}

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
	return hex.EncodeToString(hash[:]), nil
}

func Contains[T comparable](arr []T, item T) bool {
	for _, elem := range arr {
		if elem == item {
			return true
		}
	}
	return false
}

func UUIDsToStrings(in []uuid.UUID) []string {
	out := make([]string, len(in))
	for i, u := range in {
		out[i] = u.String()
	}

	return out
}

func Ptr[T any](t T) *T {
	return &t
}

func Deref[T any](v *T, zeroVal ...T) T {
	if v == nil {
		if len(zeroVal) > 0 {
			return zeroVal[0]
		}
		var zero T
		return zero
	}

	return *v
}

func MapKeys[T any](m map[string]T) []string {
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func UnfoldGlobs(paths ...string) []string {
	unfoldedPaths := make([]string, 0, len(paths))
	for _, path := range paths {
		matched, err := filepath.Glob(path)
		if err != nil {
			logger.Warnf("invalid glob pattern. path=%s; %w", path, err)
			continue
		}

		unfoldedPaths = append(unfoldedPaths, matched...)
	}

	return unfoldedPaths
}

func FreePort() int {
	// Bind to port 0 to let the OS choose a free port
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		panic(err.Error())
	}

	defer listener.Close()

	// Get the address of the listener
	address := listener.Addr().(*net.TCPAddr)
	return address.Port
}

func ParseTime(t string) *time.Time {
	formats := []string{
		time.RFC3339,
		time.RFC3339Nano,
		time.ANSIC,
		time.DateTime,
		"2006-01-02T15:04:05", // ISO8601 without timezone
		"2006-01-02 15:04:05", // MySQL datetime format
	}

	for _, format := range formats {
		parsed, err := time.Parse(format, t)
		if err == nil {
			return &parsed
		}
	}

	return nil
}

func IsMapIdentical[K comparable](map1, map2 map[string]K) bool {
	if len(map1) != len(map2) {
		return false
	}

	for k, v1 := range map1 {
		if v2, exists := map2[k]; !exists || v1 != v2 {
			return false
		}
	}

	return true
}
