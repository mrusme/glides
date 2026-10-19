package sqlite

import (
	"database/sql"

	"github.com/golang-migrate/migrate/v4"
	migratesqlite "github.com/golang-migrate/migrate/v4/database/sqlite"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

func (d *driver) migrate() (err error) {
	source, err := iofs.New(d.opts.Migrations, d.opts.MigrationsPath)
	if err != nil {
		return err
	}

	sqldb, err := sql.Open(driverName, d.dsn)
	if err != nil {
		source.Close()
		return err
	}

	driver, err := migratesqlite.WithInstance(sqldb, &migratesqlite.Config{})
	if err != nil {
		source.Close()
		sqldb.Close()
		return err
	}

	m, err := migrate.NewWithInstance("iofs", source, driverName, driver)
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
