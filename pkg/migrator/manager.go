package migrator

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type ManagerConfig struct {
	LockKey          string
	LeaseFor         time.Duration
	PollInterval     time.Duration
	OpTimeout        time.Duration // OpTimeout (locks, reads/writes)
	MigrationTimeout time.Duration // MigrationTimeout (long-running migrations)

	Logger *logrus.Entry

	FailFast      bool // true — Run error
	WaitForLeader bool // true — not ready, while leader not accept target
}

type Manager struct {
	db     *mongo.Database
	locker *Locker
	cfg    ManagerConfig

	ready atomic.Bool
	last  atomic.Value // string
}

func NewManager(db *mongo.Database, owner string, cfg ManagerConfig) *Manager {
	if cfg.LockKey == "" {
		cfg.LockKey = "userservice:migrations"
	}

	if cfg.LeaseFor <= 0 {
		cfg.LeaseFor = 30 * time.Second
	}

	if cfg.PollInterval <= 0 {
		cfg.PollInterval = 2 * time.Second
	}

	if cfg.OpTimeout <= 0 {
		cfg.OpTimeout = 5 * time.Second
	}

	if cfg.MigrationTimeout <= 0 {
		cfg.MigrationTimeout = 10 * time.Minute
	}

	if cfg.Logger == nil {
		cfg.Logger = logrus.NewEntry(logrus.StandardLogger())
	}

	m := &Manager{
		db:     db,
		locker: NewLocker(db, cfg.LockKey, owner, cfg.LeaseFor),
		cfg:    cfg,
	}

	m.ready.Store(false)
	m.last.Store("")

	return m
}

func (m *Manager) Ready() bool { return m.ready.Load() }

func (m *Manager) LastError() string {
	s, _ := m.last.Load().(string)
	return s
}

func (m *Manager) setErr(err error) {
	if err == nil {
		m.last.Store("")
		return
	}

	m.last.Store(err.Error())
}

func DefaultOwnerID(hostname string) string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)

	return hostname + "-" + hex.EncodeToString(b)
}

type migRecord struct {
	Version   int64     `bson:"version"`
	Name      string    `bson:"name"`
	AppliedAt time.Time `bson:"appliedAt"`
}

func (m *Manager) Run(ctx context.Context, list []Migration) error {
	sorted, err := ValidateAndSort(list)

	if err != nil {
		m.setErr(err)
		m.cfg.Logger.WithError(err).Error("MongoDB migrations: invalid list")

		if m.cfg.FailFast {
			return err
		}

		return nil
	}

	target := TargetVersion(sorted)

	// ready
	if target == 0 {
		m.ready.Store(true)
		m.setErr(nil)

		return nil
	}

	if err := m.ensureInternalIndexes(ctx); err != nil {
		m.setErr(err)
		m.cfg.Logger.WithError(err).Error("MongoDB migrations: ensure internal indexes failed")

		if m.cfg.FailFast {
			return err
		}

		return nil
	}

	// ready if already up
	applied, err := m.appliedVersion(ctx)
	if err == nil && applied >= target {
		m.ready.Store(true)
		m.setErr(nil)

		return nil
	}

	m.cfg.Logger.WithFields(logrus.Fields{
		"target":  target,
		"applied": applied,
	}).Debug("MongoDB migrations: starting (serve-with-degraded)")

	// try leader
	acquired, err := m.tryAcquireWithTimeout(ctx)
	if err != nil {
		m.setErr(err)
		m.cfg.Logger.WithError(err).Error("MongoDB migrations: lock acquire error")

		if m.cfg.FailFast {
			return err
		}

		return nil
	}

	if acquired {
		err = m.runAsLeader(ctx, sorted)

		if err == nil {
			m.ready.Store(true)
			m.setErr(nil)
			m.cfg.Logger.Info("MongoDB migrations: done")

			return nil
		}

		m.setErr(err)
		m.cfg.Logger.WithError(err).Error("MongoDB migrations: failed")

		if m.cfg.FailFast {
			return err
		}

		// degraded forever
		return nil
	}

	// not leader, wait target
	if !m.cfg.WaitForLeader {
		m.cfg.Logger.Debug("MongoDB migrations: not leader; not waiting for target (serve-with-degraded)")
		return nil
	}

	if err := m.waitForTarget(ctx, target, sorted); err != nil {
		m.setErr(err)
		m.cfg.Logger.WithError(err).Error("MongoDB migrations: wait for target failed")

		if m.cfg.FailFast {
			return err
		}

		return nil
	}

	m.ready.Store(true)
	m.setErr(nil)

	m.cfg.Logger.Debug("MongoDB migrations: observed done by leader")

	return nil
}

