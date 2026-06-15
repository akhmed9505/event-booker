package storage

import (
	"github.com/akhmed9505/event-booker/internal/config"
	"fmt"

	"github.com/rs/zerolog"
	"github.com/wb-go/wbf/dbpg"
)

type Postgres struct {
	db     *dbpg.DB
	logger zerolog.Logger
}

func NewPostgres(cfg *config.Config, logger zerolog.Logger) (*Postgres, error) {
	masterDSN := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Postgres.Master.Host,
		cfg.Postgres.Master.Port,
		cfg.Postgres.Master.User,
		cfg.Postgres.Master.Password,
		cfg.Postgres.Master.Database,
		cfg.Postgres.Master.SSLMode,
	)

	slaveDSNs := make([]string, 0, len(cfg.Postgres.Slaves))
	for _, slave := range cfg.Postgres.Slaves {
		dsn := fmt.Sprintf(
			"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
			slave.Host,
			slave.Port,
			slave.User,
			slave.Password,
			slave.Database,
			slave.SSLMode,
		)
		slaveDSNs = append(slaveDSNs, dsn)
	}

	db, err := dbpg.New(masterDSN, slaveDSNs, nil)
	if err != nil {
		return nil, err
	}

	return &Postgres{
		db:     db,
		logger: logger,
	}, nil
}

func (db *Postgres) Close() error {
	var err error
	if db.db.Master != nil {
		e := db.db.Master.Close()
		if e != nil {
			err = fmt.Errorf("error closing master db: %w", e)
		}
	}
	for _, slave := range db.db.Slaves {
		if slave != nil {
			e := slave.Close()
			if e != nil {
				if err == nil {
					err = fmt.Errorf("error closing slave db: %w", e)
				} else {
					err = fmt.Errorf("%v; error closing slave db: %w", err, e)
				}
			}
		}
	}
	return err
}
