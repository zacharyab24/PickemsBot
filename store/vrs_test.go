package store

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/integration/mtest"
)

func TestFetchVrsDataFromDB_Success(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("returns VRS entries from database", func(mt *mtest.T) {
		store := &Store{
			Collections: Collections{
				VRS: mt.Coll,
			},
		}

		first := mtest.CreateCursorResponse(1, "test.vrs", mtest.FirstBatch,
			bson.D{
				{Key: "standing", Value: 3},
				{Key: "points", Value: 1800},
				{Key: "team_name", Value: "Team Spirit"},
				{Key: "roster", Value: bson.A{"chopper", "donk", "magixx", "zont1x", "Perfecto"}},
				{Key: "standings_date", Value: "2026_05_01"},
				{Key: "synced_at", Value: time.Now()},
			},
		)
		second := mtest.CreateCursorResponse(1, "test.vrs", mtest.NextBatch,
			bson.D{
				{Key: "standing", Value: 7},
				{Key: "points", Value: 1650},
				{Key: "team_name", Value: "The MongolZ"},
				{Key: "roster", Value: bson.A{"910", "bLitz", "mzinho", "Techno", "cobrazera"}},
				{Key: "standings_date", Value: "2026_05_01"},
				{Key: "synced_at", Value: time.Now()},
			},
		)
		killCursor := mtest.CreateCursorResponse(0, "test.vrs", mtest.NextBatch)
		mt.AddMockResponses(first, second, killCursor)

		entries, err := store.FetchVrsDataFromDB()
		require.NoError(t, err)
		require.Len(t, entries, 2)
		assert.Equal(t, "Team Spirit", entries[0].TeamName)
		assert.Equal(t, 3, entries[0].Standing)
		assert.Equal(t, 1800, entries[0].Points)
		assert.Equal(t, "The MongolZ", entries[1].TeamName)
		assert.Equal(t, 7, entries[1].Standing)
	})
}

func TestFetchVrsDataFromDB_Empty(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("returns empty slice when no documents", func(mt *mtest.T) {
		store := &Store{
			Collections: Collections{VRS: mt.Coll},
		}

		mt.AddMockResponses(mtest.CreateCursorResponse(0, "test.vrs", mtest.FirstBatch))

		entries, err := store.FetchVrsDataFromDB()
		require.NoError(t, err)
		assert.Empty(t, entries)
	})
}

func TestFetchVrsDataFromDB_DatabaseError(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	mt.Run("returns error on database failure", func(mt *mtest.T) {
		store := &Store{
			Collections: Collections{VRS: mt.Coll},
		}

		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{
			Code:    11000,
			Message: "database error",
		}))

		entries, err := store.FetchVrsDataFromDB()
		assert.Error(t, err)
		assert.Nil(t, entries)
	})
}