func (m *Manager) tryAcquireWithTimeout(ctx context.Context) (bool, error) {
	opCtx, cancel := context.WithTimeout(ctx, m.cfg.OpTimeout)
	defer cancel()

	// takeover / lock "move" visibility
	if m.cfg.Logger != nil && m.cfg.Logger.Logger != nil && m.cfg.Logger.Logger.IsLevelEnabled(logrus.DebugLevel) {
		info, err := m.locker.TryAcquireWithInfo(opCtx)
		if err != nil {
			return false, err
		}

		if info.Acquired && (info.Takeover || info.Created) {
			fields := logrus.Fields{
				"lock_key":        m.cfg.LockKey,
				"new_owner":       info.NewOwner,
				"new_lease_until": info.NewLeaseUntil,
			}

			msg := "MongoDB migrations: lock acquired"

			if info.Created {
				msg = "MongoDB migrations: lock created"
			}

			if info.Takeover {
				msg = "MongoDB migrations: lock takeover"

				fields["prev_owner"] = info.PrevOwner
				fields["prev_lease_until"] = info.PrevLeaseUntil
			}

			m.cfg.Logger.WithFields(fields).Debug(msg)
		}

		return info.Acquired, nil
	}

	return m.locker.TryAcquire(opCtx)
}

func (m *Manager) runAsLeader(ctx context.Context, list []Migration) error {
	renewCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	renewErr := make(chan error, 1)
	go func() {
		t := time.NewTicker(m.cfg.LeaseFor / 2)
		defer t.Stop()

		for {
			select {
			case <-renewCtx.Done():
				renewErr <- nil
				return
			case <-t.C:
				opCtx, c := context.WithTimeout(renewCtx, m.cfg.OpTimeout)
				err := m.locker.Renew(opCtx)

				c()

				if err != nil {
					renewErr <- err
					return
				}
			}
		}
	}()

	defer func() {
		opCtx, c := context.WithTimeout(context.Background(), m.cfg.OpTimeout)
		_ = m.locker.Release(opCtx)

		c()
	}()

	for _, mig := range list {
		applied, err := m.isApplied(ctx, mig.Version)
		if err != nil {
			cancel()

			<-renewErr
			return err
		}

		if applied {
			continue
		}

		m.cfg.Logger.WithFields(logrus.Fields{
			"version": mig.Version,
			"name":    mig.Name,
		}).Debug("MongoDB migrations: applying")

		opCtx, c := context.WithTimeout(ctx, m.cfg.MigrationTimeout)
		err = mig.Up(opCtx, m.db)

		c()

		if err != nil {
			cancel()

			<-renewErr
			return err
		}

		if err := m.recordApplied(ctx, mig); err != nil {
			if !IsMongoDuplicateKeyError(err) {
				cancel()

				<-renewErr
				return err
			}
		}
	}

	cancel()

	if err := <-renewErr; err != nil {
		return err
	}

	return nil
}

func (m *Manager) waitForTarget(ctx context.Context, target int64, sorted []Migration) error {
	t := time.NewTicker(m.cfg.PollInterval)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
			applied, err := m.appliedVersion(ctx)
			if err != nil {
				continue
			}

			if applied >= target {
				return nil
			}

			acquired, err := m.tryAcquireWithTimeout(ctx)
			if err != nil {
				continue
			}

			if acquired {
				m.cfg.Logger.Debug("MongoDB migrations: lock acquired by follower (leader takeover)")
				return m.runAsLeader(ctx, sorted)
			}
		}
	}
}

func (m *Manager) ensureInternalIndexes(ctx context.Context) error {
	coll := m.db.Collection("__migrations")

	opCtx, cancel := context.WithTimeout(ctx, m.cfg.OpTimeout)
	defer cancel()

	_, err := coll.Indexes().CreateOne(opCtx, mongo.IndexModel{
		Keys:    bson.D{{Key: "version", Value: 1}},
		Options: options.Index().SetUnique(true).SetName("uniq_version"),
	})

	return err
}

func (m *Manager) appliedVersion(ctx context.Context) (int64, error) {
	coll := m.db.Collection("__migrations")

	opCtx, cancel := context.WithTimeout(ctx, m.cfg.OpTimeout)
	defer cancel()

	opts := options.FindOne().SetSort(bson.D{{Key: "version", Value: -1}}).SetProjection(bson.M{"version": 1})

	var rec migRecord

	err := coll.FindOne(opCtx, bson.M{}, opts).Decode(&rec)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return 0, nil
		}

		return 0, err
	}

	return rec.Version, nil
}

func (m *Manager) isApplied(ctx context.Context, version int64) (bool, error) {
	coll := m.db.Collection("__migrations")

	opCtx, cancel := context.WithTimeout(ctx, m.cfg.OpTimeout)
	defer cancel()

	n, err := coll.CountDocuments(opCtx, bson.M{"version": version})
	if err != nil {
		return false, err
	}

	return n > 0, nil
}

func (m *Manager) recordApplied(ctx context.Context, mig Migration) error {
	coll := m.db.Collection("__migrations")

	opCtx, cancel := context.WithTimeout(ctx, m.cfg.OpTimeout)
	defer cancel()

	_, err := coll.InsertOne(opCtx, migRecord{
		Version:   mig.Version,
		Name:      mig.Name,
		AppliedAt: time.Now().UTC(),
	})

	return err
}
