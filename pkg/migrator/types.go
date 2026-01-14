package migrator

import (
	"context"
	"fmt"
	"sort"

	"go.mongodb.org/mongo-driver/mongo"
)

type Migration struct {
	Version int64
	Name    string
	Up      func(ctx context.Context, db *mongo.Database) error
}

func ValidateAndSort(list []Migration) ([]Migration, error) {
	if len(list) == 0 {
		return nil, nil
	}

	seen := make(map[int64]struct{}, len(list))

	for _, m := range list {
		if m.Version <= 0 {
			return nil, fmt.Errorf("migration version must be > 0, got %d (%s)", m.Version, m.Name)
		}

		if m.Name == "" {
			return nil, fmt.Errorf("migration name is empty for version %d", m.Version)
		}

		if m.Up == nil {
			return nil, fmt.Errorf("migration Up is nil for version %d (%s)", m.Version, m.Name)
		}

		if _, ok := seen[m.Version]; ok {
			return nil, fmt.Errorf("duplicate migration version %d", m.Version)
		}

		seen[m.Version] = struct{}{}
	}

	sort.Slice(list, func(i, j int) bool { return list[i].Version < list[j].Version })
	return list, nil
}

func TargetVersion(list []Migration) int64 {
	if len(list) == 0 {
		return 0
	}

	max := list[0].Version

	for _, m := range list {
		if m.Version > max {
			max = m.Version
		}
	}

	return max
}
