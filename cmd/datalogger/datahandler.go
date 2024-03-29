package main

import (
	"fmt"
	"time"

	"github.com/streamingfast/gnss-controller/device/neom9n"
	"github.com/streamingfast/hivemapper-data-logger/data"
	"github.com/streamingfast/hivemapper-data-logger/data/direction"
	"github.com/streamingfast/hivemapper-data-logger/data/imu"
	"github.com/streamingfast/hivemapper-data-logger/data/merged"
	"github.com/streamingfast/hivemapper-data-logger/logger"
	"github.com/streamingfast/imu-controller/device/iim42652"
)

type DataHandler struct {
	sqliteLogger      *logger.Sqlite
	gnssJsonLogger    *logger.JsonFile
	imuJsonLogger     *logger.JsonFile
	gnssData          *neom9n.Data
	lastImageFileName string
}

func NewDataHandler(
	dbPath string,
	dbLogTTL time.Duration,
	gnssJsonDestFolder string,
	gnssSaveInterval time.Duration,
	imuJsonDestFolder string,
	imuSaveInterval time.Duration,
) (*DataHandler, error) {
	sqliteLogger := logger.NewSqlite(
		dbPath,
		[]logger.CreateTableQueryFunc{merged.CreateTableQuery, merged.ImuRawCreateTableQuery, direction.CreateTableQuery},
		[]logger.PurgeQueryFunc{merged.PurgeQuery, merged.ImuRawPurgeQuery, direction.PurgeQuery})
	err := sqliteLogger.Init(dbLogTTL)
	if err != nil {
		return nil, fmt.Errorf("initializing sqlite logger database: %w", err)
	}

	gnssJsonLogger := logger.NewJsonFile(gnssJsonDestFolder, gnssSaveInterval)
	err = gnssJsonLogger.Init(false)
	if err != nil {
		return nil, fmt.Errorf("initializing gnss json logger: %w", err)
	}

	imuJsonLogger := logger.NewJsonFile(imuJsonDestFolder, imuSaveInterval)
	err = imuJsonLogger.Init(true)
	if err != nil {
		return nil, fmt.Errorf("initializing imu json logger: %w", err)
	}

	return &DataHandler{
		sqliteLogger:   sqliteLogger,
		gnssJsonLogger: gnssJsonLogger,
		imuJsonLogger:  imuJsonLogger,
	}, err
}

func (h *DataHandler) HandleImage(imageFileName string) error {
	h.lastImageFileName = imageFileName
	return nil
}

func (h *DataHandler) HandleOrientedAcceleration(
	acceleration *imu.Acceleration,
	tiltAngles *imu.TiltAngles,
	temperature iim42652.Temperature,
	orientation imu.Orientation,
) error {
	gnssData := mustGnssEvent(h.gnssData)
	err := h.sqliteLogger.Log(merged.NewSqlWrapper(acceleration, tiltAngles, gnssData, temperature, orientation))
	if err != nil {
		return fmt.Errorf("logging merged data to sqlite: %w", err)
	}
	return nil
}

func (h *DataHandler) HandlerGnssData(data *neom9n.Data) error {
	h.gnssData = data
	if !h.gnssJsonLogger.IsLogging && data.Fix != "none" {
		h.gnssJsonLogger.StartStoring()
	}
	err := h.gnssJsonLogger.Log(data.Timestamp, data)

	if err != nil {
		return fmt.Errorf("logging gnss data to json: %w", err)
	}
	return nil
}

func (h *DataHandler) HandleRawImuFeed(acceleration *imu.Acceleration, angularRate *iim42652.AngularRate, temperature iim42652.Temperature) error {
	gnssData := mustGnssEvent(h.gnssData)
	err := h.sqliteLogger.Log(merged.NewImuRawSqlWrapper(temperature, acceleration, gnssData /*h.lastImageFileName*/))
	if err != nil {
		return fmt.Errorf("logging raw imu data to sqlite: %w", err)
	}
	imuDataWrapper := logger.NewImuDataWrapper(temperature, acceleration, angularRate)
	err = h.imuJsonLogger.Log(time.Now(), imuDataWrapper)
	if err != nil {
		return fmt.Errorf("logging raw imu data to json: %w", err)
	}
	return nil
}

func (h *DataHandler) HandleDirectionEvent(event data.Event) error {
	gnssData := mustGnssEvent(h.gnssData)
	err := h.sqliteLogger.Log(direction.NewSqlWrapper(event, gnssData))
	if err != nil {
		return fmt.Errorf("logging direction data to sqlite: %w", err)
	}
	return nil
}
