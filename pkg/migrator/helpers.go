package migrator

import (
	"errors"

	"go.mongodb.org/mongo-driver/mongo"
)

func IsDuplicateKey(err error) bool {
	var we mongo.WriteException

	if errors.As(err, &we) {
		for _, e := range we.WriteErrors {
			if e.Code == 11000 {
				return true
			}
		}
	}

	return false
}
