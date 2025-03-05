package projectedevent

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/openshift-assisted/ccx-exporter/internal/domain/entity"
)

// Not part of the contract, but the part of the key after the final '/' is supposed to start by [0-9a-f]
// This test should stay here until the consumer has this extra constraint.
func FuzzComputeObjectKey(f *testing.F) {
	for _, seed := range []string{"abcdef012", "78xyz", "Invalid", "!abcdef", "zxyw"} {
		f.Add(seed)
	}

	repo := S3Writer{}
	now := time.Now()

	f.Fuzz(func(t *testing.T, id string) {
		_, err := repo.computeObjectKey("type", entity.Projection{
			ID:        id,
			Timestamp: now,
		})

		if id == "" {
			assert.Error(t, err, "test should failed with empty entry")

			return
		}

		start := []rune(id)[0]
		startByLowercaseHexa := (start >= '0' && start <= '9') || (start >= 'a' && start <= 'f')

		if startByLowercaseHexa {
			assert.NoErrorf(t, err, "test should succeed for id: %s", id)
		} else {
			assert.Errorf(t, err, "test should failed for id: %s", id)
		}
	})
}

// Object key last part must start by [0-9a-f]
// This test is fragile to ensure this contract is respected
func TestComputeObjectKey(t *testing.T) {
	repo := S3Writer{}

	testcases := []struct {
		id         string
		ts         time.Time
		shouldFail bool
		expect     string
	}{
		{
			id:     "abcdef",
			ts:     time.Unix(1741014594, 0),
			expect: "custom/2025-03-03/abcdef.ndjson",
		},
		{
			id:     "04587",
			ts:     time.Unix(1741014594, 0),
			expect: "custom/2025-03-03/04587.ndjson",
		},
		{
			id:         "xyz",
			ts:         time.Unix(1741014594, 0),
			shouldFail: true,
		},
		{
			id:         "Az",
			ts:         time.Unix(1741014594, 0),
			shouldFail: true,
		},
		{
			id:         "!fff",
			ts:         time.Unix(1741014594, 0),
			shouldFail: true,
		},
		{
			id:         "",
			ts:         time.Unix(1741014594, 0),
			shouldFail: true,
		},
	}
	for _, tc := range testcases {
		key, err := repo.computeObjectKey("custom", entity.Projection{
			ID:        tc.id,
			Timestamp: tc.ts,
		})

		if tc.shouldFail {
			assert.Error(t, err, "id is supposed to generate an invalid key")

			continue
		}

		assert.Equal(t, tc.expect, key)
	}
}
