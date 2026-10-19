package postgres

import (
	"github.com/golang-migrate/migrate/v4"
	migratepgx "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5/stdlib"
)

func (d *driver) migrate() (err error) {
	source, err := iofs.New(d.opts.Migrations, d.opts.MigrationsPath)
	if err != nil {
		return err
	}

	sqldb := stdlib.OpenDBFromPool(d.pool)

	driver, err := migratepgx.WithInstance(sqldb, &migratepgx.Config{})
	if err != nil {
		source.Close()
		sqldb.Close()
		return err
	}

	m, err := migrate.NewWithInstance("iofs", source, "pgx5", driver)
	if err != nil {
		source.Close()
		driver.Close()
		return err
	}

	err = m.Up()
	serr, dberr := m.Close()

	switch {
	case err != nil && err != migrate.ErrNoChange:
		return err
	case serr != nil:
		return serr
	}

	return dberr
}
