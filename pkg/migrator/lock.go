package migrator

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type lockDoc struct {
	ID         string    `bson:"_id"`
	Owner      string    `bson:"owner"`
	LeaseUntil time.Time `bson:"leaseUntil"`
	UpdatedAt  time.Time `bson:"updatedAt"`
}

type Locker struct {
	col      *mongo.Collection
	lockKey  string
	owner    string
	leaseFor time.Duration
}

func NewLocker(db *mongo.Database, lockKey, owner string, leaseFor time.Duration) *Locker {
	return &Locker{
		col:      db.Collection("__locks"),
		lockKey:  lockKey,
		owner:    owner,
		leaseFor: leaseFor,
	}
}

func (l *Locker) TryAcquire(ctx context.Context) (bool, error) {
	var out lockDoc

	now := time.Now().UTC()
	leaseUntil := now.Add(l.leaseFor)

	filter := bson.M{
		"_id": l.lockKey,
		"$or": []bson.M{
			{"leaseUntil": bson.M{"$lte": now}},
			{"leaseUntil": bson.M{"$exists": false}},
			{"owner": l.owner},
		},
	}

	update := bson.M{
		"$set": bson.M{
			"owner":      l.owner,
			"leaseUntil": leaseUntil,
			"updatedAt":  now,
		},
	}

	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)
	err := l.col.FindOneAndUpdate(ctx, filter, update, opts).Decode(&out)

	if err == nil {
		return out.Owner == l.owner, nil
	}

	if err != mongo.ErrNoDocuments {
		return false, err
	}

	_, insErr := l.col.InsertOne(ctx, lockDoc{
		ID:         l.lockKey,
		Owner:      l.owner,
		LeaseUntil: leaseUntil,
		UpdatedAt:  now,
	})

	if insErr == nil {
		return true, nil
	}

	if IsMongoDuplicateKeyError(insErr) {
		return false, nil
	}

	return false, insErr
}

func (l *Locker) Renew(ctx context.Context) error {
	now := time.Now().UTC()
	leaseUntil := now.Add(l.leaseFor)

	filter := bson.M{"_id": l.lockKey, "owner": l.owner}
	update := bson.M{
		"$set": bson.M{
			"leaseUntil": leaseUntil,
			"updatedAt":  now,
		},
	}

	_, err := l.col.UpdateOne(ctx, filter, update)
	return err
}

func (l *Locker) Release(ctx context.Context) error {
	now := time.Now().UTC()

	filter := bson.M{"_id": l.lockKey, "owner": l.owner}
	update := bson.M{
		"$set": bson.M{
			"leaseUntil": now.Add(-time.Second),
			"updatedAt":  now,
		},
		"$unset": bson.M{
			"owner": "",
		},
	}

	_, err := l.col.UpdateOne(ctx, filter, update)
	return err
}
