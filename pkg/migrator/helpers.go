package migrator

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MongoIndexInfo struct {
	Name   string `bson:"name"`
	Key    bson.D `bson:"key"`
	Unique bool   `bson:"unique,omitempty"`
}

func ListMongoIndexes(ctx context.Context, col *mongo.Collection) ([]MongoIndexInfo, error) {
	cur, err := col.Indexes().List(ctx)
	if err != nil {
		return nil, err
	}

	defer cur.Close(ctx)

	var out []MongoIndexInfo

	for cur.Next(ctx) {
		var idx MongoIndexInfo

		if err := cur.Decode(&idx); err != nil {
			return nil, err
		}

		out = append(out, idx)
	}

	if err := cur.Err(); err != nil {
		return nil, err
	}

	return out, nil
}

func BSONSingleFieldIndexKeyEquals(key bson.D, field string, direction int32) bool {
	if len(key) != 1 || key[0].Key != field {
		return false
	}

	switch v := key[0].Value.(type) {
	case int32:
		return v == direction
	case int64:
		return v == int64(direction)
	case int:
		return int32(v) == direction
	case float64:
		return int32(v) == direction
	default:
		return false
	}
}

type MongoIndexState int

const (
	MongoIndexAbsent MongoIndexState = iota
	MongoIndexUnique
	MongoIndexNonUnique
)

// MongoSingleFieldIndexState:
// index with other desiredName, but other key-spec -> error,
// if exists by {field: direction}:
//   - unique -> MongoIndexUnique
//   - not unique -> MongoIndexNonUnique
func MongoSingleFieldIndexState(indexes []MongoIndexInfo, field string, direction int32, desiredName string) (state MongoIndexState, existingName string, err error) {
	for _, idx := range indexes {
		if idx.Name == desiredName && !BSONSingleFieldIndexKeyEquals(idx.Key, field, direction) {
			return MongoIndexAbsent, "", fmt.Errorf(
				"index name %q exists but key is %v (expected {%s:%d})",
				desiredName, idx.Key, field, direction,
			)
		}

		if BSONSingleFieldIndexKeyEquals(idx.Key, field, direction) {
			if idx.Unique {
				return MongoIndexUnique, idx.Name, nil
			}

			return MongoIndexNonUnique, idx.Name, nil
		}
	}

	return MongoIndexAbsent, "", nil
}

func IsMongoDuplicateKeyError(err error) bool {
	if err == nil {
		return false
	}

	var we mongo.WriteException

	if errors.As(err, &we) {
		for _, e := range we.WriteErrors {
			if e.Code == 11000 {
				return true
			}
		}
	}

	var bwe mongo.BulkWriteException

	if errors.As(err, &bwe) {
		for _, e := range bwe.WriteErrors {
			if e.Code == 11000 {
				return true
			}
		}
	}

	var ce mongo.CommandError
	if errors.As(err, &ce) && ce.Code == 11000 {
		return true
	}

	// fallback
	return strings.Contains(err.Error(), "E11000")
}

type MongoUniqueStringFieldDiagnostics struct {
	DuplicateStrings []bson.M
	NullCount        int64
}

// DiagnoseMongoUniqueStringField:
//   - field duplicates (limit)
//   - documents with field=null
func DiagnoseMongoUniqueStringField(ctx context.Context, col *mongo.Collection, field string, limit int) (MongoUniqueStringFieldDiagnostics, error) {
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.D{{Key: field, Value: bson.D{{Key: "$type", Value: "string"}}}}}},
		{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: "$" + field},
			{Key: "c", Value: bson.D{{Key: "$sum", Value: 1}}},
		}}},
		{{Key: "$match", Value: bson.D{{Key: "c", Value: bson.D{{Key: "$gt", Value: 1}}}}}},
		{{Key: "$limit", Value: limit}},
	}

	dupCur, err := col.Aggregate(ctx, pipeline, options.Aggregate().SetAllowDiskUse(true))
	if err != nil {
		return MongoUniqueStringFieldDiagnostics{}, err
	}

	defer dupCur.Close(ctx)

	var dups []bson.M

	if err := dupCur.All(ctx, &dups); err != nil {
		return MongoUniqueStringFieldDiagnostics{}, err
	}

	nullCount, err := col.CountDocuments(ctx, bson.D{{Key: field, Value: nil}})
	if err != nil {
		return MongoUniqueStringFieldDiagnostics{}, err
	}

	return MongoUniqueStringFieldDiagnostics{
		DuplicateStrings: dups,
		NullCount:        nullCount,
	}, nil
}
