package db

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)
// The GetPostGISConn function below establishes a connection to a PostgreSQL/PostGIS database using pgxpool,
// but it does not do any explicit PostGIS "setup" itself. Typically, PostGIS is an extension
// that must be installed and enabled on the database side (by a DBA or in SQL migrations).
// The application can optionally check if PostGIS is installed and enabled, for diagnostics. 
// Here is an optional helper to verify PostGIS is available after connecting:

func VerifyPostGISEnabled(ctx context.Context, pool *pgxpool.Pool) (bool, error) {
	const checkSQL = `SELECT postgis_full_version();`
	row := pool.QueryRow(ctx, checkSQL)
	var version string
	if err := row.Scan(&version); err != nil {
		return false, err
	}
	return true, nil
}

// Usage (example):
// pool, err := GetPostGISConn(ctx, log, dbURI)
// if err != nil { ... }
// ok, err := VerifyPostGISEnabled(ctx, pool)
// if !ok || err != nil { ... }


func GetPostGISConn(ctx context.Context, log *zap.SugaredLogger, dbURI string) (*pgxpool.Pool, error) {
	if dbURI == "" {
		return nil, errors.New("Set PostGIS URI in your env file")
	}
	pool, err := pgxpool.New(ctx, dbURI)
	if err != nil {
		return nil, errors.New("Error occured connecting to the Database")
	}
	err = pool.Ping(ctx)
	if err != nil {
		return nil, errors.New("Error occured connecting to the Database")
	}
	return pool, nil
}
