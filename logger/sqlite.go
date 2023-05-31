package logger

import (
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/streamingfast/gnss-controller/device/neom9n"
	"github.com/streamingfast/hivemapper-data-logger/data"
	"github.com/streamingfast/hivemapper-data-logger/data/gnss"

	_ "modernc.org/sqlite"
)

const create string = `
  CREATE TABLE IF NOT EXISTS gnss (
  	id INTEGER NOT NULL PRIMARY KEY,
  	time DATETIME NOT NULL,
  	system_time DATETIME NOT NULL,
	fix TEXT NOT NULL,
	Eph INTEGER NOT NULL,
	Sep INTEGER NOT NULL,
	latitude REAL NOT NULL,
	longitude REAL NOT NULL,
	altitude REAL NOT NULL,
	heading REAL NOT NULL,
	speed REAL NOT NULL,
	gdop REAL NOT NULL,
	hdop REAL NOT NULL,
	pdop REAL NOT NULL,
	tdop REAL NOT NULL,
	vdop REAL NOT NULL,
	xdop REAL NOT NULL,
	ydop REAL NOT NULL,
	seen INTEGER NOT NULL,
	used INTEGER NOT NULL,
	ttff INTEGER NOT NULL,
	rf_jamming_state STRING NOT NULL,
	rf_ant_status STRING NOT NULL,
	rf_ant_power STRING NOT NULL,
	rf_post_status INTEGER NOT NULL,
	rf_noise_per_ms INTEGER NOT NULL,
	rf_agc_cnt INTEGER NOT NULL,
	rf_jam_ind INTEGER NOT NULL,
	rf_ofsi INTEGER NOT NULL,
	rf_magif INTEGER NOT NULL,
	rf_ofsq INTEGER NOT NULL,
	rf_magq INTEGER NOT NULL
  );`

const insertQuery string = `
INSERT INTO gnss VALUES(NULL,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?);
`

const purgeQuery string = `
DELETE FROM gnss WHERE time < ?;
`

const lastPositionQuery string = `
SELECT latitude, longitude, altitude
FROM gnss
WHERE fix = '3D' or fix = '2D'
ORDER BY time DESC LIMIT 1;
`

type Sqlite struct {
	lock     sync.Mutex
	db       *sql.DB
	file     string
	doInsert bool
}

func NewSqlite(file string) *Sqlite {
	return &Sqlite{
		file: file,
	}
}

func (s *Sqlite) Init(logTTL time.Duration, gnssEventSubscription *data.Subscription) error {
	fmt.Println("initializing database:", s.file)
	db, err := sql.Open("sqlite", s.file)
	if err != nil {
		return fmt.Errorf("opening database: %s", err.Error())
	}

	if _, err := db.Exec(create); err != nil {
		return fmt.Errorf("creating table: %s", err.Error())
	}

	fmt.Println("database initialized, will purge every:", logTTL.String())

	if logTTL > 0 {
		go func() {
			for {
				time.Sleep(time.Minute)
				err := s.Purge(logTTL)
				if err != nil {
					panic(fmt.Errorf("purging database: %s", err.Error()))
				}
			}
		}()
	}

	s.db = db

	go func() {
		for {
			select {
			case event := <-gnssEventSubscription.IncomingEvents:
				e := event.(*gnss.GnssEvent)
				err := s.log(e.Data)
				if err != nil {
					panic(fmt.Errorf("writing to file: %w", err))
				}
			}
		}
	}()

	return nil
}

func (s *Sqlite) Purge(ttl time.Duration) error {
	s.lock.Lock()
	defer s.lock.Unlock()
	if s.db == nil {
		return fmt.Errorf("database not initialized")
	}

	t := time.Now().Add(ttl * -1)
	fmt.Println("purging database older than:", t)
	if res, err := s.db.Exec(purgeQuery, t); err != nil {
		return err
	} else {
		c, _ := res.RowsAffected()
		fmt.Println("purged rows:", c)
	}

	return nil
}
func (s *Sqlite) StartStoring() {
	s.doInsert = true
}

func (s *Sqlite) log(data *neom9n.Data) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	if !s.doInsert {
		return nil
	}

	if s.db == nil {
		return fmt.Errorf("database not initialized")
	}

	_, err := s.db.Exec(
		insertQuery,
		data.Timestamp,
		data.SystemTime,
		data.Fix,
		data.Eph,
		data.Sep,
		data.Latitude,
		data.Longitude,
		data.Altitude,
		data.Heading,
		data.Speed,
		data.Dop.GDop,
		data.Dop.HDop,
		data.Dop.PDop,
		data.Dop.TDop,
		data.Dop.VDop,
		data.Dop.XDop,
		data.Dop.YDop,
		data.Satellites.Seen,
		data.Satellites.Used,
		data.Ttff,
		data.RF.JammingState,
		data.RF.AntStatus,
		data.RF.AntPower,
		data.RF.PostStatus,
		data.RF.NoisePerMS,
		data.RF.AgcCnt,
		data.RF.JamInd,
		data.RF.OfsI,
		data.RF.MagI,
		data.RF.OfsQ,
		data.RF.MagQ,
	)
	if err != nil {
		return fmt.Errorf("inserting data: %s", err.Error())
	}
	return nil
}

func (s *Sqlite) GetLastPosition() (*neom9n.Position, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	fmt.Println("getting last position")

	rows, err := s.db.Query(lastPositionQuery)
	if err != nil {
		return nil, fmt.Errorf("querying last position: %s", err.Error())
	}
	defer rows.Close()

	if rows.Next() {
		position := &neom9n.Position{}
		err := rows.Scan(&position.Latitude, &position.Longitude, &position.Altitude)
		if err != nil {
			return nil, fmt.Errorf("scanning last position: %s", err.Error())
		}
		return position, nil
	}

	return nil, nil
}