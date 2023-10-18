package sql

import (
	"encoding/json"

	"github.com/Hivemapper/gnss-controller/device/neom9n"
	"github.com/Hivemapper/hivemapper-data-logger/data/imu"
	"github.com/streamingfast/imu-controller/device/iim42652"
)

const ImuCreateTable string = `
	CREATE TABLE IF NOT EXISTS imu (
		id INTEGER NOT NULL PRIMARY KEY,
		time TIMESTAMP NOT NULL,
		acc_x REAL NOT NULL,
		acc_y REAL NOT NULL,
		acc_z REAL NOT NULL,
		gyro_x REAL NOT NULL,
		gyro_y REAL NOT NULL,
		gyro_z REAL NOT NULL,
		temperature REAL NOT NULL
	);
	create index if not exists imu_time_idx on imu(imu_time);
`

const insertRawQuery string = `INSERT INTO imu VALUES`

const insertRawFields string = `(NULL,?,?,?,?,?,?,?,?),`

const imuPurgeQuery string = `
	DELETE FROM imu WHERE time < ?;
`

func ImuCreateTableQuery() string {
	return ImuCreateTable
}

func ImuPurgeQuery() string {
	return imuPurgeQuery
}

type ImuSqlWrapper struct {
	acceleration *imu.Acceleration
	temperature  iim42652.Temperature
	gyroscope *iim42652.AngularRate
}

func NewImuSqlWrapper(temperature iim42652.Temperature, acceleration *imu.Acceleration, gyroscope *iim42652.AngularRate) *ImuSqlWrapper {
	return &ImuSqlWrapper{
		acceleration: acceleration,
		temperature:  temperature,
		gyroscope:     gyroscope,
	}
}

func (w *ImuSqlWrapper) InsertQuery() (string, string, []any) {
	return insertRawQuery, insertRawFields, []any{
		w.acceleration.Time.Format("2006-01-02 15:04:05.99999"),
		w.acceleration.X,
		w.acceleration.Y,
		w.acceleration.Z,
		*w.temperature,
		w.gyroscope.X,
		w.gyroscope.Y,
		w.gyroscope.Z,
	}
}
